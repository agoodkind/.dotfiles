// Package logrotate keeps the log and cache directories this binary writes to
// from growing without bound. It runs as sync steps rather than as part of any
// write path, so ordinary logging stays a plain append.
package logrotate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"goodkind.io/.dotfiles/internal/clock"
	"goodkind.io/.dotfiles/internal/telemetry"
	"goodkind.io/gklog"
)

const (
	// defaultMaxSizeMB is the size a log must exceed before it is rotated.
	defaultMaxSizeMB = 8
	// defaultMaxBackups is how many rotated generations of each log to keep.
	defaultMaxBackups = 3
	// defaultMaxAgeDays is how long a rotated generation is kept. Zero keeps
	// each generation until MaxBackups evicts it.
	defaultMaxAgeDays = 0
	// bytesPerMB converts the configured megabyte trigger into a byte size.
	bytesPerMB = 1024 * 1024

	// syssnapPrefix marks the per-shell system snapshots that .zshenv writes.
	syssnapPrefix = ".syssnap_"
	// syssnapMaxAge is how old a snapshot must be before it counts as
	// abandoned. The shell that wrote one consumes it while writing its
	// startup log, so anything older belongs to a shell that never got there.
	syssnapMaxAge = time.Hour
	// startupLogRetention is how many startup logs to keep, newest first.
	startupLogRetention = 500
	// startupLogSuffix is the extension of a startup performance log.
	startupLogSuffix = ".json"

	// dispatchLogEnvVar names the log a background dispatch has open. Dispatch
	// exports it, and a sync the updater starts runs inside that same process,
	// so this is how such a run recognises the log it must not rename.
	dispatchLogEnvVar = "DOTFILES_DISPATCH_LOG"
)

// DefaultRotationConfig returns the rotation settings used when the dispatch
// config supplies none.
func DefaultRotationConfig() gklog.RotationConfig {
	return gklog.RotationConfig{
		MaxSizeMB:  defaultMaxSizeMB,
		MaxBackups: defaultMaxBackups,
		MaxAgeDays: defaultMaxAgeDays,
		Compress:   nil,
		LocalTime:  nil,
	}
}

// Rotate rotates every log this binary writes that has grown past the
// configured trigger.
//
// A log this process currently has open is left alone. Rotation renames the
// live file, and the open descriptor would keep pointing at the renamed one,
// so the rest of this run's output would land in the file that was just
// retired. Every other log is safe to rotate.
func Rotate(ctx context.Context, rotation gklog.RotationConfig, logger *telemetry.Logger) error {
	maxSizeMB := rotation.MaxSizeMB
	if maxSizeMB <= 0 {
		maxSizeMB = defaultMaxSizeMB
	}
	triggerBytes := int64(maxSizeMB) * bytesPerMB

	rotated := make([]string, 0)
	deferred := make([]string, 0)
	failed := make([]string, 0)
	for _, logPath := range telemetry.KnownLogPaths() {
		info, err := os.Stat(filepath.Clean(logPath))
		if err != nil || info.Size() <= triggerBytes {
			continue
		}
		if isOpenByThisProcess(logPath) {
			deferred = append(deferred, filepath.Base(logPath))
			continue
		}
		if err := rotateOne(ctx, logPath, rotation); err != nil {
			failed = append(failed, filepath.Base(logPath))
			continue
		}
		rotated = append(rotated, fmt.Sprintf("%s (%d MB)", filepath.Base(logPath), info.Size()/bytesPerMB))
	}

	if len(rotated) > 0 {
		logger.InfoContext(ctx, "  rotated: "+strings.Join(rotated, ", "))
	} else {
		logger.InfoContext(ctx, "  no logs past the size trigger")
	}
	if len(deferred) > 0 {
		logger.InfoContext(ctx, "  in use by this run, left for a later sync: "+strings.Join(deferred, ", "))
	}
	if len(failed) > 0 {
		return fmt.Errorf("rotating logs: %s", strings.Join(failed, ", "))
	}
	return nil
}

