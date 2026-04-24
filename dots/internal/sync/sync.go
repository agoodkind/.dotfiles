package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/agoodkind/.dotfiles/internal/runner"
	"github.com/agoodkind/.dotfiles/internal/sync/compilation"
	"github.com/agoodkind/.dotfiles/internal/sync/platform"
	"github.com/agoodkind/.dotfiles/internal/sync/repository"
	"github.com/agoodkind/.dotfiles/internal/sync/tools"
	"github.com/agoodkind/.dotfiles/internal/sync/workspace"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
)

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

func Run(ctx context.Context, options Options) error {
	if ctx == nil {
		ctx = context.Background()
	}

	dotfiles := os.Getenv("DOTDOTFILES")
	if dotfiles == "" {
		dotfiles = filepath.Join(os.Getenv("HOME"), ".dotfiles")
	}
	_ = os.Setenv("DOTDOTFILES", dotfiles)
	repository.LoadOverrides()

	if err := os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles"), 0o755); err != nil {
		return err
	}
	logPath := os.Getenv("DOTFILES_LOG")
	if logPath == "" {
		logPath = filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "sync.log")
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	logger, err := telemetry.NewLogger(logPath)
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
			logger.Warn(fmt.Sprintf("notification write failed: %v", err))
		}
		if level == "warn" || level == "error" {
			logger.Warn(message)
		}
	}

	if err := os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".cache"), 0o755); err != nil {
		return err
	}
	lockPath := filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles_sync.flock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}
	defer lockFile.Close()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		logger.Info("sync already running, exiting")
		return nil
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	failed := make([]string, 0)
	logger.Info("Dotfiles sync started")

	runStep := func(name string, critical bool, fn func(context.Context) error) error {
		done := logger.Section(name)
		defer done()
		if options.DryRun {
			logger.Info("  dry-run: no changes applied")
			return nil
		}
		if err := fn(ctx); err != nil {
			if critical {
				logger.Error(fmt.Sprintf("FATAL: %s: %v", name, err))
				notify("error", fmt.Sprintf("sync failed at %s: %v", name, err))
				return err
			}
			logger.Warn(fmt.Sprintf("WARN: %s: %v", name, err))
			notify("warn", fmt.Sprintf("sync step failed (continued): %s", name))
			failed = append(failed, name)
		}
		return nil
	}

	if err := runStep("Updating git repo", true, func(ctx context.Context) error {
		return repository.UpdateGitRepoSync(ctx, options.SkipGit, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Cleaning zinit completions", false, func(ctx context.Context) error {
		return workspace.CleanupZinitCompletions(ctx, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Linking dotfiles", false, func(ctx context.Context) error {
		return workspace.LinkDotfiles(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Syncing SSH config", false, func(ctx context.Context) error {
		return workspace.SyncSSHConfig(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Updating authorized keys", false, func(ctx context.Context) error {
		return workspace.UpdateAuthorizedKeys(ctx, options.SkipNetwork, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Syncing Cursor configuration", false, func(ctx context.Context) error {
		if options.SkipCursorSync {
			logger.Info("  skipping cursor config sync")
			return nil
		}
		return workspace.SyncCursorConfig(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Syncing Cursor User Rules", false, func(ctx context.Context) error {
		if options.SkipCursorSync {
			logger.Info("  skipping cursor user rules sync")
			return nil
		}
		return workspace.SyncCursorUserRules(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Syncing Claude configuration", false, func(ctx context.Context) error {
		return workspace.SyncClaudeConfig(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Checking git hooks path", false, func(ctx context.Context) error {
		return workspace.CheckGitHooksPath(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Syncing git hooks", false, func(ctx context.Context) error {
		return workspace.SyncGitHooks(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Configuring global git hooks", false, func(ctx context.Context) error {
		return workspace.SyncGlobalGitHooks(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Updating and compiling zinit plugins", false, func(ctx context.Context) error {
		if options.QuickMode || options.SkipNetwork {
			return nil
		}
		return workspace.UpdateZinitPlugins(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Running OS setup", false, func(ctx context.Context) error {
		return platform.RunOSInstall(ctx, options.QuickMode, options.UseDefaults, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Installing custom tools", false, func(ctx context.Context) error {
		if options.QuickMode {
			return nil
		}
		return tools.InstallCustomTools(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Updating Neovim plugins", false, func(ctx context.Context) error {
		if options.QuickMode || options.SkipNetwork {
			return nil
		}
		return workspace.UpdateNeovimPlugins(ctx, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Repair: cleaning up Homebrew", false, func(ctx context.Context) error {
		if !options.RepairMode {
			return nil
		}
		return workspace.CleanupHomebrewRepair(ctx, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Repair: cleaning up Neovim", false, func(ctx context.Context) error {
		if !options.RepairMode {
			return nil
		}
		return workspace.CleanupNeovimRepair(ctx, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Compiling zsh files", false, func(ctx context.Context) error {
		return compilation.CompileZshFiles(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Rebuilding zcompdump", false, func(ctx context.Context) error {
		return compilation.RebuildZcompdump(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Rebuilding prefer cache", false, func(ctx context.Context) error {
		return compilation.RebuildPreferCache(ctx, dotfiles, logger)
	}); err != nil {
		return err
	}
	if err := runStep("Creating hushlogin", false, func(ctx context.Context) error {
		return compilation.CreateHushLogin(ctx, logger)
	}); err != nil {
		return err
	}

	if len(failed) > 0 {
		msg := strings.Join(failed, ", ")
		notify("warn", "sync completed with non-critical failures: "+msg)
		logger.Warn("Completed with non-critical failures: " + msg)
	}

	logger.Success("Dotfiles synced")
	return nil
}
