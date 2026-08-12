package logrotate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goodkind.io/.dotfiles/internal/telemetry"
)

// TestRotateOnlyTouchesLogsPastTheTrigger confirms rotation is size driven, so
// an ordinary small log is left alone while an oversized one is renamed aside
// and restarted.
func TestRotateOnlyTouchesLogsPastTheTrigger(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	oversized := filepath.Join(home, ".cache", "dotfiles", "dispatch.log")
	small := filepath.Join(home, ".cache", "dotfiles", "sync.log")
	writeLog(t, oversized, 2*bytesPerMB)
	writeLog(t, small, 1024)

	logger, _ := newTestLogger(t)
	rotation := DefaultRotationConfig()
	rotation.MaxSizeMB = 1

	if err := Rotate(context.Background(), rotation, logger); err != nil {
		t.Fatalf("Rotate returned error: %v", err)
	}
	waitForCompressedBackup(t, filepath.Dir(oversized), "dispatch")

	oversizedInfo, err := os.Stat(oversized)
	if err != nil {
		t.Fatalf("stat rotated log: %v", err)
	}
	if oversizedInfo.Size() != 0 {
		t.Fatalf("live log size = %d, want 0 after rotation", oversizedInfo.Size())
	}
	if backups := backupCount(t, filepath.Dir(oversized), "dispatch"); backups != 1 {
		t.Fatalf("backup count = %d, want 1", backups)
	}

	smallInfo, err := os.Stat(small)
	if err != nil {
		t.Fatalf("stat small log: %v", err)
	}
	if smallInfo.Size() != 1024 {
		t.Fatalf("small log size = %d, want 1024 (it should not have rotated)", smallInfo.Size())
	}
}

// TestRotateReportsAFailedRotation confirms a rotation failure is not
// swallowed. A rotation step that silently continued on error would report
// success to the sync runner even though a log was never rotated.
func TestRotateReportsAFailedRotation(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores directory write permissions, so this cannot fail")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	cacheDir := filepath.Join(home, ".cache")
	t.Setenv("XDG_CACHE_HOME", cacheDir)

	logDir := filepath.Join(cacheDir, "dotfiles")
	oversized := filepath.Join(logDir, "dispatch.log")
	writeLog(t, oversized, 2*bytesPerMB)

	// Rotation renames the log within its parent directory, which needs
	// write permission on that directory rather than on the file itself.
	if err := os.Chmod(logDir, 0o500); err != nil {
		t.Fatalf("removing write permission on %s: %v", logDir, err)
	}
	t.Cleanup(func() { _ = os.Chmod(logDir, 0o755) })

	logger, _ := newTestLogger(t)
	rotation := DefaultRotationConfig()
	rotation.MaxSizeMB = 1

	err := Rotate(context.Background(), rotation, logger)
	if err == nil {
		t.Fatal("Rotate returned nil for a log it could not rename, want an error")
	}
	if !strings.Contains(err.Error(), "dispatch.log") {
		t.Fatalf("Rotate error = %v, want it to name the log that failed to rotate", err)
	}
}

// TestRotateLeavesLogsThisRunHasOpen covers the case that made the first
// attempt at this step a no-op in practice: the updater runs a sync inside the
// dispatch process, so the dispatch log is open the whole time. Renaming it
// would send the rest of the run's output into the retired file.
//
// Everything else must still rotate, since skipping the whole step was what
// stopped any log from ever being rotated on that path.
func TestRotateLeavesLogsThisRunHasOpen(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	dispatchLog := filepath.Join(home, ".cache", "dotfiles", "dispatch.log")
	ownLog := filepath.Join(home, ".cache", "dotfiles", "sync.log")
	otherLog := filepath.Join(home, ".cache", "dotfiles", "dots.log")
	writeLog(t, dispatchLog, 2*bytesPerMB)
	writeLog(t, ownLog, 2*bytesPerMB)
	writeLog(t, otherLog, 2*bytesPerMB)

	t.Setenv("DOTFILES_DISPATCH_LOG", dispatchLog)
	t.Setenv("DOTFILES_LOG", ownLog)

	logger, logPath := newTestLogger(t)
	rotation := DefaultRotationConfig()
	rotation.MaxSizeMB = 1

	if err := Rotate(context.Background(), rotation, logger); err != nil {
		t.Fatalf("Rotate returned error: %v", err)
	}
	waitForCompressedBackup(t, filepath.Dir(otherLog), "dots")

	assertSize(t, dispatchLog, 2*bytesPerMB, "the dispatch log is open and must not rotate")
	assertSize(t, ownLog, 2*bytesPerMB, "this run's own log is open and must not rotate")
	assertSize(t, otherLog, 0, "an idle oversized log must rotate")

	if backups := backupCount(t, filepath.Dir(otherLog), "dots"); backups != 1 {
		t.Fatalf("backup count for the idle log = %d, want 1", backups)
	}

	if err := logger.Close(); err != nil {
		t.Fatalf("closing logger: %v", err)
	}
	logged := readFile(t, logPath)
	if !strings.Contains(logged, "in use by this run") {
		t.Fatalf("log does not name the logs it left alone: %s", logged)
	}
}

