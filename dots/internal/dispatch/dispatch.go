// Package dispatch implements the background dispatch runner for dotfiles maintenance.
package dispatch

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"goodkind.io/.dotfiles/internal/catalog"
	"goodkind.io/.dotfiles/internal/dispatch/pathcache"
	"goodkind.io/.dotfiles/internal/dispatch/prefercache"
	"goodkind.io/.dotfiles/internal/dispatch/sshkey"
	"goodkind.io/.dotfiles/internal/dispatch/updater"
	"goodkind.io/.dotfiles/internal/dispatch/zwc"
	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/telemetry"
	"goodkind.io/.dotfiles/internal/util"
)

type workerEntry struct {
	name string
	run  func(context.Context, string) error
}

type workerOutcome struct {
	name string
	err  error
}

type workerName string

const (
	workerUpdater            workerName = "updater"
	workerPreferCacheRebuild workerName = "prefer-cache-rebuild"
	workerPathCacheRebuild   workerName = "path-cache-rebuild"
	workerZwcRecompile       workerName = "zwc-recompile"
	workerSSHKeyLoadMac      workerName = "ssh-key-load-mac"
)

func buildWorkersFromConfig(
	ctx context.Context,
	statusDir string,
	logPath string,
	dispatchConfig *catalog.DispatchConfig,
	dispatchLogger *telemetry.Logger,
	selected map[string]bool,
) []workerEntry {
	workers := make([]workerEntry, 0, len(dispatchConfig.Workers))
	for _, workerConfig := range dispatchConfig.Workers {
		if !workerConfig.Enabled {
			continue
		}
		if len(selected) > 0 {
			if _, ok := selected[workerConfig.Name]; !ok {
				continue
			}
			selected[workerConfig.Name] = true
		}
		switch workerName(workerConfig.Name) {
		case workerUpdater:
			workers = append(workers, workerEntry{
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
		case workerPreferCacheRebuild:
			workers = append(workers, workerEntry{
				name: workerConfig.Name,
				run: func(ctx context.Context, dotfiles string) error {
					return prefercache.Rebuild(ctx, dotfiles, false, dispatchLogger)
				},
			})
		case workerPathCacheRebuild:
			workers = append(workers, workerEntry{
				name: workerConfig.Name,
				run: func(ctx context.Context, dotfiles string) error {
					return pathcache.Rebuild(ctx, dispatchLogger)
				},
			})
		case workerZwcRecompile:
			workers = append(workers, workerEntry{
				name: workerConfig.Name,
				run: func(ctx context.Context, dotfiles string) error {
					return zwc.Recompile(ctx, dotfiles, dispatchLogger)
				},
			})
		case workerSSHKeyLoadMac:
			workers = append(workers, workerEntry{
				name: workerConfig.Name,
				run: func(ctx context.Context, _ string) error {
					return sshkey.Load(ctx, dispatchLogger)
				},
			})
		default:
			dispatchLogger.WarnContext(ctx, "unknown worker: "+workerConfig.Name)
		}
	}
	for name, found := range selected {
		if !found {
			dispatchLogger.WarnContext(ctx, "unknown or disabled worker: "+name)
		}
	}
	return workers
}

func defaultWorkerList(
	statusDir string,
	logPath string,
	dispatchConfig *catalog.DispatchConfig,
	dispatchLogger *telemetry.Logger,
) []workerEntry {
	return []workerEntry{
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
			return pathcache.Rebuild(ctx, dispatchLogger)
		}},
		{name: "zwc-recompile", run: func(ctx context.Context, dotfiles string) error {
			return zwc.Recompile(ctx, dotfiles, dispatchLogger)
		}},
		{name: "ssh-key-load-mac", run: func(ctx context.Context, _ string) error {
			return sshkey.Load(ctx, dispatchLogger)
		}},
	}
}

func launchWorkers(dispatchCtx context.Context, dotfiles string, workers []workerEntry, workerDone chan<- workerOutcome) {
	for _, current := range workers {
		go func(name string, fn func(context.Context, string) error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					workerDone <- workerOutcome{name: name, err: fmt.Errorf("panic: %v", recovered)}
				}
			}()
			workerDone <- workerOutcome{name: name, err: fn(dispatchCtx, dotfiles)}
		}(current.name, current.run)
	}
}

