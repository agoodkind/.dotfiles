package updater

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/agoodkind/.dotfiles/internal/cmdexec"
	"github.com/agoodkind/.dotfiles/internal/runner"
	syncer "github.com/agoodkind/.dotfiles/internal/sync"
	"github.com/agoodkind/.dotfiles/internal/sync/repository"
	"github.com/agoodkind/.dotfiles/internal/telemetry"
)

func Run(
	ctx context.Context,
	dotfiles string,
	statusDir string,
	weeklyMarkerPath string,
	weeklyHours int64,
	notifyLogPath string,
	dispatchLogger *telemetry.Logger,
) error {
	notifyf := func(level string, message string) {
		if err := telemetry.Notify(level, message, notifyLogPath); err != nil {
			dispatchLogger.Warn(fmt.Sprintf("notification write failed: %v", err))
		}
	}

	if !hasInternet(ctx) {
		dispatchLogger.Info("updater: no internet, skipping")
		return nil
	}

	pulled, oldSHA, newSHA, err := repository.UpdateRepo(ctx, dotfiles, dispatchLogger)
	if err != nil {
		dispatchLogger.Error(fmt.Sprintf("updater: fetch failed: %s", strings.TrimSpace(err.Error())))
		return nil
	}

	if pulled {
		if oldSHA != "" && newSHA != "" {
			dispatchLogger.Info(fmt.Sprintf(
				"updater: new changes found (%s -> %s), running sync",
				truncateSHA(oldSHA),
				truncateSHA(newSHA),
			))
		} else {
			dispatchLogger.Info("updater: new changes found, running sync")
		}
		if statusDir != "" {
			_ = os.WriteFile(filepath.Join(statusDir, "status"), []byte("sync"), 0o644)
		}
		if err := runSyncOnly(ctx, dotfiles, dispatchLogger); err != nil {
			dispatchLogger.Warn(fmt.Sprintf("updater: sync step failed: %v", err))
			notifyf("warn", "Dotfiles sync completed with non-critical issues")
		}
		notifyf("success", "Dotfiles updated in background")
		return nil
	}

	if !needsWeeklyUpdate(weeklyMarkerPath, weeklyHours) {
		dispatchLogger.Info("updater: no new changes and weekly update not due")
		return nil
	}

	lastUpdateDate := epochToDate(readEpoch(weeklyMarkerPath))
	dispatchLogger.Info(fmt.Sprintf("updater: weekly update due (last: %s), running weekly update", lastUpdateDate))
	if statusDir != "" {
		_ = os.WriteFile(filepath.Join(statusDir, "status"), []byte("weekly"), 0o644)
	}
	if err := doWeeklyUpdate(ctx, dotfiles, weeklyMarkerPath, dispatchLogger); err != nil {
		dispatchLogger.Error(fmt.Sprintf("updater: weekly update failed: %v", err))
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
		QuickMode:   true,
		SkipGit:     true,
		UseDefaults: true,
	})
	if runErr != nil {
		dispatchLogger.Warn("updater: sync exited with non-zero status")
		return runErr
	}
	dispatchLogger.Info("updater: sync exited successfully")
	return nil
}

func doWeeklyUpdate(ctx context.Context, dotfiles, weeklyMarkerPath string, dispatchLogger *telemetry.Logger) error {
	if dotfiles != "" {
		_ = os.Setenv("DOTDOTFILES", dotfiles)
	}
	runErr := syncer.Run(ctx, syncer.Options{
		RepairMode:  true,
		SkipGit:     true,
		UseDefaults: true,
	})
	if runErr != nil {
		dispatchLogger.Warn("updater: weekly sync exited with non-zero status")
		return runErr
	}
	dispatchLogger.Info("updater: weekly sync exited successfully")

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

	_ = doBrewUpgrade(ctx, dispatchLogger)
	_ = doAptUpgrade(ctx, dispatchLogger)

	now := strconv.FormatInt(time.Now().Unix(), 10)
	if weeklyMarkerPath == "" {
		weeklyMarkerPath = filepath.Join(os.Getenv("HOME"), ".cache", "dotfiles_weekly_update")
	}
	_ = os.WriteFile(weeklyMarkerPath, []byte(now), 0o644)
	return nil
}

func needsWeeklyUpdate(weekPath string, weekDurationHours int64) bool {
	if _, err := os.Stat(weekPath); err != nil {
		return true
	}
	raw, err := os.ReadFile(weekPath)
	if err != nil {
		return true
	}
	last, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		now := time.Now().Unix()
		_ = os.WriteFile(weekPath, []byte(strconv.FormatInt(now, 10)), 0o644)
		return false
	}
	if weekDurationHours == 0 {
		weekDurationHours = 168
	}
	threshold := weekDurationHours * 60 * 60
	return time.Now().Unix()-last > threshold
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

func doBrewUpgrade(ctx context.Context, dispatchLogger *telemetry.Logger) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	if !runner.HasCommand("brew") {
		return nil
	}
	_, _ = cmdexec.OutputWithLogger(ctx, dispatchLogger, "brew", "update")
	_, _ = cmdexec.OutputWithLogger(ctx, dispatchLogger, "brew", "upgrade")
	_, _ = cmdexec.OutputWithLogger(ctx, dispatchLogger, "brew", "upgrade", "--cask")
	_, _ = cmdexec.OutputWithLogger(ctx, dispatchLogger, "brew", "cleanup", "--prune=all")
	return nil
}

func doAptUpgrade(ctx context.Context, dispatchLogger *telemetry.Logger) error {
	if runtime.GOOS != "linux" || !isUbuntu() {
		return nil
	}
	if !runner.HasCommand("apt-get") {
		return nil
	}
	if _, err := cmdexec.OutputWithLogger(ctx, dispatchLogger, "sudo", "-n", "true"); err != nil {
		return nil
	}
	_, _ = cmdexec.OutputWithLogger(ctx, dispatchLogger, "sudo", "-n", "apt-get", "update")
	_, _ = cmdexec.OutputWithLogger(
		ctx,
		dispatchLogger,
		"sudo",
		"-n",
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
	_, _ = cmdexec.OutputWithLogger(ctx, dispatchLogger, "sudo", "-n", "apt-get", "-y", "autoremove")
	return nil
}
