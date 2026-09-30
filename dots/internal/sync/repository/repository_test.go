package repository

import (
	"context"
	"fmt"
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

func TestUpdateRepoUsesRecordedSubmoduleVersions(t *testing.T) {
	parent, submodule := createSubmoduleFixture(t, "main")
	remote := filepath.Join(t.TempDir(), "parent.git")
	runGit(t, parent, "clone", "--bare", parent, remote)
	runGit(t, parent, "remote", "add", "origin", remote)
	consumer := filepath.Join(t.TempDir(), "consumer")
	runGit(t, parent, "clone", remote, consumer)
	configureTestRepository(t, consumer)
	logger := newTestLogger(t)
	parentHead := strings.TrimSpace(runGitOutput(t, consumer, "rev-parse", "HEAD"))
	oldSubmodule := strings.TrimSpace(runGitOutput(t, submodule, "rev-parse", "HEAD"))
	updateRepoForTest(t, consumer, logger, false)
	consumerSubmodule := filepath.Join(consumer, "lib", "demo")
	ignoreFile := filepath.Join(t.TempDir(), "ignore")
	writeTestFile(t, ignoreFile, "cache.txt\n")
	runGit(t, consumerSubmodule, "config", "core.excludesFile", ignoreFile)
	writeTestFile(t, filepath.Join(consumerSubmodule, "cache.txt"), "regenerable cache\n")
	source := strings.TrimSpace(runGitOutput(t, submodule, "remote", "get-url", "origin"))
	writeTestFile(t, filepath.Join(source, "tracked.txt"), "upstream update\n")
	runGit(t, source, "add", "tracked.txt")
	runGit(t, source, "commit", "-m", "Update upstream source")
	updateRepoForTest(t, consumer, logger, false)
	if got := strings.TrimSpace(runGitOutput(t, consumerSubmodule, "rev-parse", "HEAD")); got != oldSubmodule {
		t.Fatalf("submodule advanced without parent update: %s", got)
	}
	if got := strings.TrimSpace(runGitOutput(t, consumer, "rev-parse", "HEAD")); got != parentHead {
		t.Fatalf("parent created local commit: %s", got)
	}
	if got := strings.TrimSpace(runGitOutput(t, consumer, "diff", "--cached")); got != "" {
		t.Fatalf("parent index changed: %s", got)
	}
	runGit(t, submodule, "fetch", "origin")
	runGit(t, submodule, "merge", "--ff-only", "origin/main")
	newSubmodule := strings.TrimSpace(runGitOutput(t, submodule, "rev-parse", "HEAD"))
	runGit(t, parent, "add", "lib/demo")
	runGit(t, parent, "commit", "-m", "Update recorded submodule version")
	runGit(t, parent, "push", "origin", "main")
	updateRepoForTest(t, consumer, logger, true)
	if got := strings.TrimSpace(runGitOutput(t, consumerSubmodule, "rev-parse", "HEAD")); got != newSubmodule {
		t.Fatalf("submodule = %s, want %s", got, newSubmodule)
	}
	if got := strings.TrimSpace(runGitOutput(t, consumer, "status", "--porcelain")); got != "" {
		t.Fatalf("consumer is dirty: %s", got)
	}
	updateRepoForTest(t, consumer, logger, false)
	runGit(t, parent, "update-index", "--cacheinfo", "160000,"+oldSubmodule+",lib/demo")
	runGit(t, parent, "commit", "-m", "Restore earlier recorded version")
	runGit(t, parent, "push", "origin", "main")
	updateRepoForTest(t, consumer, logger, true)
	if got := strings.TrimSpace(runGitOutput(t, consumerSubmodule, "rev-parse", "HEAD")); got != oldSubmodule {
		t.Fatalf("recorded rollback = %s, want %s", got, oldSubmodule)
	}
	cache, err := os.ReadFile(filepath.Join(consumerSubmodule, "cache.txt"))
	if err != nil || string(cache) != "regenerable cache\n" {
		t.Fatalf("ignored cache changed: %q, err = %v", cache, err)
	}
	writeTestFile(t, filepath.Join(consumer, "staged.txt"), "staged parent work\n")
	runGit(t, consumer, "add", "staged.txt")
	stagedBefore := runGitOutput(t, consumer, "diff", "--cached")
	updateRepoForTest(t, consumer, logger, false)
	if got := runGitOutput(t, consumer, "diff", "--cached"); got != stagedBefore {
		t.Fatalf("staged parent work changed: %s", got)
	}
}

func TestUpdateRepoPreservesSubmoduleLocalWork(t *testing.T) {
	for _, localCommit := range []bool{false, true} {
		t.Run(fmt.Sprintf("localCommit=%t", localCommit), func(t *testing.T) {
			parent, submodule := createSubmoduleFixture(t, "main")
			remote := filepath.Join(t.TempDir(), "parent.git")
			runGit(t, parent, "clone", "--bare", parent, remote)
			runGit(t, parent, "remote", "add", "origin", remote)
			consumer := filepath.Join(t.TempDir(), "consumer")
			runGit(t, parent, "clone", remote, consumer)
			configureTestRepository(t, consumer)
			logger := newTestLogger(t)
			updateRepoForTest(t, consumer, logger, false)
			consumerSubmodule := filepath.Join(consumer, "lib", "demo")
			runGit(t, consumerSubmodule, "config", "user.name", "Smoke Test")
			runGit(t, consumerSubmodule, "config", "user.email", "smoke@example.invalid")
			runGit(t, consumerSubmodule, "config", "commit.gpgsign", "false")
			runGit(t, consumerSubmodule, "config", "core.hooksPath", t.TempDir())
			writeTestFile(t, filepath.Join(consumerSubmodule, "tracked.txt"), "local work\n")
			if localCommit {
				runGit(t, consumerSubmodule, "add", "tracked.txt")
				runGit(t, consumerSubmodule, "commit", "-m", "Add local submodule work")
			} else {
				updateRepoForTest(t, consumer, logger, false)
			}
			source := strings.TrimSpace(runGitOutput(t, submodule, "remote", "get-url", "origin"))
			writeTestFile(t, filepath.Join(source, "tracked.txt"), "upstream work\n")
			runGit(t, source, "add", "tracked.txt")
			runGit(t, source, "commit", "-m", "Update upstream source")
			runGit(t, submodule, "fetch", "origin")
			runGit(t, submodule, "merge", "--ff-only", "origin/main")
			runGit(t, parent, "add", "lib/demo")
			runGit(t, parent, "commit", "-m", "Update recorded version")
			runGit(t, parent, "push", "origin", "main")
			oldParent := strings.TrimSpace(runGitOutput(t, consumer, "rev-parse", "HEAD"))
			oldSubmodule := strings.TrimSpace(runGitOutput(t, consumerSubmodule, "rev-parse", "HEAD"))
			_, _, _, err := UpdateRepo(context.Background(), consumer, logger)
			if err == nil || !strings.Contains(err.Error(), "submodule lib/demo has local") {
				t.Fatalf("UpdateRepo error = %v, want precise local work error", err)
			}
			if got := strings.TrimSpace(runGitOutput(t, consumer, "rev-parse", "HEAD")); got != oldParent {
				t.Fatalf("parent advanced before local work check: %s", got)
			}
			if got := strings.TrimSpace(runGitOutput(t, consumerSubmodule, "rev-parse", "HEAD")); got != oldSubmodule {
				t.Fatalf("local submodule HEAD changed: %s", got)
			}
			content, err := os.ReadFile(filepath.Join(consumerSubmodule, "tracked.txt"))
			if err != nil || string(content) != "local work\n" {
				t.Fatalf("local content = %q, err = %v", content, err)
			}
		})
	}
}

func updateRepoForTest(t *testing.T, repository string, logger *telemetry.Logger, wantPulled bool) {
	t.Helper()
	pulled, _, _, err := UpdateRepo(context.Background(), repository, logger)
	if err != nil {
		t.Fatalf("UpdateRepo: %v", err)
	}
	if pulled != wantPulled {
		t.Fatalf("UpdateRepo pulled = %t, want %t", pulled, wantPulled)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateRepoUpdatesNestedRecordedVersion(t *testing.T) {
	parent, outer := createSubmoduleFixture(t, "main")
	source := strings.TrimSpace(runGitOutput(t, outer, "remote", "get-url", "origin"))
	nested := filepath.Join(t.TempDir(), "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, nested, "init", "--initial-branch=main")
	configureTestRepository(t, nested)
	writeTestFile(t, filepath.Join(nested, "data"), "old\n")
	runGit(t, nested, "add", "data")
	runGit(t, nested, "commit", "-m", "Initial nested")
	runGit(t, source, "submodule", "add", nested, "nested")
	runGit(t, source, "commit", "-am", "Add nested")
	runGit(t, outer, "fetch", "origin")
	runGit(t, outer, "merge", "--ff-only", "origin/main")
	runGit(t, parent, "commit", "-am", "Record nested parent")
	remote := filepath.Join(t.TempDir(), "parent.git")
	runGit(t, parent, "clone", "--bare", parent, remote)
	runGit(t, parent, "remote", "add", "origin", remote)
	consumer := filepath.Join(t.TempDir(), "consumer")
	runGit(t, parent, "clone", remote, consumer)
	configureTestRepository(t, consumer)
	runGit(t, consumer, "config", "fetch.recurseSubmodules", "false")
	logger := newTestLogger(t)
	updateRepoForTest(t, consumer, logger, false)
	nestedConsumer := filepath.Join(consumer, "lib", "demo", "nested")
	ignoreFile := filepath.Join(t.TempDir(), "ignore")
	writeTestFile(t, ignoreFile, "cache.txt\n")
	runGit(t, nestedConsumer, "config", "core.excludesFile", ignoreFile)
	writeTestFile(t, filepath.Join(nestedConsumer, "cache.txt"), "nested cache\n")
	writeTestFile(t, filepath.Join(nested, "data"), "new\n")
	runGit(t, nested, "commit", "-am", "Update nested")
	runGit(t, filepath.Join(source, "nested"), "fetch", "origin")
	runGit(t, filepath.Join(source, "nested"), "merge", "--ff-only", "origin/main")
	runGit(t, source, "commit", "-am", "Record nested update")
	runGit(t, outer, "fetch", "origin")
	runGit(t, outer, "merge", "--ff-only", "origin/main")
	runGit(t, parent, "commit", "-am", "Record outer update")
	runGit(t, parent, "push", "origin", "main")
	_, _, _, err := UpdateRepo(context.Background(), consumer, logger)
	if err != nil {
		t.Fatalf("nested update failed: %v", err)
	}
	expected := strings.TrimSpace(runGitOutput(t, nested, "rev-parse", "HEAD"))
	if got := strings.TrimSpace(runGitOutput(t, nestedConsumer, "rev-parse", "HEAD")); got != expected {
		t.Fatalf("nested HEAD = %s, want %s", got, expected)
	}
	content, err := os.ReadFile(filepath.Join(nestedConsumer, "data"))
	if err != nil || string(content) != "new\n" {
		t.Fatalf("nested content = %q, err = %v", content, err)
	}
	cache, err := os.ReadFile(filepath.Join(nestedConsumer, "cache.txt"))
	if err != nil || string(cache) != "nested cache\n" {
		t.Fatalf("nested ignored cache changed: %q, err = %v", cache, err)
	}
}

func TestUpdateRepoPreservesStagedParentChangesDuringFastForward(t *testing.T) {
	parent, _ := createSubmoduleFixture(t, "main")
	writeTestFile(t, filepath.Join(parent, "local.txt"), "base\n")
	runGit(t, parent, "add", "local.txt")
	runGit(t, parent, "commit", "-m", "Add local file")
	remote := filepath.Join(t.TempDir(), "parent.git")
	runGit(t, parent, "clone", "--bare", parent, remote)
	runGit(t, parent, "remote", "add", "origin", remote)
	consumer := filepath.Join(t.TempDir(), "consumer")
	runGit(t, parent, "clone", remote, consumer)
	configureTestRepository(t, consumer)
	logger := newTestLogger(t)
	updateRepoForTest(t, consumer, logger, false)
	writeTestFile(t, filepath.Join(consumer, "local.txt"), "staged work\n")
	runGit(t, consumer, "add", "local.txt")
	before := runGitOutput(t, consumer, "diff", "--cached")
	writeTestFile(t, filepath.Join(parent, "upstream.txt"), "upstream\n")
	runGit(t, parent, "add", "upstream.txt")
	runGit(t, parent, "commit", "-m", "Add upstream")
	runGit(t, parent, "push", "origin", "main")
	updateRepoForTest(t, consumer, logger, true)
	after := runGitOutput(t, consumer, "diff", "--cached")
	if strings.TrimSpace(before) != strings.TrimSpace(after) {
		t.Fatalf("staged work lost index state: before=%q after=%q", before, after)
	}
}

func TestUpdateRepoPreservesStagedGitlink(t *testing.T) {
	parent, outer := createSubmoduleFixture(t, "main")
	remote := filepath.Join(t.TempDir(), "parent.git")
	runGit(t, parent, "clone", "--bare", parent, remote)
	runGit(t, parent, "remote", "add", "origin", remote)
	consumer := filepath.Join(t.TempDir(), "consumer")
	runGit(t, parent, "clone", remote, consumer)
	configureTestRepository(t, consumer)
	logger := newTestLogger(t)
	updateRepoForTest(t, consumer, logger, false)
	source := strings.TrimSpace(runGitOutput(t, outer, "remote", "get-url", "origin"))
	writeTestFile(t, filepath.Join(source, "tracked.txt"), "new\n")
	runGit(t, source, "commit", "-am", "Update child")
	next := strings.TrimSpace(runGitOutput(t, source, "rev-parse", "HEAD"))
	child := filepath.Join(consumer, "lib", "demo")
	old := strings.TrimSpace(runGitOutput(t, child, "rev-parse", "HEAD"))
	runGit(t, child, "fetch", "origin")
	runGit(t, consumer, "update-index", "--cacheinfo", "160000,"+next+",lib/demo")
	parentBefore := strings.TrimSpace(runGitOutput(t, consumer, "rev-parse", "HEAD"))
	indexBefore := runGitOutput(t, consumer, "diff", "--cached")
	_, _, _, err := UpdateRepo(context.Background(), consumer, logger)
	if err == nil || !strings.Contains(err.Error(), "staged gitlink") {
		t.Fatalf("UpdateRepo error = %v, want staged gitlink error", err)
	}
	if got := strings.TrimSpace(runGitOutput(t, consumer, "rev-parse", "HEAD")); got != parentBefore {
		t.Fatalf("parent HEAD changed: %s", got)
	}
	if got := runGitOutput(t, consumer, "diff", "--cached"); got != indexBefore {
		t.Fatalf("staged gitlink changed: %s", got)
	}
	if got := strings.TrimSpace(runGitOutput(t, child, "rev-parse", "HEAD")); got != old {
		t.Fatalf("checkout used staged gitlink: got=%s recorded=%s", got, old)
	}
}

func TestUpdateRepoPreservesCheckoutWhenRecordedCommitFetchFails(t *testing.T) {
	parent, submodule := createSubmoduleFixture(t, "main")
	remote := filepath.Join(t.TempDir(), "parent.git")
	runGit(t, parent, "clone", "--bare", parent, remote)
	runGit(t, parent, "remote", "add", "origin", remote)
	consumer := filepath.Join(t.TempDir(), "consumer")
	runGit(t, parent, "clone", remote, consumer)
	configureTestRepository(t, consumer)
	runGit(t, consumer, "config", "fetch.recurseSubmodules", "false")
	logger := newTestLogger(t)
	updateRepoForTest(t, consumer, logger, false)
	child := filepath.Join(consumer, "lib", "demo")
	parentBefore := strings.TrimSpace(runGitOutput(t, consumer, "rev-parse", "HEAD"))
	childBefore := strings.TrimSpace(runGitOutput(t, child, "rev-parse", "HEAD"))
	source := strings.TrimSpace(runGitOutput(t, submodule, "remote", "get-url", "origin"))
	writeTestFile(t, filepath.Join(source, "tracked.txt"), "next recorded version\n")
	runGit(t, source, "commit", "-am", "Update source")
	runGit(t, submodule, "fetch", "origin")
	runGit(t, submodule, "merge", "--ff-only", "origin/main")
	runGit(t, parent, "commit", "-am", "Update recorded version")
	runGit(t, parent, "push", "origin", "main")
	runGit(t, child, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "missing.git"))
	_, _, _, err := UpdateRepo(context.Background(), consumer, logger)
	if err == nil || !strings.Contains(err.Error(), "fetching recorded commit") {
		t.Fatalf("UpdateRepo error = %v, want recorded commit fetch error", err)
	}
	if got := strings.TrimSpace(runGitOutput(t, consumer, "rev-parse", "HEAD")); got != parentBefore {
		t.Fatalf("parent changed after failed fetch: %s", got)
	}
	if got := strings.TrimSpace(runGitOutput(t, child, "rev-parse", "HEAD")); got != childBefore {
		t.Fatalf("child changed after failed fetch: %s", got)
	}
	if got := strings.TrimSpace(runGitOutput(t, consumer, "status", "--porcelain")); got != "" {
		t.Fatalf("checkout changed after failed fetch: %s", got)
	}
}

func TestUpdateRepoPreservesIgnoredChildFile(t *testing.T) {
	parent, outer := createSubmoduleFixture(t, "main")
	source := strings.TrimSpace(runGitOutput(t, outer, "remote", "get-url", "origin"))
	writeTestFile(t, filepath.Join(source, ".gitignore"), "ignored.txt\n")
	runGit(t, source, "add", ".gitignore")
	runGit(t, source, "commit", "-m", "Ignore local file")
	runGit(t, outer, "fetch", "origin")
	runGit(t, outer, "merge", "--ff-only", "origin/main")
	runGit(t, parent, "commit", "-am", "Record ignore")
	remote := filepath.Join(t.TempDir(), "parent.git")
	runGit(t, parent, "clone", "--bare", parent, remote)
	runGit(t, parent, "remote", "add", "origin", remote)
	consumer := filepath.Join(t.TempDir(), "consumer")
	runGit(t, parent, "clone", remote, consumer)
	configureTestRepository(t, consumer)
	logger := newTestLogger(t)
	updateRepoForTest(t, consumer, logger, false)
	local := filepath.Join(consumer, "lib", "demo", "ignored.txt")
	writeTestFile(t, local, "unique local file\n")
	writeTestFile(t, filepath.Join(source, "ignored.txt"), "tracked upstream\n")
	runGit(t, source, "add", "--force", "ignored.txt")
	runGit(t, source, "commit", "-m", "Track ignored file")
	runGit(t, outer, "fetch", "origin")
	runGit(t, outer, "merge", "--ff-only", "origin/main")
	runGit(t, parent, "commit", "-am", "Record upstream file")
	runGit(t, parent, "push", "origin", "main")
	_, _, _, updateErr := UpdateRepo(context.Background(), consumer, logger)
	if updateErr == nil {
		t.Fatal("want local ignored file collision error")
	}
	content, err := os.ReadFile(local)
	if err != nil || string(content) != "unique local file\n" {
		t.Fatalf("ignored local file overwritten: %q err=%v", content, err)
	}
}

func TestUpdateRepoPreservesIgnoredParentFile(t *testing.T) {
	parent, _ := createSubmoduleFixture(t, "main")
	writeTestFile(t, filepath.Join(parent, ".gitignore"), "ignored.txt\n")
	runGit(t, parent, "add", ".gitignore")
	runGit(t, parent, "commit", "-m", "Ignore local parent file")
	remote := filepath.Join(t.TempDir(), "parent.git")
	runGit(t, parent, "clone", "--bare", parent, remote)
	runGit(t, parent, "remote", "add", "origin", remote)
	consumer := filepath.Join(t.TempDir(), "consumer")
	runGit(t, parent, "clone", remote, consumer)
	configureTestRepository(t, consumer)
	logger := newTestLogger(t)
	updateRepoForTest(t, consumer, logger, false)
	local := filepath.Join(consumer, "ignored.txt")
	writeTestFile(t, local, "unique local parent file\n")
	writeTestFile(t, filepath.Join(parent, "ignored.txt"), "tracked upstream\n")
	runGit(t, parent, "add", "--force", "ignored.txt")
	runGit(t, parent, "commit", "-m", "Track ignored parent file")
	runGit(t, parent, "push", "origin", "main")
	_, _, _, updateErr := UpdateRepo(context.Background(), consumer, logger)
	if updateErr == nil {
		t.Fatal("want local ignored file collision error")
	}
	content, err := os.ReadFile(local)
	if err != nil || string(content) != "unique local parent file\n" {
		t.Fatalf("ignored parent file overwritten: %q err=%v", content, err)
	}
}

func TestUpdateRepoRollsBackPartiallyUpdatedSubmodules(t *testing.T) {
	parent, outer := createSubmoduleFixture(t, "main")
	source := strings.TrimSpace(runGitOutput(t, outer, "remote", "get-url", "origin"))
	remote := filepath.Join(t.TempDir(), "parent.git")
	runGit(t, parent, "clone", "--bare", parent, remote)
	runGit(t, parent, "remote", "add", "origin", remote)
	consumer := filepath.Join(t.TempDir(), "consumer")
	runGit(t, parent, "clone", remote, consumer)
	configureTestRepository(t, consumer)
	logger := newTestLogger(t)
	updateRepoForTest(t, consumer, logger, false)
	child := filepath.Join(consumer, "lib", "demo")
	old := strings.TrimSpace(runGitOutput(t, child, "rev-parse", "HEAD"))
	writeTestFile(t, filepath.Join(source, "tracked.txt"), "new\n")
	runGit(t, source, "commit", "-am", "Update child")
	runGit(t, outer, "fetch", "origin")
	runGit(t, outer, "merge", "--ff-only", "origin/main")
	runGit(t, parent, "config", "--file", ".gitmodules", "submodule.missing.path", "lib/missing")
	runGit(t, parent, "config", "--file", ".gitmodules", "submodule.missing.url", filepath.Join(t.TempDir(), "does-not-exist"))
	runGit(t, parent, "update-index", "--add", "--cacheinfo", "160000,"+old+",lib/missing")
	runGit(t, parent, "add", "lib/demo", ".gitmodules")
	runGit(t, parent, "commit", "-m", "Update child and add unavailable module")
	runGit(t, parent, "push", "origin", "main")
	_, _, _, err := UpdateRepo(context.Background(), consumer, logger)
	if err == nil {
		t.Fatal("want unavailable submodule error")
	}
	if got := strings.TrimSpace(runGitOutput(t, child, "rev-parse", "HEAD")); got != old {
		t.Fatalf("rollback left advanced child: got=%s old=%s error=%v", got, old, err)
	}
}
