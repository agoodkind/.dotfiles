package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNotifyWritesTimestampedNotification(t *testing.T) {
	homeDirectory := t.TempDir()
	logPath := filepath.Join(homeDirectory, ".cache", "dotfiles", "sync.log")
	notificationPath := filepath.Join(homeDirectory, ".cache", "dotfiles", "notifications")

	t.Setenv("HOME", homeDirectory)

	if err := Notify("warn", "sync failed", logPath); err != nil {
		t.Fatalf("writing notification: %v", err)
	}

	notificationBytes, err := os.ReadFile(notificationPath)
	if err != nil {
		t.Fatalf("reading notification: %v", err)
	}

	notificationLine := strings.TrimSpace(string(notificationBytes))
	parts := strings.SplitN(notificationLine, "|", 4)
	if len(parts) != 4 {
		t.Fatalf("notification line has %d fields, want 4: %q", len(parts), notificationLine)
	}
	if _, err := time.Parse(displayTimestampFormat, parts[0]); err != nil {
		t.Fatalf("parsing notification timestamp %q: %v", parts[0], err)
	}
	if parts[1] != "warn" {
		t.Fatalf("notification level = %q, want warn", parts[1])
	}
	if parts[2] != logPath {
		t.Fatalf("notification log path = %q, want %q", parts[2], logPath)
	}
	if parts[3] != "sync failed" {
		t.Fatalf("notification message = %q, want sync failed", parts[3])
	}
}
