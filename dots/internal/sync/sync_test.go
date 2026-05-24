package sync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestRunExitsWhenSyncLockIsHeld(t *testing.T) {
	homeDirectory := t.TempDir()
	dotfilesDirectory := filepath.Join(homeDirectory, ".dotfiles")
	logPath := filepath.Join(homeDirectory, ".cache", "dotfiles", "sync.log")
	lockPath := filepath.Join(homeDirectory, ".cache", "dotfiles_sync.flock")

	t.Setenv("HOME", homeDirectory)
	t.Setenv("DOTDOTFILES", dotfilesDirectory)
	t.Setenv("DOTFILES_LOG", logPath)

	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatalf("creating lock directory: %v", err)
	}

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		t.Fatalf("opening lock file: %v", err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("holding sync lock: %v", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	err = Run(context.Background(), Options{
		QuickMode:      true,
		SkipGit:        true,
		SkipNetwork:    true,
		SkipCursorSync: true,
		UseDefaults:    true,
	})
	if err != nil {
		t.Fatalf("Run returned error while sync lock was held: %v", err)
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading sync log: %v", err)
	}
	logContent := string(logBytes)
	if !strings.Contains(logContent, "sync already running, exiting") {
		t.Fatalf("sync log did not contain lock skip message:\n%s", logContent)
	}
	if strings.Contains(logContent, "Dotfiles sync started") {
		t.Fatalf("sync log entered main sync pipeline while lock was held:\n%s", logContent)
	}

	notificationPath := filepath.Join(homeDirectory, ".cache", "dotfiles", "notifications")
	if _, err := os.Stat(notificationPath); !os.IsNotExist(err) {
		t.Fatalf("notification file exists after lock skip: %v", err)
	}
}
