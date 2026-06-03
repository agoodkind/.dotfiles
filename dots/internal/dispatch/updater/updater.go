// Package updater implements the dotfiles repository update worker.
package updater

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"goodkind.io/.dotfiles/internal/clock"
	"goodkind.io/.dotfiles/internal/cmdexec"
	"goodkind.io/.dotfiles/internal/runner"
	syncer "goodkind.io/.dotfiles/internal/sync"
	"goodkind.io/.dotfiles/internal/sync/common"
	"goodkind.io/.dotfiles/internal/sync/repository"
	"goodkind.io/.dotfiles/internal/telemetry"
)

// Run executes the background updater worker, checking for new commits and running weekly maintenance.
func Run(
	ctx context.Context,
	dotfiles string,
	statusDir string,
	weeklyMarkerPath string,
	weeklyHours int64,
	notifyLogPath string,
	dispatchLogger *telemetry.Logger,
) error {
	ctx = telemetry.WithRun(ctx)
	notifyf := func(level string, message string) {
		if err := telemetry.Notify(level, message, notifyLogPath, telemetry.RunID(ctx)); err != nil {
			dispatchLogger.WarnContextWithErr(ctx, "notification write failed", err)
		}
	}

	if !hasInternet(ctx) {
		dispatchLogger.InfoContext(ctx, "updater: no internet, skipping")
		return nil
	}

	pulled, oldSHA, newSHA, err := repository.UpdateRepo(ctx, dotfiles, dispatchLogger)
	if err != nil {
		dispatchLogger.ErrorContextWithErr(ctx, "updater: fetch failed", err)
		return nil
	}

	if pulled {
		if oldSHA != "" && newSHA != "" {
			dispatchLogger.InfoContext(ctx, fmt.Sprintf(
				"updater: new changes found (%s -> %s), running sync",
				truncateSHA(oldSHA),
				truncateSHA(newSHA),
			))
		} else {
			dispatchLogger.InfoContext(ctx, "updater: new changes found, running sync")
		}
		if statusDir != "" {
			_ = os.WriteFile(filepath.Join(statusDir, "status"), []byte("sync"), 0o600)
		}
		if err := runSyncOnly(ctx, dotfiles, dispatchLogger); err != nil {
			dispatchLogger.WarnContextWithErr(ctx, "updater: sync step failed", err)
			notifyf("warn", "Dotfiles sync completed with non-critical issues")
		}
		notifyf("success", "Dotfiles updated in background")
		return nil
	}

	if !needsWeeklyUpdate(weeklyMarkerPath, weeklyHours) {
		dispatchLogger.InfoContext(ctx, "updater: no new changes and weekly update not due")
		return nil
	}

	lastUpdateDate := epochToDate(readEpoch(weeklyMarkerPath))
	dispatchLogger.InfoContext(ctx, fmt.Sprintf("updater: weekly update due (last: %s), running weekly update", lastUpdateDate))
	if statusDir != "" {
		_ = os.WriteFile(filepath.Join(statusDir, "status"), []byte("weekly"), 0o600)
	}
	if err := doWeeklyUpdate(ctx, dotfiles, weeklyMarkerPath, dispatchLogger); err != nil {
		dispatchLogger.ErrorContextWithErr(ctx, "updater: weekly update failed", err)
		notifyf("warn", "Weekly sync completed with non-critical issues")
		return err
	}
	notifyf("success", "Weekly sync completed (zinit, nvim, repair)")
	return nil
}

func runSyncOnly(ctx context.Context, dotfiles string, dispatchLogger *telemetry.Logger) error {
	if dotfiles != "" {
		_ = os.Setenv("DOTDOTFILES", dotfiles)
	}
	runErr := syncer.Run(ctx, syncer.Options{
		RepairMode:     false,
		QuickMode:      true,
		SkipGit:        true,
		SkipNetwork:    false,
		SkipCursorSync: false,
		DryRun:         false,
		UseDefaults:    true,
		StrictMode:     false,
	})
	if runErr != nil {
		dispatchLogger.WarnContextWithErr(ctx, "updater: sync exited with non-zero status", runErr)
		slog.WarnContext(ctx, "updater: sync exited with non-zero status", "err", runErr)
		return fmt.Errorf("running sync: %w", runErr)
	}
	dispatchLogger.InfoContext(ctx, "updater: sync exited successfully")
	return nil
}

