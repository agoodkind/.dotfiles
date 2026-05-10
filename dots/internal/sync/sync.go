// Package sync implements the top-level dotfiles sync orchestration.
package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"goodkind.io/.dotfiles/internal/runner"
	"goodkind.io/.dotfiles/internal/sync/compilation"
	"goodkind.io/.dotfiles/internal/sync/platform"
	"goodkind.io/.dotfiles/internal/sync/repository"
	"goodkind.io/.dotfiles/internal/sync/tools"
	"goodkind.io/.dotfiles/internal/sync/workspace"
	"goodkind.io/.dotfiles/internal/telemetry"
)

// Options controls which phases and checks the sync pipeline performs.
type Options struct {
	RepairMode     bool
	QuickMode      bool
	SkipGit        bool
	SkipNetwork    bool
	SkipCursorSync bool
	DryRun         bool
	UseDefaults    bool
}

var commandLogger *telemetry.Logger

// Run executes the full dotfiles sync pipeline with the given options.
func Run(ctx context.Context, options Options) error {
	dotfiles := resolveDotfilesEnv()

	logger, logPath, err := openSyncLogger()
	if err != nil {
		return err
	}
	defer logger.Close()
	commandLogger = logger
	defer func() {
		commandLogger = nil
		runner.SetLogger(nil)
	}()
	runner.SetLogger(commandLogger)
	_ = os.Setenv("DOTFILES_LOG", logPath)

	notify := func(level string, message string) {
		if err := telemetry.Notify(level, message, logPath); err != nil {
			logger.WarnContextWithErr(ctx, "notification write failed", err)
		}
		if level == "warn" || level == "error" {
			logger.WarnContext(ctx, message)
		}
	}

	lockFile, flockFdInt, alreadyRunning, err := acquireSyncLock(ctx, logger)
	if err != nil {
		return err
	}
	if alreadyRunning {
		return nil
	}
	defer lockFile.Close()
	defer syscall.Flock(flockFdInt, syscall.LOCK_UN)

	failed := make([]string, 0)
	logger.InfoContext(ctx, "Dotfiles sync started")

	runStep := func(name string, critical bool, fn func(context.Context) error) error {
		done := logger.SectionContext(ctx, name)
		defer done()
		if options.DryRun {
			logger.InfoContext(ctx, "  dry-run: no changes applied")
			return nil
		}
		if err := fn(ctx); err != nil {
			if critical {
				logger.ErrorContextWithErr(ctx, "FATAL: "+name, err)
				notify("error", fmt.Sprintf("sync failed at %s: %v", name, err))
				return err
			}
			logger.WarnContextWithErr(ctx, "WARN: "+name, err)
			notify("warn", "sync step failed (continued): "+name)
			failed = append(failed, name)
		}
		return nil
	}

	if err := runLinkSteps(options, dotfiles, logger, runStep); err != nil {
		return err
	}
	if err := runConfigSteps(options, dotfiles, logger, runStep); err != nil {
		return err
	}
	if err := runUpdateSteps(options, dotfiles, logger, runStep); err != nil {
		return err
	}
	if err := runCompilationSteps(dotfiles, logger, runStep); err != nil {
		return err
	}

	if len(failed) > 0 {
		msg := strings.Join(failed, ", ")
		notify("warn", "sync completed with non-critical failures: "+msg)
		logger.WarnContext(ctx, "Completed with non-critical failures: "+msg)
	}

	logger.SuccessContext(ctx, "Dotfiles synced")
	return nil
}

func resolveDotfilesEnv() string {
	dotfiles := os.Getenv("DOTDOTFILES")
	if dotfiles == "" {
		dotfiles = filepath.Join(os.Getenv("HOME"), ".dotfiles")
	}
	_ = os.Setenv("DOTDOTFILES", dotfiles)
	repository.LoadOverrides()
	return dotfiles
}

func openSyncLogger() (*telemetry.Logger, string, error) {
	if err := os.MkdirAll(filepath.Clean(filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles")), 0o755); err != nil {
		slog.Error("sync: openSyncLogger: creating cache directory", "err", err)
		return nil, "", fmt.Errorf("creating dotfiles cache directory: %w", err)
	}
	logPath := os.Getenv("DOTFILES_LOG")
	if logPath == "" {
		logPath = filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "sync.log")
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(logPath)), 0o755); err != nil {
		slog.Error("sync: openSyncLogger: creating log directory", "err", err)
		return nil, "", fmt.Errorf("creating log directory: %w", err)
	}
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		slog.Error("sync: openSyncLogger: creating logger", "err", err)
		return nil, "", fmt.Errorf("creating sync logger: %w", err)
	}
	return logger, logPath, nil
}

