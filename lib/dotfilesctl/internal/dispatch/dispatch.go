package dispatch

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/agoodkind/.dotfiles/internal/catalog"
	"github.com/agoodkind/.dotfiles/internal/dispatch/pathcache"
	"github.com/agoodkind/.dotfiles/internal/dispatch/prefercache"
	"github.com/agoodkind/.dotfiles/internal/dispatch/sshkey"
	"github.com/agoodkind/.dotfiles/internal/dispatch/updater"
	"github.com/agoodkind/.dotfiles/internal/dispatch/zwc"
	"github.com/agoodkind/.dotfiles/internal/runner"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
	"github.com/agoodkind/.dotfiles/internal/util"
)

func Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	dotfiles := os.Getenv("DOTDOTFILES")
	if dotfiles == "" {
		dotfiles = filepath.Join(os.Getenv("HOME"), ".dotfiles")
	}
	dispatchConfig := catalog.DefaultDispatchConfig()

	logPath := util.ResolveConfigPath(dispatchConfig.LogPath, dotfiles)
	if logPath == "" {
		logPath = filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles_dispatch.log")
	}
	dispatchLogger, err := telemetry.NewLogger(logPath)
	if err != nil {
		return err
	}
	defer dispatchLogger.Close()
	runner.SetLogger(dispatchLogger)
	defer runner.SetLogger(nil)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(dotfiles, ".cache"), 0o755); err != nil {
		return err
	}
	lockPath := util.ResolveConfigPath(dispatchConfig.LockFile, dotfiles)
	if lockPath == "" {
		lockPath = filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles_dispatch.flock")
	}
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		dispatchLogger.Warn("another dispatch already running, exiting")
		return nil
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	statusDir := util.ResolveConfigPath(dispatchConfig.StatusDir, dotfiles)
	if statusDir == "" {
		statusDir = filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles_dispatch.lock")
	}
	if err := os.MkdirAll(statusDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(statusDir, "pid"), []byte(fmt.Sprintf("%d", os.Getpid())), 0o644); err != nil {
		return err
	}
	defer os.RemoveAll(statusDir)

	type workerResult struct {
		name string
		err  error
	}
	type worker struct {
		name string
		run  func(context.Context, string) error
	}
	workers := make([]worker, 0, len(dispatchConfig.Workers))
	for _, workerConfig := range dispatchConfig.Workers {
		if !workerConfig.Enabled {
			continue
		}
		switch workerConfig.Name {
		case "updater":
			workers = append(workers, worker{
				name: workerConfig.Name,
				run: func(ctx context.Context, dotfiles string) error {
					return updater.Run(
						ctx,
						dotfiles,
						statusDir,
						util.ResolveConfigPath(dispatchConfig.WeeklyUpdateMarker, dotfiles),
						dispatchConfig.WeeklyUpdateHours,
						notifyDispatchLogPath(logPath),
						dispatchLogger,
					)
				},
			})
		case "prefer-cache-rebuild":
			workers = append(workers, worker{
				name: workerConfig.Name,
				run: func(ctx context.Context, dotfiles string) error {
					return prefercache.Rebuild(ctx, dotfiles, false, dispatchLogger)
				},
			})
		case "path-cache-rebuild":
			workers = append(workers, worker{
				name: workerConfig.Name,
				run: func(ctx context.Context, dotfiles string) error {
					return pathcache.Rebuild(dispatchLogger)
				},
			})
		case "zwc-recompile":
			workers = append(workers, worker{
				name: workerConfig.Name,
				run: func(ctx context.Context, dotfiles string) error {
					return zwc.Recompile(ctx, dotfiles, dispatchLogger)
				},
			})
		case "ssh-key-load-mac":
			workers = append(workers, worker{
				name: workerConfig.Name,
				run: func(ctx context.Context, _ string) error {
					return sshkey.Load(ctx, dispatchLogger)
				},
			})
		default:
			dispatchLogger.Warn(fmt.Sprintf("unknown worker: %s", workerConfig.Name))
		}
	}
	if len(workers) == 0 {
		workers = []worker{
			{name: "updater", run: func(ctx context.Context, dotfiles string) error {
				return updater.Run(
					ctx,
					dotfiles,
					statusDir,
					util.ResolveConfigPath(dispatchConfig.WeeklyUpdateMarker, dotfiles),
					dispatchConfig.WeeklyUpdateHours,
					notifyDispatchLogPath(logPath),
					dispatchLogger,
				)
			}},
			{name: "prefer-cache-rebuild", run: func(ctx context.Context, dotfiles string) error {
				return prefercache.Rebuild(ctx, dotfiles, false, dispatchLogger)
			}},
			{name: "path-cache-rebuild", run: func(ctx context.Context, dotfiles string) error {
				return pathcache.Rebuild(dispatchLogger)
			}},
			{name: "zwc-recompile", run: func(ctx context.Context, dotfiles string) error {
				return zwc.Recompile(ctx, dotfiles, dispatchLogger)
			}},
			{name: "ssh-key-load-mac", run: func(ctx context.Context, _ string) error {
				return sshkey.Load(ctx, dispatchLogger)
			}},
		}
	}

	workerDone := make(chan workerResult, len(workers))
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	dispatchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, current := range workers {
		dispatchLogger.Info(fmt.Sprintf("starting %s", current.name))
		go func(name string, fn func(context.Context, string) error) {
			workerDone <- workerResult{name: name, err: fn(dispatchCtx, dotfiles)}
		}(current.name, current.run)
	}

	active := len(workers)
	for active > 0 {
		select {
		case <-dispatchCtx.Done():
			for active > 0 {
				result := <-workerDone
				if result.err != nil {
					dispatchLogger.Warn(fmt.Sprintf("WARN: %s exited with %v", result.name, result.err))
				}
				active--
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return nil
		case <-sigCh:
			cancel()
			for active > 0 {
				result := <-workerDone
				if result.err != nil {
					dispatchLogger.Warn(fmt.Sprintf("WARN: %s exited with %v", result.name, result.err))
				}
				active--
			}
			return fmt.Errorf("interrupted")
		case result := <-workerDone:
			if result.err != nil {
				dispatchLogger.Warn(fmt.Sprintf("WARN: %s: %v", result.name, result.err))
			}
			active--
		}
	}

	return nil
}

func notifyDispatchLogPath(logPath string) string {
	if logPath == "" {
		return filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles_dispatch.log")
	}
	return logPath
}
