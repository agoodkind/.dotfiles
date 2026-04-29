package main

import (
	"context"
	"flag"
	"io"
	"os"
	"path/filepath"

	agentsync "github.com/agoodkind/.dotfiles/internal/agentsync"
	cursorSync "github.com/agoodkind/.dotfiles/internal/cursor/syncer"
	dispatcher "github.com/agoodkind/.dotfiles/internal/dispatch"
	installer "github.com/agoodkind/.dotfiles/internal/install"
	perfcmd "github.com/agoodkind/.dotfiles/internal/perf"
	"github.com/agoodkind/.dotfiles/internal/runner"
	syncer "github.com/agoodkind/.dotfiles/internal/sync"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
	uninstaller "github.com/agoodkind/.dotfiles/internal/uninstall"
)

var appLogger *telemetry.Logger

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	logPath := os.Getenv("DOTFILES_LOG")
	if logPath == "" {
		logPath = filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles", "dots.log")
	}
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		return 1
	}
	appLogger = logger
	defer logger.Close()
	runner.SetLogger(logger)
	defer func() {
		appLogger = nil
		runner.SetLogger(nil)
	}()

	if len(args) == 0 {
		printUsage()
		return 2
	}

	switch args[0] {
	case "sync":
		return runSync(args[1:])
	case "dispatch":
		return runDispatch(args[1:])
	case "perf":
		return runPerf(args[1:])
	case "sync-agent-repo":
		return runSyncAgentRepo(args[1:])
	case "refresh-shell-caches":
		return runRefreshShellCaches(args[1:])
	case "cursor-sync":
		return runCursorSync(args[1:])
	case "install":
		return runInstall(args[1:])
	case "uninstall":
		return runUninstall(args[1:])
	case "version":
		logInfo("dots 0.1.0")
		return 0
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		logError("unknown command: " + args[0])
		printUsage()
		return 2
	}
}

func runSync(args []string) int {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			logInfo("Usage:")
			logInfo("  dots sync [--repair] [--quick] [--skip-git] [--skip-network] [--skip-cursor-sync] [--dry-run] [--use-defaults]")
			return 0
		}
	}
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	repairMode := fs.Bool("repair", false, "run full repair flow")
	quickMode := fs.Bool("quick", false, "skip heavy network steps")
	skipGit := fs.Bool("skip-git", false, "skip repository sync step")
	skipNetwork := fs.Bool("skip-network", false, "skip networked operations")
	skipCursorSync := fs.Bool("skip-cursor-sync", false, "skip cursor rule syncing")
	dryRun := fs.Bool("dry-run", false, "run through sync steps without applying changes")
	useDefaults := fs.Bool("use-defaults", false, "use non-interactive installer defaults")
	if err := fs.Parse(args); err != nil {
		logWarn(err.Error())
		printUsage()
		return 2
	}

	if err := syncer.Run(context.Background(), syncer.Options{
		RepairMode:     *repairMode,
		QuickMode:      *quickMode,
		SkipGit:        *skipGit,
		SkipNetwork:    *skipNetwork,
		SkipCursorSync: *skipCursorSync,
		DryRun:         *dryRun,
		UseDefaults:    *useDefaults,
	}); err != nil {
		logError("sync failed: " + err.Error())
		return 1
	}

	return 0
}

func runDispatch(args []string) int {
	if len(args) > 0 {
		for _, arg := range args {
			if arg == "-h" || arg == "--help" {
				logInfo("Usage: dots dispatch [worker...]")
				return 0
			}
		}
	}

	if err := dispatcher.RunWorkers(context.Background(), args); err != nil {
		logError("dispatch failed: " + err.Error())
		return 1
	}

	return 0
}

func runPerf(args []string) int {
	if err := perfcmd.Run(args); err != nil {
		logError("perf failed: " + err.Error())
		return 1
	}
	return 0
}

func runSyncAgentRepo(args []string) int {
	if err := agentsync.Run(context.Background(), args...); err != nil {
		logError("sync-agent-repo failed: " + err.Error())
		return 1
	}
	return 0
}

func runRefreshShellCaches(args []string) int {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			logInfo("Usage: dots refresh-shell-caches")
			return 0
		}
	}
	if len(args) > 0 {
		logError("refresh-shell-caches does not accept arguments")
		return 2
	}
	if err := dispatcher.RunWorkers(context.Background(), []string{"prefer-cache-rebuild", "zwc-recompile"}); err != nil {
		logError("refresh-shell-caches failed: " + err.Error())
		return 1
	}
	return 0
}

func runCursorSync(args []string) int {
	if len(args) > 0 {
		for _, arg := range args {
			if arg == "-h" || arg == "--help" {
				logInfo("Usage: dots cursor-sync")
				return 0
			}
		}
	}
	if err := cursorSync.Run(); err != nil {
		logError("cursor sync failed: " + err.Error())
		return 1
	}
	return 0
}

func runInstall(args []string) int {
	if err := installer.Run(context.Background(), args...); err != nil {
		logError("install failed: " + err.Error())
		return 1
	}

	return 0
}

func runUninstall(args []string) int {
	if err := uninstaller.Run(context.Background(), args...); err != nil {
		logError("uninstall failed: " + err.Error())
		return 1
	}

	return 0
}

func printUsage() {
	logInfo("Usage:")
	logInfo("  dots sync [--repair] [--quick] [--skip-git] [--skip-network] [--skip-cursor-sync] [--dry-run] [--use-defaults]")
	logInfo("  dots dispatch [worker...]")
	logInfo("  dots perf [log|history|arm-zprof|rebuild-path-cache]")
	logInfo("  dots sync-agent-repo [path]")
	logInfo("  dots refresh-shell-caches")
	logInfo("  dots cursor-sync")
	logInfo("  dots install [--use-defaults] [--quick] [--skip-git] [--skip-network] [--repair]")
	logInfo("  dots uninstall [--purge-packages]")
	logInfo("  dots version")
}

func logInfo(message string) {
	if appLogger != nil {
		appLogger.Info(message)
	}
}

func logWarn(message string) {
	if appLogger != nil {
		appLogger.Warn(message)
	}
}

func logError(message string) {
	if appLogger != nil {
		appLogger.Error(message)
	}
}