// isOpenByThisProcess reports whether logPath is a log this process is writing
// to right now. Two are possible: the log this command opened, and the
// dispatch log when this command runs as part of a background dispatch.
func isOpenByThisProcess(logPath string) bool {
	target := filepath.Clean(logPath)
	for _, envName := range []string{"DOTFILES_LOG", dispatchLogEnvVar} {
		value := os.Getenv(envName)
		if value != "" && filepath.Clean(value) == target {
			return true
		}
	}
	return false
}

// rotateOne renames one log aside and starts a fresh file in its place. The
// rotator also prunes older generations according to rotation.
func rotateOne(ctx context.Context, logPath string, rotation gklog.RotationConfig) error {
	writer := gklog.NewLumberjackWriterWithConfig(logPath, rotation)
	if err := writer.Rotate(); err != nil {
		slog.WarnContext(ctx, "logrotate: rotateOne: rotating log", "path", logPath, "err", err)
		return fmt.Errorf("rotating %s: %w", logPath, err)
	}
	if err := writer.Close(); err != nil {
		slog.WarnContext(ctx, "logrotate: rotateOne: closing rotator", "path", logPath, "err", err)
	}
	return nil
}

// PruneStartupLogs bounds the shell startup cache directory.
//
// It removes abandoned per-shell snapshots and keeps only the newest
// [startupLogRetention] startup logs. The directory is read in a single pass,
// since it can hold tens of thousands of entries and a glob over it is slow.
func PruneStartupLogs(ctx context.Context, logger *telemetry.Logger) error {
	startupDir := filepath.Join(os.Getenv("HOME"), ".cache", "zsh_startup")
	entries, err := os.ReadDir(filepath.Clean(startupDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		slog.WarnContext(ctx, "logrotate: PruneStartupLogs: reading startup dir", "path", startupDir, "err", err)
		return fmt.Errorf("reading startup log directory: %w", err)
	}

	now := clock.Now()
	startupLogs := make([]startupLog, 0, len(entries))
	removedSnapshots := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		switch {
		case strings.HasPrefix(name, syssnapPrefix):
			if now.Sub(info.ModTime()) < syssnapMaxAge {
				continue
			}
			path, ok := containedPath(startupDir, name)
			if !ok {
				continue
			}
			if err := os.Remove(path); err == nil {
				removedSnapshots++
			}
		case strings.HasSuffix(name, startupLogSuffix):
			startupLogs = append(startupLogs, startupLog{name: name, modTime: info.ModTime()})
		}
	}

	removedLogs := removeOldestStartupLogs(ctx, startupDir, startupLogs)
	logger.InfoContext(ctx, fmt.Sprintf("  removed %d abandoned snapshot(s), %d old startup log(s)", removedSnapshots, removedLogs))
	return nil
}

// startupLog pairs a startup log's directory entry name with the time it was
// written. The name is kept rather than a full path so the containment check
// runs at the point of removal.
type startupLog struct {
	name    string
	modTime time.Time
}

// removeOldestStartupLogs keeps the newest [startupLogRetention] logs and
// removes the rest, returning how many it removed.
func removeOldestStartupLogs(ctx context.Context, startupDir string, logs []startupLog) int {
	if len(logs) <= startupLogRetention {
		return 0
	}
	sort.Slice(logs, func(i int, j int) bool {
		return logs[i].modTime.After(logs[j].modTime)
	})
	removed := 0
	failures := 0
	for _, log := range logs[startupLogRetention:] {
		path, ok := containedPath(startupDir, log.name)
		if !ok {
			failures++
			continue
		}
		if err := os.Remove(path); err != nil {
			failures++
			continue
		}
		removed++
	}
	if failures > 0 {
		slog.WarnContext(ctx, "logrotate: removeOldestStartupLogs: some logs could not be removed", "dir", startupDir, "failures", failures)
	}
	return removed
}

// containedPath joins name onto dir and reports whether the result stays
// inside dir. Directory entry names never contain a separator, so this can
// only reject a name the filesystem should not have produced.
func containedPath(dir string, name string) (string, bool) {
	cleanDir := filepath.Clean(dir)
	joined := filepath.Clean(filepath.Join(cleanDir, name))
	if filepath.Dir(joined) != cleanDir {
		return "", false
	}
	return joined, true
}
