package telemetry

import (
	"encoding/json"
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

	const runID = "0123456789abcdef0123456789abcdef"
	if err := Notify("warn", "sync failed", logPath, runID); err != nil {
		t.Fatalf("writing notification: %v", err)
	}

	notificationBytes, err := os.ReadFile(notificationPath)
	if err != nil {
		t.Fatalf("reading notification: %v", err)
	}

	notificationLine := strings.TrimSpace(string(notificationBytes))
	parts := strings.SplitN(notificationLine, "|", 5)
	if len(parts) != 5 {
		t.Fatalf("notification line has %d fields, want 5: %q", len(parts), notificationLine)
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
	if parts[3] != runID {
		t.Fatalf("notification run id = %q, want %q", parts[3], runID)
	}
	if parts[4] != "sync failed" {
		t.Fatalf("notification message = %q, want sync failed", parts[4])
	}
}

func TestLoggerEmitsCorrelationIDsToFile(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "sync.log")
	logger, err := NewLogger(logPath)
	if err != nil {
		t.Fatalf("creating logger: %v", err)
	}
	ctx := WithRun(t.Context())
	wantTrace := RunID(ctx)
	logger.InfoContext(ctx, "correlated run line")
	logger.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}

	var found map[string]any
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if record["msg"] == "correlated run line" {
			found = record
			break
		}
	}
	if found == nil {
		t.Fatalf("log line not found in:\n%s", data)
	}
	if got, _ := found["trace_id"].(string); got != wantTrace {
		t.Fatalf("trace_id = %q, want %q", got, wantTrace)
	}
	if got, _ := found["span_id"].(string); len(got) != 16 {
		t.Fatalf("span_id = %q, want 16 hex chars", got)
	}
}

func TestWithRunMintsStableRunID(t *testing.T) {
	base := WithRun(t.Context())
	id := RunID(base)
	if len(id) != 32 {
		t.Fatalf("run id %q has length %d, want 32 hex chars", id, len(id))
	}
	if got := RunID(WithRun(base)); got != id {
		t.Fatalf("WithRun re-minted the run id: %q != %q", got, id)
	}
	if RunID(t.Context()) != "" {
		t.Fatal("RunID returned non-empty for a context without a run")
	}
}
