package gitdir

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestResolveDistinguishesWorktreeFromCanonicalCheckout builds a real
// repository with a linked worktree and asserts that each is classified
// correctly. This is the classification a sync run depends on before it
// rewrites the home directory.
func TestResolveDistinguishesWorktreeFromCanonicalCheckout(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	canonicalRoot := newTestRepo(t)
	worktreeRoot := filepath.Join(t.TempDir(), "linked")
	runGit(t, canonicalRoot, "worktree", "add", "-b", "feature", worktreeRoot)

	canonical, err := Resolve(context.Background(), canonicalRoot, nil)
	if err != nil {
		t.Fatalf("Resolve(canonical) returned error: %v", err)
	}
	if canonical.IsWorktree {
		t.Fatalf("Resolve(canonical).IsWorktree = true, want false (gitDir=%s commonDir=%s)", canonical.GitDir, canonical.CommonDir)
	}

	linked, err := Resolve(context.Background(), worktreeRoot, nil)
	if err != nil {
		t.Fatalf("Resolve(worktree) returned error: %v", err)
	}
	if !linked.IsWorktree {
		t.Fatalf("Resolve(worktree).IsWorktree = false, want true (gitDir=%s commonDir=%s)", linked.GitDir, linked.CommonDir)
	}
	if linked.CommonDir != canonical.CommonDir {
		t.Fatalf("worktree common dir = %s, want %s", linked.CommonDir, canonical.CommonDir)
	}

	wantMain := canonicalPath(canonicalRoot)
	if linked.MainWorktree() != wantMain {
		t.Fatalf("MainWorktree() = %s, want %s", linked.MainWorktree(), wantMain)
	}
}

// TestLinkedWorktreeReportsFalseOutsideGit confirms a directory with no git at
// all is not treated as a worktree, so an archive install still syncs.
func TestLinkedWorktreeReportsFalseOutsideGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	// A temp dir can sit inside an unrelated repository, so point git at a
	// directory it must refuse to walk out of.
	plainDir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", plainDir)

	if _, isWorktree := LinkedWorktree(context.Background(), plainDir, nil); isWorktree {
		t.Fatal("LinkedWorktree(non-git dir) = true, want false")
	}
}

// TestModulesDirPointsAtSharedGitDir confirms submodule git directories are
// looked for under the shared directory, which is where git puts them for
// every worktree.
func TestModulesDirPointsAtSharedGitDir(t *testing.T) {
	t.Parallel()

	layout := Info{Root: "/repo", GitDir: "/repo/.git/worktrees/x", CommonDir: "/repo/.git", IsWorktree: true}
	if got, want := layout.ModulesDir(), filepath.Join("/repo/.git", "modules"); got != want {
		t.Fatalf("ModulesDir() = %s, want %s", got, want)
	}

	empty := Info{Root: "/repo", GitDir: "", CommonDir: "", IsWorktree: false}
	if got := empty.ModulesDir(); got != "" {
		t.Fatalf("ModulesDir() on zero Info = %s, want empty", got)
	}
	if got := empty.MainWorktree(); got != "" {
		t.Fatalf("MainWorktree() on zero Info = %s, want empty", got)
	}
}

func newTestRepo(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	runGit(t, repoRoot, "init")
	// The machine's global hooks refuse commits on a protected branch name, so
	// point this fixture at a hooks directory of its own.
	hooksDirectory := filepath.Join(repoRoot, ".git", "hooks-disabled")
	if err := os.MkdirAll(hooksDirectory, 0o755); err != nil {
		t.Fatalf("creating hooks directory: %v", err)
	}
	runGit(t, repoRoot, "config", "core.hooksPath", hooksDirectory)
	runGit(t, repoRoot, "config", "user.name", "Test")
	runGit(t, repoRoot, "config", "user.email", "test@example.invalid")
	runGit(t, repoRoot, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("test\n"), 0o600); err != nil {
		t.Fatalf("writing README.md: %v", err)
	}
	runGit(t, repoRoot, "add", "README.md")
	runGit(t, repoRoot, "commit", "-m", "Initial commit")
	return repoRoot
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, output)
	}
}
