package updater

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"goodkind.io/.dotfiles/internal/telemetry"
)

func TestRunUsesSyncDryRunForLinkedWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	canonicalRoot := newUpdaterTestRepo(t)
	worktreeRoot := filepath.Join(t.TempDir(), "linked")
	runUpdaterTestGit(t, canonicalRoot, "worktree", "add", "-b", "feature", worktreeRoot)

	dispatchLogPath := filepath.Join(homeDirectory, ".cache", "dotfiles", "dispatch.log")
	syncLogPath := filepath.Join(homeDirectory, ".cache", "dotfiles", "sync.log")
	t.Setenv("DOTFILES_LOG", syncLogPath)
	logger, err := telemetry.NewLogger(dispatchLogPath)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	runErr := Run(
		context.Background(),
		worktreeRoot,
		filepath.Join(homeDirectory, "status"),
		filepath.Join(homeDirectory, "weekly_update"),
		168,
		dispatchLogPath,
		logger,
	)
	if closeErr := logger.Close(); closeErr != nil {
		t.Fatalf("closing logger: %v", closeErr)
	}
	if runErr != nil {
		t.Fatalf("Run: %v", runErr)
	}

	dispatchLogContents, err := os.ReadFile(dispatchLogPath)
	if err != nil {
		t.Fatalf("reading dispatch log: %v", err)
	}
	dispatchLogText := string(dispatchLogContents)
	if !strings.Contains(dispatchLogText, "updater: linked worktree, running sync dry run") {
		t.Fatalf("dispatch log = %q, want linked-worktree sync path", dispatchLogText)
	}
	if !strings.Contains(dispatchLogText, "updater: sync exited successfully") {
		t.Fatalf("dispatch log = %q, want successful sync completion", dispatchLogText)
	}

	syncLogContents, err := os.ReadFile(syncLogPath)
	if err != nil {
		t.Fatalf("reading sync log: %v", err)
	}
	syncLogText := string(syncLogContents)
	if !strings.Contains(syncLogText, "dry-run: no changes applied") {
		t.Fatalf("sync log = %q, want dry-run pipeline output", syncLogText)
	}
	if strings.Contains(syncLogText, "FATAL: refusing") {
		t.Fatalf("sync log = %q, want no worktree refusal", syncLogText)
	}

	notificationPath := filepath.Join(homeDirectory, ".cache", "dotfiles", "notifications")
	notifications, err := os.ReadFile(notificationPath)
	if err != nil {
		t.Fatalf("reading notifications: %v", err)
	}
	notificationText := string(notifications)
	if !strings.Contains(notificationText, "|info|") || !strings.Contains(notificationText, "ran as a dry run from linked worktree") {
		t.Fatalf("notifications = %q, want informational worktree dry-run entry", notificationText)
	}
	if strings.Contains(notificationText, "|error|") || strings.Contains(notificationText, "sync refused") {
		t.Fatalf("notifications = %q, want no worktree error", notificationText)
	}
}

func newUpdaterTestRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	runUpdaterTestGit(t, repoRoot, "init")
	hooksDirectory := filepath.Join(repoRoot, ".git", "hooks-disabled")
	if err := os.MkdirAll(hooksDirectory, 0o755); err != nil {
		t.Fatalf("creating hooks directory: %v", err)
	}
	runUpdaterTestGit(t, repoRoot, "config", "core.hooksPath", hooksDirectory)
	runUpdaterTestGit(t, repoRoot, "config", "user.name", "Test")
	runUpdaterTestGit(t, repoRoot, "config", "user.email", "test@example.invalid")
	runUpdaterTestGit(t, repoRoot, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("test\n"), 0o600); err != nil {
		t.Fatalf("writing README: %v", err)
	}
	runUpdaterTestGit(t, repoRoot, "add", "README.md")
	runUpdaterTestGit(t, repoRoot, "commit", "-m", "Initial commit")
	return repoRoot
}

func runUpdaterTestGit(t *testing.T, directory string, args ...string) {
	t.Helper()

	command := exec.Command("git", args...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, directory, err, output)
	}
}