func acquireSyncLock(ctx context.Context, logger *telemetry.Logger) (*os.File, int, bool, error) {
	if err := os.MkdirAll(filepath.Clean(filepath.Join(os.Getenv("HOME"), ".cache")), 0o755); err != nil {
		return nil, 0, false, fmt.Errorf("creating cache directory: %w", err)
	}
	lockPath := filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles_sync.flock")
	lockFile, err := os.OpenFile(filepath.Clean(lockPath), os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return nil, 0, false, fmt.Errorf("opening sync lock file: %w", err)
	}
	flockFd := lockFile.Fd()
	if uint64(flockFd) > uint64(^uint(0)>>1) {
		_ = lockFile.Close()
		return nil, 0, false, fmt.Errorf("lock file descriptor %d exceeds int bounds", flockFd)
	}
	flockFdInt := int(flockFd)
	if err := syscall.Flock(flockFdInt, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lockFile.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			logger.InfoContext(ctx, "sync already running, exiting")
			return nil, 0, true, nil
		}
		logger.WarnContextWithErr(ctx, "flock acquire failed", err)
		slog.WarnContext(ctx, "flock acquire failed", "err", err)
		return nil, 0, false, fmt.Errorf("acquiring sync lock: %w", err)
	}
	return lockFile, flockFdInt, false, nil
}

type syncStep = func(string, bool, func(context.Context) error) error

func runLinkSteps(options Options, dotfiles string, logger *telemetry.Logger, step syncStep) error {
	if err := step("Updating git repo", true, func(ctx context.Context) error {
		return repository.UpdateGitRepoSync(ctx, options.SkipGit, logger)
	}); err != nil {
		return err
	}
	if err := step("Cleaning zinit completions", false, func(ctx context.Context) error {
		return workspace.CleanupZinitCompletions(ctx, logger)
	}); err != nil {
		return err
	}
	if err := step("Linking dotfiles", false, func(ctx context.Context) error {
		return workspace.LinkDotfiles(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Syncing SSH config", false, func(ctx context.Context) error {
		return workspace.SyncSSHConfig(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Updating authorized keys", false, func(ctx context.Context) error {
		return workspace.UpdateAuthorizedKeys(ctx, options.SkipNetwork, logger)
	}); err != nil {
		return err
	}
	return nil
}

func runConfigSteps(options Options, dotfiles string, logger *telemetry.Logger, step syncStep) error {
	if err := step("Syncing Cursor configuration", false, func(ctx context.Context) error {
		if options.SkipCursorSync {
			logger.InfoContext(ctx, "  skipping cursor config sync")
			return nil
		}
		return workspace.SyncCursorConfig(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Syncing Cursor User Rules", false, func(ctx context.Context) error {
		if options.SkipCursorSync {
			logger.InfoContext(ctx, "  skipping cursor user rules sync")
			return nil
		}
		return workspace.SyncCursorUserRules(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Syncing Claude configuration", false, func(ctx context.Context) error {
		return workspace.SyncClaudeConfig(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Syncing Codex configuration", false, func(ctx context.Context) error {
		return workspace.SyncCodexConfig(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Syncing Gemini configuration", false, func(ctx context.Context) error {
		return workspace.SyncGeminiConfig(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Syncing Copilot configuration", false, func(ctx context.Context) error {
		return workspace.SyncCopilotConfig(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Checking git hooks path", false, func(ctx context.Context) error {
		return workspace.CheckGitHooksPath(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Syncing git hooks", false, func(ctx context.Context) error {
		return workspace.SyncGitHooks(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Configuring global git hooks", false, func(ctx context.Context) error {
		return workspace.SyncGlobalGitHooks(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	return nil
}

func runUpdateSteps(options Options, dotfiles string, logger *telemetry.Logger, step syncStep) error {
	if err := step("Updating and compiling zinit plugins", false, func(ctx context.Context) error {
		if options.QuickMode || options.SkipNetwork {
			return nil
		}
		return workspace.UpdateZinitPlugins(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Running OS setup", false, func(ctx context.Context) error {
		return platform.RunOSInstall(ctx, options.QuickMode, options.UseDefaults, logger)
	}); err != nil {
		return err
	}
	if err := step("Installing custom tools", false, func(ctx context.Context) error {
		if options.QuickMode {
			return nil
		}
		return tools.InstallCustomTools(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Updating Neovim plugins", false, func(ctx context.Context) error {
		if options.QuickMode || options.SkipNetwork {
			return nil
		}
		return workspace.UpdateNeovimPlugins(ctx, logger)
	}); err != nil {
		return err
	}
	if err := step("Repair: cleaning up Homebrew", false, func(ctx context.Context) error {
		if !options.RepairMode {
			return nil
		}
		return workspace.CleanupHomebrewRepair(ctx, logger)
	}); err != nil {
		return err
	}
	if err := step("Repair: cleaning up Neovim", false, func(ctx context.Context) error {
		if !options.RepairMode {
			return nil
		}
		return workspace.CleanupNeovimRepair(ctx, logger)
	}); err != nil {
		return err
	}
	return nil
}

func runCompilationSteps(dotfiles string, logger *telemetry.Logger, step syncStep) error {
	if err := step("Compiling zsh files", false, func(ctx context.Context) error {
		return compilation.CompileZshFiles(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Rebuilding zcompdump", false, func(ctx context.Context) error {
		return compilation.RebuildZcompdump(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Rebuilding prefer cache", false, func(ctx context.Context) error {
		return compilation.RebuildPreferCache(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := step("Creating hushlogin", false, func(ctx context.Context) error {
		return compilation.CreateHushLogin(ctx, logger)
	}); err != nil {
		return err
	}
	return nil
}