func drainActiveWorkers(
	dispatchCtx context.Context,
	ctx context.Context,
	sigCh <-chan os.Signal,
	workerDone <-chan workerOutcome,
	active int,
	cancel context.CancelFunc,
	dispatchLogger *telemetry.Logger,
) error {
	for active > 0 {
		select {
		case <-dispatchCtx.Done():
			for active > 0 {
				result := <-workerDone
				if result.err != nil {
					dispatchLogger.WarnContextWithErr(ctx, fmt.Sprintf("WARN: %s exited", result.name), result.err)
				}
				active--
			}
			if ctx.Err() != nil {
				slog.WarnContext(ctx, "dispatch: context cancelled after drain")
				return fmt.Errorf("context cancelled: %w", ctx.Err())
			}
			return nil
		case <-sigCh:
			cancel()
			for active > 0 {
				result := <-workerDone
				if result.err != nil {
					dispatchLogger.WarnContextWithErr(ctx, fmt.Sprintf("WARN: %s exited", result.name), result.err)
				}
				active--
			}
			return fmt.Errorf("interrupted")
		case result := <-workerDone:
			if result.err != nil {
				dispatchLogger.WarnContextWithErr(ctx, "WARN: "+result.name, result.err)
			}
			active--
		}
	}
	return nil
}

// RunWorkers acquires a dispatch lock, builds the worker list, and runs each
// worker concurrently until all complete or the context is cancelled.
func RunWorkers(ctx context.Context, selectedWorkers []string) error {
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
		return fmt.Errorf("creating dispatch logger: %w", err)
	}
	defer dispatchLogger.Close()
	runner.SetLogger(dispatchLogger)
	defer runner.SetLogger(nil)
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(logPath)), 0o755); err != nil {
		return fmt.Errorf("creating log directory: %w", err)
	}

	if err := os.MkdirAll(filepath.Clean(filepath.Join(dotfiles, ".cache")), 0o755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}
	lockPath := util.ResolveConfigPath(dispatchConfig.LockFile, dotfiles)
	if lockPath == "" {
		lockPath = filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles_dispatch.flock")
	}
	lockFile, err := os.OpenFile(filepath.Clean(lockPath), os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return fmt.Errorf("opening dispatch lock file: %w", err)
	}
	defer lockFile.Close()

	flockFd := lockFile.Fd()
	if uint64(flockFd) > uint64(^uint(0)>>1) {
		return fmt.Errorf("lock file descriptor %d exceeds int bounds", flockFd)
	}
	flockFdInt := int(flockFd)
	if err := syscall.Flock(flockFdInt, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		dispatchLogger.WarnContext(ctx, "another dispatch already running, exiting")
		return nil
	}
	defer syscall.Flock(flockFdInt, syscall.LOCK_UN)

	statusDir := util.ResolveConfigPath(dispatchConfig.StatusDir, dotfiles)
	if statusDir == "" {
		statusDir = filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles_dispatch.lock")
	}
	if err := os.MkdirAll(filepath.Clean(statusDir), 0o755); err != nil {
		return fmt.Errorf("creating status directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Clean(statusDir), "pid"), fmt.Appendf(nil, "%d", os.Getpid()), 0o600); err != nil {
		return fmt.Errorf("writing pid file: %w", err)
	}
	defer os.RemoveAll(statusDir)

	selected := make(map[string]bool, len(selectedWorkers))
	for _, name := range selectedWorkers {
		if name != "" {
			selected[name] = false
		}
	}

	workers := buildWorkersFromConfig(ctx, statusDir, logPath, dispatchConfig, dispatchLogger, selected)
	if len(workers) == 0 {
		if len(selected) > 0 {
			return nil
		}
		workers = defaultWorkerList(statusDir, logPath, dispatchConfig, dispatchLogger)
	}

	workerDone := make(chan workerOutcome, len(workers))
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	dispatchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	logWorkersStart(ctx, workers, dispatchLogger)
	launchWorkers(dispatchCtx, dotfiles, workers, workerDone)

	return drainActiveWorkers(dispatchCtx, ctx, sigCh, workerDone, len(workers), cancel, dispatchLogger)
}

func logWorkersStart(ctx context.Context, workers []workerEntry, logger *telemetry.Logger) {
	workerNames := make([]string, 0, len(workers))
	for _, current := range workers {
		workerNames = append(workerNames, current.name)
	}
	logger.InfoContext(ctx, "starting workers: "+strings.Join(workerNames, ", "))
}

func notifyDispatchLogPath(logPath string) string {
	if logPath == "" {
		return filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles_dispatch.log")
	}
	return logPath
}
