package sync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunRefusesLinkedWorktree covers the failure that repointed this
// machine's home directory: a sync run from a linked worktree relinked every
// managed dotfile at that worktree and still exited zero.
//
// The refusal must also come before the sync lock is taken, so a refused run
// cannot block a real one.
func TestRunRefusesLinkedWorktree(t *testing.T) {
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

	err := Run(context.Background(), syncOptionsForTest(false))
	if err == nil {
		t.Fatal("Run from a linked worktree returned nil, want a refusal")
	}
	if !strings.Contains(err.Error(), "linked worktree") {
		t.Fatalf("Run error = %v, want it to name the linked worktree", err)
	}
	if !strings.Contains(err.Error(), canonicalRoot) && !strings.Contains(err.Error(), filepath.Base(canonicalRoot)) {
		t.Fatalf("Run error = %v, want it to name the canonical checkout %s", err, canonicalRoot)
	}

	// The refusal happens before the lock is acquired, so no lock file exists.
	if _, statErr := os.Stat(filepath.Join(home, ".cache", "dotfiles_sync.flock")); statErr == nil {
		t.Fatal("a refused run left a sync lock behind")
	}

	// The refusal is queued for the next login rather than only logged.
	notifications, readErr := os.ReadFile(filepath.Join(home, ".cache", "dotfiles", "notifications"))
	if readErr != nil {
		t.Fatalf("reading notifications: %v", readErr)
	}
	if !strings.Contains(string(notifications), "sync refused") {
		t.Fatalf("notifications = %q, want a refusal entry", notifications)
	}
}

// TestRunAcceptsLinkedWorktreeWithOverride confirms the escape hatch works, so
// the guard is a default rather than a wall.
func TestRunAcceptsLinkedWorktreeWithOverride(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("DOTFILES_LOG", filepath.Join(home, ".cache", "dotfiles", "sync.log"))

	canonicalRoot := newSyncTestRepo(t)
	worktreeRoot := filepath.Join(t.TempDir(), "linked")
	runSyncTestGit(t, canonicalRoot, "worktree", "add", "-b", "feature", worktreeRoot)
	t.Setenv("DOTDOTFILES", worktreeRoot)

	// Dry run keeps this from touching the real machine while still proving
	// the guard let the pipeline start.
	err := Run(context.Background(), syncOptionsForTest(true))
	if err != nil {
		t.Fatalf("Run with AllowWorktree returned error: %v", err)
	}
}

func syncOptionsForTest(allowWorktree bool) Options {
	return Options{
		RepairMode:     false,
		QuickMode:      true,
		SkipGit:        true,
		SkipNetwork:    true,
		SkipCorpusSync: true,
		SkipCursorSync: true,
		DryRun:         true,
		UseDefaults:    true,
		StrictMode:     false,
		AllowWorktree:  allowWorktree,
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
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("test\n"), 0o600); err != nil {
		t.Fatalf("writing README.md: %v", err)
	}
	runSyncTestGit(t, repoRoot, "add", "README.md")
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
