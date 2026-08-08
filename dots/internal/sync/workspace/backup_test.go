package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"goodkind.io/.dotfiles/internal/telemetry"
)

// TestIsWithinDirRejectsSiblingWithSharedPrefix covers the containment bypass
// a plain strings.HasPrefix check misses: a sibling directory whose name
// starts with the same characters as root, but is not inside it.
func TestIsWithinDirRejectsSiblingWithSharedPrefix(t *testing.T) {
	if isWithinDir("/tmp/backup", "/tmp/backup-evil/x.bak") {
		t.Fatal("isWithinDir(/tmp/backup, /tmp/backup-evil/x.bak) = true, want false")
	}
}

// TestIsWithinDirAcceptsDescendantsAndSelf covers the positive cases: a
// nested descendant, and the root path itself.
func TestIsWithinDirAcceptsDescendantsAndSelf(t *testing.T) {
	if !isWithinDir("/tmp/backup", "/tmp/backup/rel/x.bak") {
		t.Fatal("isWithinDir(/tmp/backup, /tmp/backup/rel/x.bak) = false, want true")
	}
	if !isWithinDir("/tmp/backup", "/tmp/backup") {
		t.Fatal("isWithinDir(/tmp/backup, /tmp/backup) = false, want true")
	}
}

// TestClearExistingHomeFileBacksUpAndFreesThePath confirms the normal path:
// an existing real file is backed up, then removed so the caller can place a
// symlink there.
func TestClearExistingHomeFileBacksUpAndFreesThePath(t *testing.T) {
	if _, err := os.Stat("/usr/bin/cp"); err != nil {
		t.Skip("cp is not available")
	}

	root := t.TempDir()
	homeFile := filepath.Join(root, "home", ".zshrc")
	backupPath := filepath.Join(root, "backup")
	if err := os.MkdirAll(filepath.Dir(homeFile), 0o755); err != nil {
		t.Fatalf("creating home dir: %v", err)
	}
	if err := os.WriteFile(homeFile, []byte("existing content\n"), 0o600); err != nil {
		t.Fatalf("writing existing file: %v", err)
	}

	logger, _ := newBackupTestLogger(t)
	backedUp, pathIsFree := clearExistingHomeFile(context.Background(), homeFile, backupPath, ".zshrc", logger)

	if !backedUp {
		t.Fatal("clearExistingHomeFile did not back up an existing real file")
	}
	if !pathIsFree {
		t.Fatal("clearExistingHomeFile left the path occupied after a successful backup")
	}
	if _, err := os.Stat(homeFile); !os.IsNotExist(err) {
		t.Fatalf("homeFile still present after backup, stat err = %v", err)
	}
	backupContent, err := os.ReadFile(filepath.Join(backupPath, ".zshrc.bak"))
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(backupContent) != "existing content\n" {
		t.Fatalf("backup content = %q, want %q", backupContent, "existing content\n")
	}
}

func newBackupTestLogger(t *testing.T) (*telemetry.Logger, string) {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "test.log")
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		t.Fatalf("creating logger: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })
	return logger, logPath
}
