package sync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunForcesLinkedWorktreeDryRun prevents any caller from applying a sync
// from a linked worktree, even when the caller did not request a dry run.
func TestRunForcesLinkedWorktreeDryRun(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	logPath := filepath.Join(home, ".cache", "dotfiles", "sync.log")
	t.Setenv("DOTFILES_LOG", logPath)

	canonicalRoot := newSyncTestRepo(t)
	worktreeRoot := filepath.Join(t.TempDir(), "linked")
	runSyncTestGit(t, canonicalRoot, "worktree", "add", "-b", "feature", worktreeRoot)
	t.Setenv("DOTDOTFILES", worktreeRoot)

	err := Run(context.Background(), syncOptionsForTest())
	if err != nil {
		t.Fatalf("Run from a linked worktree returned error: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(home, ".cache", "dotfiles_sync.flock")); statErr != nil {
		t.Fatalf("sync lock was not created: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".worktree-probe")); !os.IsNotExist(statErr) {
		t.Fatalf("worktree probe was applied during dry run: %v", statErr)
	}

	logContents, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("reading sync log: %v", readErr)
	}
	logText := string(logContents)
	if !strings.Contains(logText, "dry-run: no changes applied") {
		t.Fatalf("sync log = %q, want dry-run pipeline output", logText)
	}
	if !strings.Contains(logText, "Pruning startup logs") {
		t.Fatalf("sync log = %q, want final dry-run pipeline step", logText)
	}
	mainCheckout, evalErr := filepath.EvalSymlinks(canonicalRoot)
	if evalErr != nil {
		t.Fatalf("resolving main checkout: %v", evalErr)
	}
	if !strings.Contains(logText, "Run sync from ") || !strings.Contains(logText, mainCheckout) {
		t.Fatalf("sync log = %q, want main checkout path %s", logText, mainCheckout)
	}
	if strings.Contains(logText, "FATAL: refusing") {
		t.Fatalf("sync log = %q, want no worktree refusal", logText)
	}

	notifications, readErr := os.ReadFile(filepath.Join(home, ".cache", "dotfiles", "notifications"))
	if readErr != nil {
		t.Fatalf("reading notifications: %v", readErr)
	}
	notificationText := string(notifications)
	if !strings.Contains(notificationText, "|info|") || !strings.Contains(notificationText, "ran as a dry run from linked worktree") {
		t.Fatalf("notifications = %q, want informational worktree dry-run entry", notificationText)
	}
	if strings.Contains(notificationText, "|error|") || strings.Contains(notificationText, "sync refused") {
		t.Fatalf("notifications = %q, want no worktree error", notificationText)
	}
}

func syncOptionsForTest() Options {
	return Options{
		RepairMode:     false,
		QuickMode:      true,
		SkipGit:        true,
		SkipNetwork:    true,
		SkipCorpusSync: true,
		SkipCursorSync: true,
		DryRun:         false,
		UseDefaults:    true,
		StrictMode:     false,
	}
}

func newSyncTestRepo(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	runSyncTestGit(t, repoRoot, "init")
	// The machine's global hooks refuse commits on a protected branch name, so
	// point this fixture at a hooks directory of its own.
	hooksDirectory := filepath.Join(repoRoot, ".git", "hooks-disabled")
	if err := os.MkdirAll(hooksDirectory, 0o755); err != nil {
		t.Fatalf("creating hooks directory: %v", err)
	}
	runSyncTestGit(t, repoRoot, "config", "core.hooksPath", hooksDirectory)
	runSyncTestGit(t, repoRoot, "config", "user.name", "Test")
	runSyncTestGit(t, repoRoot, "config", "user.email", "test@example.invalid")
	runSyncTestGit(t, repoRoot, "config", "commit.gpgsign", "false")
	if err := os.MkdirAll(filepath.Join(repoRoot, "home"), 0o755); err != nil {
		t.Fatalf("creating home source directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "home", ".worktree-probe"), []byte("test\n"), 0o600); err != nil {
		t.Fatalf("writing worktree probe: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("test\n"), 0o600); err != nil {
		t.Fatalf("writing README.md: %v", err)
	}
	runSyncTestGit(t, repoRoot, "add", "README.md", "home/.worktree-probe")
	runSyncTestGit(t, repoRoot, "commit", "-m", "Initial commit")
	return repoRoot
}

func runSyncTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, output)
	}
}
