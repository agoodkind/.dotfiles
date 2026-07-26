package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"goodkind.io/.dotfiles/internal/telemetry"
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

func TestDeclaredSubmodulePathsReadsAllConfiguredSubmodules(t *testing.T) {
	repoRoot := t.TempDir()
	gitmodules := filepath.Join(repoRoot, ".gitmodules")
	content := `[submodule "lib/zinit"]
	path = lib/zinit
	url = https://github.com/zdharma-continuum/zinit.git
	branch = main
[submodule "lib/zsh-defer"]
	path = lib/zsh-defer
	url = https://github.com/romkatv/zsh-defer.git
[submodule "lib/Claude-Opus-5-tools"]
	path = lib/Claude-Opus-5-tools
	url = https://github.com/Lunarsong/Claude-Opus-5-tools.git
	branch = main
`
	if err := os.WriteFile(gitmodules, []byte(content), 0o644); err != nil {
		t.Fatalf("writing .gitmodules: %v", err)
	}

	got, err := declaredSubmodulePaths(context.Background(), repoRoot, nil)
	if err != nil {
		t.Fatalf("declaredSubmodulePaths() returned error: %v", err)
	}
	want := []string{"lib/zinit", "lib/zsh-defer", "lib/Claude-Opus-5-tools"}
	if len(got) != len(want) {
		t.Fatalf("declaredSubmodulePaths() length = %d, want %d (%v)", len(got), len(want), got)
	}
	for index, wantPath := range want {
		if got[index] != wantPath {
			t.Fatalf("declaredSubmodulePaths()[%d] = %q, want %q (all: %v)", index, got[index], wantPath, got)
		}
	}
}

