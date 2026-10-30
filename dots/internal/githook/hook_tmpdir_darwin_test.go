package githook_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReferenceTransactionUsesTMPDIRInSandbox(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolving repository path: %v", err)
	}
	t.Setenv("HOME", repo)
	t.Setenv("PATH", "/usr/bin:/bin:"+os.Getenv("PATH"))
	runHookTestGit(t, repo, "init", "--initial-branch=main")
	runHookTestGit(t, repo,
		"-c", "user.name=Test User", "-c", "user.email=test@example.invalid",
		"-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "seed")
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("finding the hook source path failed")
	}
	hooks := filepath.Join(filepath.Dir(sourceFile), "..", "..", "..", "git-global-hooks")
	runHookTestGit(t, repo, "config", "core.hooksPath", hooks)
	tempDir := filepath.Join(repo, "custom-temp")
	if err := os.Mkdir(tempDir, 0o700); err != nil {
		t.Fatalf("creating custom temporary directory: %v", err)
	}
	t.Setenv("TMPDIR", tempDir)
	head := runHookTestGit(t, repo, "rev-parse", "HEAD")
	profile := filepath.Join(hooks, "testdata", "reference-transaction.sb")
	command := exec.Command("/usr/bin/sandbox-exec",
		"-D", "GIT_DIR="+filepath.Join(repo, ".git"),
		"-D", "TMPDIR="+tempDir, "-f", profile,
		"/usr/bin/git", "-C", repo, "update-ref", "refs/remotes/origin/main", head)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("updating a Git reference in the sandbox: %v\n%s", err, output)
	}
	got := runHookTestGit(t, repo, "rev-parse", "refs/remotes/origin/main")
	if got != head {
		t.Fatalf("remote reference = %s, want %s", got, head)
	}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("reading custom temporary directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("hook left %d temporary files", len(entries))
	}
}

func runHookTestGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	command := exec.Command("/usr/bin/git", append([]string{"-C", repo}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}
