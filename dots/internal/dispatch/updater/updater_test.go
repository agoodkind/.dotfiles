package updater

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"goodkind.io/.dotfiles/internal/telemetry"
)

func TestRunSkipsLinkedWorktreeWithoutNotification(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	canonicalRoot := newUpdaterTestRepo(t)
	worktreeRoot := filepath.Join(t.TempDir(), "linked")
	runUpdaterTestGit(t, canonicalRoot, "worktree", "add", "-b", "feature", worktreeRoot)

	logPath := filepath.Join(homeDirectory, ".cache", "dotfiles", "dispatch.log")
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	runErr := Run(
		context.Background(),
		worktreeRoot,
		filepath.Join(homeDirectory, "status"),
		filepath.Join(homeDirectory, "weekly_update"),
		168,
		logPath,
		logger,
	)
	if closeErr := logger.Close(); closeErr != nil {
		t.Fatalf("closing logger: %v", closeErr)
	}
	if runErr != nil {
		t.Fatalf("Run: %v", runErr)
	}

	logContents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading dispatch log: %v", err)
	}
	if !strings.Contains(string(logContents), "updater: linked worktree, skipping") {
		t.Fatalf("dispatch log = %q, want linked-worktree skip", logContents)
	}

	notificationPath := filepath.Join(homeDirectory, ".cache", "dotfiles", "notifications")
	if _, err := os.Stat(notificationPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("notification path error = %v, want not exist", err)
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
