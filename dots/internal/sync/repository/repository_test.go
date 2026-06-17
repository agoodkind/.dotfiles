package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateGitRepoSyncRequiresSkipGitForDetachedHead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	repoRoot := t.TempDir()
	runGit(t, repoRoot, "init")
	hooksDirectory := filepath.Join(repoRoot, ".git", "hooks-disabled")
	if err := os.MkdirAll(hooksDirectory, 0o755); err != nil {
		t.Fatalf("creating hooks directory: %v", err)
	}
	runGit(t, repoRoot, "config", "core.hooksPath", hooksDirectory)
	runGit(t, repoRoot, "config", "user.name", "Smoke Test")
	runGit(t, repoRoot, "config", "user.email", "smoke@example.invalid")
	runGit(t, repoRoot, "config", "commit.gpgsign", "false")

	readmePath := filepath.Join(repoRoot, "README.md")
	if err := os.WriteFile(readmePath, []byte("smoke\n"), 0o644); err != nil {
		t.Fatalf("writing README.md: %v", err)
	}
	runGit(t, repoRoot, "add", "README.md")
	runGit(t, repoRoot, "commit", "-m", "Initial commit")

	headSHA := strings.TrimSpace(runGitOutput(t, repoRoot, "rev-parse", "HEAD"))
	runGit(t, repoRoot, "checkout", "--detach", headSHA)
	t.Setenv("DOTDOTFILES", repoRoot)

	err := UpdateGitRepoSync(context.Background(), false, nil)
	if err == nil {
		t.Fatal("UpdateGitRepoSync(skipGit=false) returned nil, want detached HEAD error")
	}
	if !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("UpdateGitRepoSync(skipGit=false) error = %v, want detached HEAD", err)
	}

	if err := UpdateGitRepoSync(context.Background(), true, nil); err != nil {
		t.Fatalf("UpdateGitRepoSync(skipGit=true) returned error: %v", err)
	}
}

func runGit(t *testing.T, repoRoot string, args ...string) {
	t.Helper()
	output := runGitOutput(t, repoRoot, args...)
	_ = output
}

func runGitOutput(t *testing.T, repoRoot string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
