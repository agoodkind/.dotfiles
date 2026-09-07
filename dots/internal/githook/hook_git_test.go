package githook

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestHumanCheckoutFeatureOnPrimaryIsAllowed(t *testing.T) {
	repo := newSeededRepo(t, "main")
	enableGlobalHooks(t, repo)
	if _, err := runGitResult(t, repo, "switch", "-c", "feature"); err != nil {
		t.Fatalf("human checkout feature: %v", err)
	}
}

func TestCursorOnlyCheckoutFeatureOnPrimaryIsAllowed(t *testing.T) {
	t.Setenv("CURSOR_AGENT", "1")
	repo := newSeededRepo(t, "main")
	enableGlobalHooks(t, repo)
	if _, err := runGitResult(t, repo, "switch", "-c", "feature"); err != nil {
		t.Fatalf("Cursor checkout feature: %v", err)
	}
}

func TestClaudeFetchAndResetToOriginMainAreAllowed(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	repo := newSeededRepo(t, "main")
	bare := t.TempDir()
	if _, err := runGitResult(t, bare, "init", "--bare"); err != nil {
		t.Fatalf("git init --bare: %v", err)
	}
	if _, err := runGitResult(t, repo, "remote", "add", "origin", bare); err != nil {
		t.Fatalf("git remote add: %v", err)
	}
	if _, err := runGitResult(t, repo, "push", "origin", "HEAD:main"); err != nil {
		t.Fatalf("git push origin: %v", err)
	}
	enableGlobalHooks(t, repo)
	if _, err := runGitResult(t, repo, "fetch", "origin"); err != nil {
		t.Fatalf("git fetch origin: %v", err)
	}
	output, err := runGitResult(t, repo, "reset", "--hard", "origin/main")
	if err != nil {
		t.Fatalf("git reset --hard origin/main: %v\n%s", err, output)
	}
}

func TestRunReferenceTransactionIgnoresNonPreparingState(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	var stderr bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"reference-transaction", "committed"},
		strings.NewReader("0000000000000000000000000000000000000000 ref:refs/heads/feature HEAD\n"),
		&stderr,
	)
	if err != nil {
		t.Fatalf("Run(committed) returned error: %v", err)
	}
}

func TestRunUnknownHookReturnsUsage(t *testing.T) {
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"post-commit"}, nil, &stderr)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("Run(post-commit) error = %v, want ErrUsage", err)
	}
}

func newSeededRepo(t *testing.T, branch string) string {
	t.Helper()
	t.Setenv("GIT_EMAIL_RULES", filepath.Join(t.TempDir(), "missing-email-rules"))
	root := t.TempDir()
	emptyHooks := filepath.Join(root, ".empty-hooks")
	if err := os.MkdirAll(emptyHooks, 0o755); err != nil {
		t.Fatalf("creating empty hooks dir: %v", err)
	}
	if _, err := runGitResult(t, root, "-c", "core.hooksPath="+emptyHooks, "init", "--initial-branch="+branch); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, err := runGitResult(t, root, "config", "core.hooksPath", emptyHooks); err != nil {
		t.Fatalf("config hooksPath: %v", err)
	}
	if _, err := runGitResult(t, root, "config", "user.name", "Test User"); err != nil {
		t.Fatalf("config user.name: %v", err)
	}
	if _, err := runGitResult(t, root, "config", "user.email", "test@example.invalid"); err != nil {
		t.Fatalf("config user.email: %v", err)
	}
	if _, err := runGitResult(t, root, "config", "commit.gpgsign", "false"); err != nil {
		t.Fatalf("config commit.gpgsign: %v", err)
	}
	if _, err := runGitResult(t, root, "commit", "--allow-empty", "-m", "seed"); err != nil {
		t.Fatalf("seed commit: %v", err)
	}
	return root
}

func enableGlobalHooks(t *testing.T, repo string) {
	t.Helper()
	if _, err := runGitResult(t, repo, "config", "core.hooksPath", globalHooksDir(t)); err != nil {
		t.Fatalf("enabling global hooks: %v", err)
	}
}

func globalHooksDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	return filepath.Join(root, "git-global-hooks")
}

func runGitResult(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	return string(output), err
}