func doWeeklyUpdate(ctx context.Context, dotfiles, weeklyMarkerPath string, dispatchLogger *telemetry.Logger) error {
	if dotfiles != "" {
		_ = os.Setenv("DOTDOTFILES", dotfiles)
	}
	runErr := syncer.Run(ctx, syncer.Options{
		RepairMode:     true,
		QuickMode:      false,
		SkipGit:        true,
		SkipNetwork:    false,
		SkipCursorSync: false,
		DryRun:         false,
		UseDefaults:    true,
		StrictMode:     false,
	})
	if runErr != nil {
		dispatchLogger.WarnContextWithErr(ctx, "updater: weekly sync exited with non-zero status", runErr)
		slog.WarnContext(ctx, "updater: weekly sync exited with non-zero status", "err", runErr)
		return fmt.Errorf("running weekly sync: %w", runErr)
	}
	dispatchLogger.InfoContext(ctx, "updater: weekly sync exited successfully")

	zinitPath := filepath.Join(dotfiles, "lib", "zinit", "zinit.zsh")
	if _, err := os.Stat(zinitPath); err == nil {
		_, _ = cmdexec.OutputWithLoggerAndEnv(
			ctx,
			dispatchLogger,
			append(os.Environ(), "DOTDOTFILES="+dotfiles),
			"zsh",
			"-c",
			"source '$DOTDOTFILES/lib/zinit/zinit.zsh'; zinit self-update; zinit update --all --quiet",
		)
	}

	doBrewUpgrade(ctx, dispatchLogger)
	doAptUpgrade(ctx, dispatchLogger)

	now := strconv.FormatInt(clock.Now().Unix(), 10)
	if weeklyMarkerPath == "" {
		weeklyMarkerPath = filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles_weekly_update")
	}
	_ = os.WriteFile(filepath.Clean(weeklyMarkerPath), []byte(now), 0o600)
	return nil
}

func needsWeeklyUpdate(weekPath string, weekDurationHours int64) bool {
	slog.Info("updater: needsWeeklyUpdate", "weekPath", weekPath)
	if _, err := os.Stat(weekPath); err != nil {
		return true
	}
	raw, err := os.ReadFile(weekPath)
	if err != nil {
		return true
	}
	last, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		currentTimestamp := clock.Now().Unix()
		_ = os.WriteFile(weekPath, []byte(strconv.FormatInt(currentTimestamp, 10)), 0o600)
		return false
	}
	if weekDurationHours == 0 {
		weekDurationHours = 168
	}
	threshold := weekDurationHours * 60 * 60
	return clock.Now().Unix()-last > threshold
}

func hasInternet(ctx context.Context) bool {
	d := net.Dialer{
		Timeout: 2 * time.Second,
	}
	conn, err := d.DialContext(ctx, "tcp", "8.8.8.8:53")
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func isUbuntu() bool {
	content, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	text := string(content)
	return strings.Contains(text, "ID=ubuntu") || strings.Contains(text, "NAME=\"Ubuntu\"")
}

func epochToDate(value int64) string {
	return time.Unix(value, 0).Format(time.RFC3339)
}

func readEpoch(path string) int64 {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	timestamp, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0
	}
	return timestamp
}

func truncateSHA(value string) string {
	const width = 7
	if len(value) <= width {
		return value
	}
	return value[:width]
}

func doBrewUpgrade(ctx context.Context, dispatchLogger *telemetry.Logger) {
	if runtime.GOOS != "darwin" {
		return
	}
	if !runner.HasCommand("brew") {
		return
	}
	_, _ = cmdexec.OutputWithLogger(ctx, dispatchLogger, "brew", "update")
	_, _ = cmdexec.OutputWithLogger(ctx, dispatchLogger, "brew", "upgrade")
	_, _ = cmdexec.OutputWithLogger(ctx, dispatchLogger, "brew", "upgrade", "--cask")
	_, _ = cmdexec.OutputWithLogger(ctx, dispatchLogger, "brew", "cleanup", "--prune=all")
}

func doAptUpgrade(ctx context.Context, dispatchLogger *telemetry.Logger) {
	if runtime.GOOS != "linux" || !isUbuntu() {
		return
	}
	if !runner.HasCommand("apt-get") {
		return
	}
	if _, err := common.OutputDebianPrivilegedCommand(ctx, dispatchLogger, "apt-get", "update"); err != nil {
		return
	}
	_, _ = common.OutputDebianPrivilegedCommand(
		ctx,
		dispatchLogger,
		"env",
		"DEBIAN_FRONTEND=noninteractive",
		"apt-get",
		"-y",
		"-o",
		"Dpkg::Options::=--force-confdef",
		"-o",
		"Dpkg::Options::=--force-confold",
		"dist-upgrade",
	)
	_, _ = common.OutputDebianPrivilegedCommand(ctx, dispatchLogger, "apt-get", "-y", "autoremove")
}