func TestDeclaredSubmodulePathsUsesGitConfigParsing(t *testing.T) {
	repoRoot := t.TempDir()
	content := `[submodule "logical name"]
	path = "lib/demo path" # local comment
	url = https://example.invalid/demo.git
	branch = "release"
`
	if err := os.WriteFile(filepath.Join(repoRoot, ".gitmodules"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing .gitmodules: %v", err)
	}

	paths, err := declaredSubmodulePaths(context.Background(), repoRoot, nil)
	if err != nil {
		t.Fatalf("declaredSubmodulePaths() returned error: %v", err)
	}
	if len(paths) != 1 || paths[0] != filepath.Join("lib", "demo path") {
		t.Fatalf("declaredSubmodulePaths() = %q, want quoted path without comment", paths)
	}
	branch, err := declaredSubmoduleBranch(
		context.Background(),
		repoRoot,
		filepath.Join("lib", "demo path"),
		nil,
	)
	if err != nil {
		t.Fatalf("declaredSubmoduleBranch() returned error: %v", err)
	}
	if branch != "release" {
		t.Fatalf("declaredSubmoduleBranch() = %q, want release", branch)
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

func TestDeclaredSubmodulePathsRejectsEscapingPath(t *testing.T) {
	repoRoot := t.TempDir()
	gitmodules := filepath.Join(repoRoot, ".gitmodules")
	content := `[submodule "bad"]
	path = ../outside
	url = https://example.invalid/outside.git
`
	if err := os.WriteFile(gitmodules, []byte(content), 0o644); err != nil {
		t.Fatalf("writing .gitmodules: %v", err)
	}

	_, err := declaredSubmodulePaths(context.Background(), repoRoot, nil)
	if err == nil {
		t.Fatal("declaredSubmodulePaths() returned nil, want containment error")
	}
	if !strings.Contains(err.Error(), "escapes repository root") {
		t.Fatalf("declaredSubmodulePaths() error = %v, want containment failure", err)
	}
}

func TestDeclaredSubmodulePathsRejectsRepositoryRoot(t *testing.T) {
	repoRoot := t.TempDir()
	gitmodules := filepath.Join(repoRoot, ".gitmodules")
	content := `[submodule "bad"]
	path = .
	url = https://example.invalid/root.git
`
	if err := os.WriteFile(gitmodules, []byte(content), 0o644); err != nil {
		t.Fatalf("writing .gitmodules: %v", err)
	}

	_, err := declaredSubmodulePaths(context.Background(), repoRoot, nil)
	if err == nil {
		t.Fatal("declaredSubmodulePaths() returned nil, want repository root error")
	}
	if !strings.Contains(err.Error(), "must be below repository root") {
		t.Fatalf("declaredSubmodulePaths() error = %v, want strict descendant failure", err)
	}
}

func TestSyncOneSubmoduleReturnsPullFailure(t *testing.T) {
	dotfiles := t.TempDir()
	subPath := filepath.Join("lib", "failing")
	submodule := filepath.Join(dotfiles, subPath)
	if err := os.MkdirAll(submodule, 0o755); err != nil {
		t.Fatalf("creating submodule directory: %v", err)
	}
	runGit(t, submodule, "init", "--initial-branch=main")
	runGit(t, submodule, "remote", "add", "origin", "https://example.invalid/missing.git")
	gitmodules := `[submodule "lib/failing"]
	path = lib/failing
	url = https://example.invalid/missing.git
	branch = main
`
	if err := os.WriteFile(filepath.Join(dotfiles, ".gitmodules"), []byte(gitmodules), 0o644); err != nil {
		t.Fatalf("writing .gitmodules: %v", err)
	}
	logger, err := telemetry.NewLogger(filepath.Join(t.TempDir(), "test.log"))
	if err != nil {
		t.Fatalf("creating logger: %v", err)
	}
	t.Cleanup(func() { _ = logger.Close() })

	err = syncOneSubmodule(context.Background(), dotfiles, subPath, logger)
	if err == nil {
		t.Fatal("syncOneSubmodule() returned nil, want pull failure")
	}
}

func TestSubmoduleBranchIsCurrentWhenRemoteCommitIsContained(t *testing.T) {
	_, submodule := createSubmoduleFixture(t, "main")

	current, err := submoduleBranchIsCurrent(
		context.Background(),
		submodule,
		"main",
		newTestLogger(t),
	)
	if err != nil {
		t.Fatalf("submoduleBranchIsCurrent() returned error: %v", err)
	}
	if !current {
		t.Fatal("submoduleBranchIsCurrent() = false, want true")
	}
}

func TestSyncDotfilesSubmodulesLeavesDirtySubmoduleWorktreeUnchanged(t *testing.T) {
	parent, submodule := createSubmoduleFixture(t, "main")
	dirtyContent := []byte("local work\n")
	trackedPath := filepath.Join(submodule, "tracked.txt")
	if err := os.WriteFile(trackedPath, dirtyContent, 0o644); err != nil {
		t.Fatalf("writing dirty submodule file: %v", err)
	}

	logger := newTestLogger(t)
	if err := syncDotfilesSubmodules(context.Background(), parent, logger); err != nil {
		t.Fatalf("syncDotfilesSubmodules() returned error for unchanged pointer: %v", err)
	}

	content, err := os.ReadFile(trackedPath)
	if err != nil {
		t.Fatalf("reading dirty submodule file: %v", err)
	}
	if string(content) != string(dirtyContent) {
		t.Fatalf("dirty submodule content = %q, want %q", content, dirtyContent)
	}
}

func TestRestoreSubmoduleWorktreesPreservesDirtyFiles(t *testing.T) {
	parent, submodule := createSubmoduleFixture(t, "main")
	dirtyContent := []byte("local work\n")
	trackedPath := filepath.Join(submodule, "tracked.txt")
	if err := os.WriteFile(trackedPath, dirtyContent, 0o644); err != nil {
		t.Fatalf("writing dirty submodule file: %v", err)
	}

	logger := newTestLogger(t)
	if err := restoreSubmoduleWorktrees(context.Background(), parent, logger); err != nil {
		t.Fatalf("restoreSubmoduleWorktrees() returned error: %v", err)
	}

	content, err := os.ReadFile(trackedPath)
	if err != nil {
		t.Fatalf("reading dirty submodule file: %v", err)
	}
	if string(content) != string(dirtyContent) {
		t.Fatalf("dirty submodule content = %q, want %q", content, dirtyContent)
	}
}

func TestSyncOneSubmoduleUsesBranchFromLogicalSectionName(t *testing.T) {
	parent, submodule := createSubmoduleFixture(t, "release")
	logger := newTestLogger(t)

	if err := syncOneSubmodule(
		context.Background(),
		parent,
		filepath.Join("lib", "demo"),
		logger,
	); err != nil {
		t.Fatalf("syncOneSubmodule() returned error: %v", err)
	}

	branch := strings.TrimSpace(runGitOutput(t, submodule, "branch", "--show-current"))
	if branch != "release" {
		t.Fatalf("submodule branch = %q, want release", branch)
	}
}

func createSubmoduleFixture(t *testing.T, branch string) (string, string) {
	t.Helper()
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")

	source := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("creating source repository: %v", err)
	}
	runGit(t, source, "init", "--initial-branch="+branch)
	configureTestRepository(t, source)
	if err := os.WriteFile(filepath.Join(source, "tracked.txt"), []byte("committed\n"), 0o644); err != nil {
		t.Fatalf("writing source file: %v", err)
	}
	runGit(t, source, "add", "tracked.txt")
	runGit(t, source, "commit", "-m", "Add tracked file")

	parent := filepath.Join(t.TempDir(), "parent")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatalf("creating parent repository: %v", err)
	}
	runGit(t, parent, "init", "--initial-branch=main")
	configureTestRepository(t, parent)
	runGit(
		t,
		parent,
		"-c",
		"protocol.file.allow=always",
		"submodule",
		"add",
		"--name",
		"logical-name",
		"-b",
		branch,
		source,
		filepath.Join("lib", "demo"),
	)
	runGit(t, parent, "add", ".gitmodules", filepath.Join("lib", "demo"))
	runGit(t, parent, "commit", "-m", "Add demo submodule")

	return parent, filepath.Join(parent, "lib", "demo")
}

func configureTestRepository(t *testing.T, repository string) {
	t.Helper()
	hooksDirectory := filepath.Join(repository, ".git", "hooks-disabled")
	if err := os.MkdirAll(hooksDirectory, 0o755); err != nil {
		t.Fatalf("creating hooks directory: %v", err)
	}
	runGit(t, repository, "config", "core.hooksPath", hooksDirectory)
	runGit(t, repository, "config", "user.name", "Smoke Test")
	runGit(t, repository, "config", "user.email", "smoke@example.invalid")
	runGit(t, repository, "config", "commit.gpgsign", "false")
}

func newTestLogger(t *testing.T) *telemetry.Logger {
	t.Helper()
	logger, err := telemetry.NewLogger(filepath.Join(t.TempDir(), "test.log"))
	if err != nil {
		t.Fatalf("creating logger: %v", err)
	}
	t.Cleanup(func() {
		if err := logger.Close(); err != nil {
			t.Errorf("closing logger: %v", err)
		}
	})
	return logger
}