// TestPruneStartupLogsRemovesAbandonedSnapshotsAndOldLogs confirms the sweep
// keeps a shell that is still running, keeps the newest startup logs, and
// removes everything else.
func TestPruneStartupLogsRemovesAbandonedSnapshotsAndOldLogs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	startupDir := filepath.Join(home, ".cache", "zsh_startup")
	if err := os.MkdirAll(startupDir, 0o755); err != nil {
		t.Fatalf("creating startup dir: %v", err)
	}

	now := time.Now()
	abandonedSnapshot := filepath.Join(startupDir, syssnapPrefix+"111")
	liveSnapshot := filepath.Join(startupDir, syssnapPrefix+"222")
	writeAt(t, abandonedSnapshot, now.Add(-2*time.Hour))
	writeAt(t, liveSnapshot, now.Add(-time.Minute))

	// One more log than the retention limit, each a minute older than the last.
	totalLogs := startupLogRetention + 5
	logNames := make([]string, 0, totalLogs)
	for i := range totalLogs {
		name := filepath.Join(startupDir, formatLogName(i))
		writeAt(t, name, now.Add(-time.Duration(i)*time.Minute))
		logNames = append(logNames, name)
	}

	// A file that is neither a snapshot nor a startup log must survive.
	pathCache := filepath.Join(startupDir, "path_cache.zsh")
	writeAt(t, pathCache, now.Add(-72*time.Hour))

	logger, _ := newTestLogger(t)
	if err := PruneStartupLogs(context.Background(), logger); err != nil {
		t.Fatalf("PruneStartupLogs returned error: %v", err)
	}

	assertMissing(t, abandonedSnapshot)
	assertPresent(t, liveSnapshot)
	assertPresent(t, pathCache)

	for i, name := range logNames {
		if i < startupLogRetention {
			assertPresent(t, name)
		} else {
			assertMissing(t, name)
		}
	}
}

// TestPruneStartupLogsAcceptsAMissingDirectory confirms a host that has never
// run an interactive shell does not fail the sync step.
func TestPruneStartupLogsAcceptsAMissingDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	logger, _ := newTestLogger(t)
	if err := PruneStartupLogs(context.Background(), logger); err != nil {
		t.Fatalf("PruneStartupLogs returned error: %v", err)
	}
}

func formatLogName(index int) string {
	return "20260807_" + string(rune('a'+index/26)) + string(rune('a'+index%26)) + "_ttys000.json"
}

func writeLog(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating log dir: %v", err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
		t.Fatalf("writing log %s: %v", path, err)
	}
}

func writeAt(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("setting mod time on %s: %v", path, err)
	}
}

// waitForCompressedBackup blocks until dir holds exactly one backup with the
// given prefix and it is compressed.
//
// lumberjack.Logger.Rotate renames the oversized log aside synchronously, but
// compression and removal of the uncompressed copy happen afterward in a
// background goroutine it never waits on: the compressed file is written
// first and the plain one removed only once that succeeds, so both can exist
// briefly. Waiting for a lone .gz match, rather than any .gz match, is what
// closes that window. Without waiting at all, a test can return and
// t.TempDir's cleanup can race the goroutine, removing the directory while it
// is still being written to.
func waitForCompressedBackup(t *testing.T, dir string, prefix string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(dir)
		if err == nil {
			matches := 0
			compressed := 0
			for _, entry := range entries {
				name := entry.Name()
				if !strings.HasPrefix(name, prefix+"-") {
					continue
				}
				matches++
				if strings.HasSuffix(name, ".gz") {
					compressed++
				}
			}
			if matches == 1 && compressed == 1 {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for exactly one compressed backup with prefix %q in %s", prefix, dir)
}

func backupCount(t *testing.T, dir string, prefix string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	count := 0
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix+"-") {
			count++
		}
	}
	return count
}

func assertSize(t *testing.T, path string, want int, why string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Size() != int64(want) {
		t.Fatalf("%s size = %d, want %d: %s", filepath.Base(path), info.Size(), want, why)
	}
}

func newTestLogger(t *testing.T) (*telemetry.Logger, string) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "test.log")
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		t.Fatalf("creating logger: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	return logger, logPath
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(content)
}

func assertPresent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to survive, stat err = %v", path, err)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, stat err = %v", path, err)
	}
}
