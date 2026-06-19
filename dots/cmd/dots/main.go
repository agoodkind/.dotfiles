// Package main is the entry point for the dots command-line tool.
package main

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	cursorSync "goodkind.io/.dotfiles/internal/cursor/syncer"
	dispatcher "goodkind.io/.dotfiles/internal/dispatch"
	installer "goodkind.io/.dotfiles/internal/install"
	perfcmd "goodkind.io/.dotfiles/internal/perf"
	"goodkind.io/.dotfiles/internal/runner"
	syncer "goodkind.io/.dotfiles/internal/sync"
	"goodkind.io/.dotfiles/internal/sync/compilation"
	"goodkind.io/.dotfiles/internal/telemetry"
	uninstaller "goodkind.io/.dotfiles/internal/uninstall"
)

type subcommand string

const (
	cmdSync               subcommand = "sync"
	cmdDispatch           subcommand = "dispatch"
	cmdPerf               subcommand = "perf"
	cmdRefreshShellCaches subcommand = "refresh-shell-caches"
	cmdCursorSync         subcommand = "cursor-sync"
	cmdInstall            subcommand = "install"
	cmdUninstall          subcommand = "uninstall"
	cmdVersion            subcommand = "version"
	cmdHelp               subcommand = "help"
	cmdHelpShort          subcommand = "-h"
	cmdHelpLong           subcommand = "--help"
)

var appLogger *telemetry.Logger

func main() {
	telemetry.ConfigureDefaultSlogFromEnv()
	slog.InfoContext(context.Background(), "dots process started")
	slog.DebugContext(context.Background(), "dots debug logging enabled")
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

	logger.InfoContext(context.Background(), "dots command started")

	if len(args) == 0 {
		printUsage()
		return 2
	}

	switch subcommand(args[0]) {
	case cmdSync:
		return runSync(args[1:])
	case cmdDispatch:
		return runDispatch(args[1:])
	case cmdPerf:
		return runPerf(args[1:])
	case cmdRefreshShellCaches:
		return runRefreshShellCaches(args[1:])
	case cmdCursorSync:
		return runCursorSync(args[1:])
	case cmdInstall:
		return runInstall(args[1:])
	case cmdUninstall:
		return runUninstall(args[1:])
	case cmdVersion:
		logInfo("dots 0.1.0")
		return 0
	case cmdHelp, cmdHelpShort, cmdHelpLong:
		printUsage()
		return 0
	default:
		logWarn("unknown command: " + args[0])
		printUsage()
		return 2
	}
}

func runSync(args []string) int {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			logInfo("Usage:")
			logInfo("  dots sync [--repair] [--quick] [--skip-git] [--skip-network] [--skip-cursor-sync] [--dry-run] [--use-defaults] [--strict]")
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
	strictMode := fs.Bool("strict", false, "fail on non-critical sync step failures")
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
		StrictMode:     *strictMode,
	}); err != nil {
		logError("sync failed", err)
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
		logError("dispatch failed", err)
		return 1
	}

	return 0
}

func runPerf(args []string) int {
	if err := perfcmd.Run(args); err != nil {
		logError("perf failed", err)
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
		logWarn("refresh-shell-caches does not accept arguments")
		return 2
	}
	if err := dispatcher.RunWorkers(context.Background(), []string{"prefer-cache-rebuild", "zwc-recompile"}); err != nil {
		logError("refresh-shell-caches failed", err)
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
	dotfiles := os.Getenv("DOTDOTFILES")
	if dotfiles == "" {
		dotfiles = filepath.Join(os.Getenv("HOME"), ".dotfiles")
	}
	source := compilation.ResolveCorpusSource(dotfiles)
	style := compilation.RuleRenderStyle{SkillsRelDir: "../skills"}
	rules, err := compilation.RenderRulesForUpload(source.Rules, style)
	if err != nil {
		logError("rendering corpus rules for cursor upload", err)
		return 1
	}
	if err := cursorSync.Run(rules); err != nil {
		logError("cursor sync failed", err)
		return 1
	}
	return 0
}

func runInstall(args []string) int {
	if err := installer.Run(context.Background(), args...); err != nil {
		logError("install failed", err)
		return 1
	}

	return 0
}

func runUninstall(args []string) int {
	if err := uninstaller.Run(context.Background(), args...); err != nil {
		logError("uninstall failed", err)
		return 1
	}

	return 0
}

func printUsage() {
	logInfo("Usage:")
	logInfo("  dots sync [--repair] [--quick] [--skip-git] [--skip-network] [--skip-cursor-sync] [--dry-run] [--use-defaults] [--strict]")
	logInfo("  dots dispatch [worker...]")
	logInfo("  dots perf [log|history|arm-zprof|rebuild-path-cache]")
	logInfo("  dots refresh-shell-caches")
	logInfo("  dots cursor-sync")
	logInfo("  dots install [--use-defaults] [--quick] [--skip-git] [--skip-network] [--repair] [--strict]")
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

func logError(message string, err error) {
	if appLogger != nil {
		appLogger.ErrorWithErr(message, err)
	}
}
