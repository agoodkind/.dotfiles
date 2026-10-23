package githook

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestClaudeSubmoduleUpdateChecksOutPinnedCommit(t *testing.T) {
	fixture := newSubmoduleFixture(t)
	enableAgentPolicy(t, fixture.root)
	output, err := runGitResult(t, fixture.superproject, "submodule", "update", "--init", submodulePath)
	if err != nil {
		t.Fatalf("git submodule update --init: %v\n%s", err, output)
	}
	assertHeadAtGitlink(t, fixture.superproject, submodulePath)
}

func TestClaudeSubmoduleUpdateChecksOutPinnedCommitAtNonASCIIPath(t *testing.T) {
	fixture := newSubmoduleFixtureAt(t, nonASCIISubmodulePath)
	enableAgentPolicy(t, fixture.root)
	output, err := runGitResult(t, fixture.superproject, "submodule", "update", "--init", nonASCIISubmodulePath)
	if err != nil {
		t.Fatalf("git submodule update --init: %v\n%s", err, output)
	}
	assertHeadAtGitlink(t, fixture.superproject, nonASCIISubmodulePath)
}

func TestClaudeRecursiveSubmoduleUpdateChecksOutNestedPinnedCommit(t *testing.T) {
	fixture := newSubmoduleFixture(t)
	enableAgentPolicy(t, fixture.root)
	output, err := runGitResult(t, fixture.superproject, "submodule", "update", "--init", "--recursive")
	if err != nil {
		t.Fatalf("git submodule update --init --recursive: %v\n%s", err, output)
	}
	assertHeadAtGitlink(t, fixture.superproject, submodulePath)
	assertHeadAtGitlink(t, filepath.Join(fixture.superproject, submodulePath), nestedSubmodulePath)
}

func TestClaudeDetachSubmoduleAwayFromGitlinkIsRefused(t *testing.T) {
	fixture := newSubmoduleFixture(t)
	enableAgentPolicy(t, fixture.root)
	if output, err := runGitResult(t, fixture.superproject, "submodule", "update", "--init", submodulePath); err != nil {
		t.Fatalf("git submodule update --init: %v\n%s", err, output)
	}
	submodule := filepath.Join(fixture.superproject, submodulePath)
	if output, err := runGitResult(t, submodule, "checkout", "main"); err != nil {
		t.Fatalf("git checkout main in submodule: %v\n%s", err, output)
	}
	output, err := runGitResult(t, submodule, "checkout", "--detach", fixture.submoduleBase)
	if err == nil {
		t.Fatalf("git checkout --detach %s in submodule succeeded, want refusal\n%s", fixture.submoduleBase, output)
	}
	if !strings.Contains(output, msgLeaveDefault) {
		t.Fatalf("detach refusal output missing %q:\n%s", msgLeaveDefault, output)
	}
	branch := gitOutputForTest(t, submodule, "symbolic-ref", "--short", "HEAD")
	if branch != "main" {
		t.Fatalf("submodule HEAD = %q after refused detach, want main", branch)
	}
}

func TestClaudeCommitOnUnprotectedDefaultBranchIsAllowed(t *testing.T) {
	fixture := newSubmoduleFixture(t)
	enableAgentPolicy(t, fixture.root)
	if output, err := runGitResult(t, fixture.superproject, "submodule", "update", "--init", submodulePath); err != nil {
		t.Fatalf("git submodule update --init: %v\n%s", err, output)
	}
	submodule := filepath.Join(fixture.superproject, submodulePath)
	if output, err := runGitResult(t, submodule, "checkout", "main"); err != nil {
		t.Fatalf("git checkout main in submodule: %v\n%s", err, output)
	}
	for _, repo := range []string{fixture.superproject, submodule} {
		before := gitOutputForTest(t, repo, "rev-parse", "HEAD")
		output, err := runGitResult(t, repo, "commit", "--allow-empty", "-m", "agent commit")
		if err != nil {
			t.Fatalf("git commit on unprotected main in %s failed: %v\n%s", repo, err, output)
		}
		after := gitOutputForTest(t, repo, "rev-parse", "HEAD")
		if after == before {
			t.Fatalf("HEAD in %s did not change after allowed commit", repo)
		}
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

const (
	submodulePath       = "third_party/gksyntax"
	nestedSubmodulePath = "grammars/swift"
	// nonASCIISubmodulePath is C-quoted by ls-files unless -z is passed.
	nonASCIISubmodulePath = "third_party/modé"
)

type submoduleFixture struct {
	root          string
	superproject  string
	submoduleBase string
}

// newSubmoduleFixture builds a clone of a superproject that pins a submodule,
// which in turn pins a nested submodule. Each submodule origin has a base
// commit, the pinned commit, and a newer tip on main, so the pin differs from
// the branch a fresh submodule clone lands on. Git config is isolated from the
// operator's global config, and no hooks run until enableAgentPolicy.
func newSubmoduleFixture(t *testing.T) submoduleFixture {
	t.Helper()
	return newSubmoduleFixtureAt(t, submodulePath)
}

// newSubmoduleFixtureAt builds the newSubmoduleFixture layout with the
// top-level submodule at path.
func newSubmoduleFixtureAt(t *testing.T, path string) submoduleFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolving temp root: %v", err)
	}
	globalConfig := filepath.Join(root, "gitconfig")
	t.Setenv("GIT_CONFIG_GLOBAL", globalConfig)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_EMAIL_RULES", filepath.Join(root, "missing-email-rules"))
	t.Setenv("DOTS_BINARY_DIR", filepath.Join(root, "dots-bin"))
	for _, setting := range [][2]string{
		{"user.name", "Test User"},
		{"user.email", "test@example.invalid"},
		{"commit.gpgsign", "false"},
		{"init.defaultBranch", "main"},
		{"protocol.file.allow", "always"},
		{"core.hooksPath", filepath.Join(root, "no-hooks")},
	} {
		mustGit(t, root, "config", "--file", globalConfig, setting[0], setting[1])
	}

	nestedOrigin := newOriginRepo(t, root, "grammar")
	nestedPin := commitToOrigin(t, nestedOrigin.work, "pin")
	commitToOrigin(t, nestedOrigin.work, "tip")

	submoduleOrigin := newOriginRepo(t, root, "gksyntax")
	mustGit(t, submoduleOrigin.work, "submodule", "add", nestedOrigin.bare, nestedSubmodulePath)
	mustGit(t, filepath.Join(submoduleOrigin.work, nestedSubmodulePath), "checkout", "--quiet", nestedPin)
	mustGit(t, submoduleOrigin.work, "add", nestedSubmodulePath)
	submodulePin := commitToOrigin(t, submoduleOrigin.work, "pin")
	commitToOrigin(t, submoduleOrigin.work, "tip")

	superOrigin := newOriginRepo(t, root, "super")
	mustGit(t, superOrigin.work, "submodule", "add", submoduleOrigin.bare, path)
	mustGit(t, filepath.Join(superOrigin.work, path), "checkout", "--quiet", submodulePin)
	mustGit(t, superOrigin.work, "add", path)
	commitToOrigin(t, superOrigin.work, "pin")

	superproject := filepath.Join(root, "clone")
	mustGit(t, root, "clone", "--quiet", superOrigin.bare, superproject)
	return submoduleFixture{
		root:          root,
		superproject:  superproject,
		submoduleBase: submoduleOrigin.base,
	}
}

type originRepo struct {
	bare string
	work string
	base string
}

// newOriginRepo creates a bare origin and a working clone whose main tracks
// it, seeded with one pushed base commit.
func newOriginRepo(t *testing.T, root string, name string) originRepo {
	t.Helper()
	bare := filepath.Join(root, name+".git")
	work := filepath.Join(root, name+"-work")
	mustGit(t, root, "init", "--quiet", "--bare", bare)
	mustGit(t, root, "init", "--quiet", work)
	mustGit(t, work, "remote", "add", "origin", bare)
	mustGit(t, work, "commit", "--quiet", "--allow-empty", "-m", "base")
	mustGit(t, work, "push", "--quiet", "--set-upstream", "origin", "main")
	return originRepo{bare: bare, work: work, base: gitOutputForTest(t, work, "rev-parse", "HEAD")}
}

func commitToOrigin(t *testing.T, work string, message string) string {
	t.Helper()
	mustGit(t, work, "commit", "--quiet", "--allow-empty", "-m", message)
	mustGit(t, work, "push", "--quiet", "origin", "main")
	return gitOutputForTest(t, work, "rev-parse", "HEAD")
}

// enableAgentPolicy turns on the global hooks and the shell gate for every
// repository under root, then marks the process as a Claude Code session.
func enableAgentPolicy(t *testing.T, root string) {
	t.Helper()
	globalConfig := os.Getenv("GIT_CONFIG_GLOBAL")
	mustGit(t, root, "config", "--file", globalConfig, "core.hooksPath", globalHooksDir(t))
	mustGit(t, root, "config", "--file", globalConfig, "goodkind.protectedBranchEnforcePath", root)
	t.Setenv("CLAUDECODE", "1")
}

func assertHeadAtGitlink(t *testing.T, superproject string, path string) {
	t.Helper()
	entry := gitOutputForTest(t, superproject, "ls-files", "--stage", "--", path)
	fields := strings.Fields(entry)
	if len(fields) != 4 || fields[0] != gitlinkMode {
		t.Fatalf("ls-files --stage %s in %s = %q, want one gitlink entry", path, superproject, entry)
	}
	head := gitOutputForTest(t, filepath.Join(superproject, path), "rev-parse", "HEAD")
	if head != fields[1] {
		t.Fatalf("%s HEAD = %s, want gitlink %s", path, head, fields[1])
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if output, err := runGitResult(t, dir, args...); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, output)
	}
}

func gitOutputForTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		t.Fatalf("git %v in %s: %v", args, dir, err)
	}
	return strings.TrimSpace(string(output))
}

func runGitResult(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	return string(output), err
}

func TestClaudePrimaryBranchGuardWithoutUpstream(t *testing.T) {
	for _, operation := range [][]string{{"switch", "-c", "feature"}, {"checkout", "--detach"}} {
		t.Run(strings.Join(operation, " "), func(t *testing.T) {
			repo := newSeededRepo(t, "main")
			mustGit(t, repo, "config", "goodkind.protectedBranchEnforcePath", repo)
			t.Setenv("CLAUDECODE", "1")
			enableGlobalHooks(t, repo)
			output, err := runGitResult(t, repo, operation...)
			if err == nil || !strings.Contains(output, msgLeaveDefault) {
				t.Fatalf("git %v error = %v, want default branch refusal:\n%s", operation, err, output)
			}
			if branch := gitOutputForTest(t, repo, "symbolic-ref", "--short", "HEAD"); branch != "main" {
				t.Fatalf("branch after refused operation = %q, want main", branch)
			}
		})
	}
}

func TestClaudeUnprotectedPrimaryMutations(t *testing.T) {
	for _, origin := range []string{"", "https://gitlab.com/example/project.git", "https://github.com/invalid"} {
		t.Run(origin, func(t *testing.T) {
			repo := newSeededRepo(t, "main")
			mustGit(t, repo, "config", "goodkind.protectedBranchEnforcePath", repo)
			if origin != "" {
				mustGit(t, repo, "remote", "add", "origin", origin)
			}
			t.Setenv("CLAUDECODE", "1")
			enableGlobalHooks(t, repo)
			output, err := runGitResult(t, repo, "commit", "--allow-empty", "-m", "allowed")
			if err != nil {
				t.Fatalf("unprotected commit failed: %v\n%s", err, output)
			}
			if origin == "https://github.com/invalid" && !strings.Contains(output, "lookup failed") {
				t.Fatalf("failed lookup did not report its error:\n%s", output)
			}
			if err := os.WriteFile(filepath.Join(repo, "change.txt"), []byte("stash\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			mustGit(t, repo, "add", "change.txt")
			mustGit(t, repo, "stash", "push", "-m", "allowed")
			mustGit(t, repo, "stash", "pop")
		})
	}
}

func TestProtectionHelperTargets(t *testing.T) {
	repo := newSeededRepo(t, "main")
	file := filepath.Join(repo, "file.txt")
	if err := os.WriteFile(file, []byte("test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(repo, "new", "file.txt")
	for _, args := range [][]string{{repo}, {file}, {missing}, {"--dir-only", repo}, {"--dir-only", file}, {"--dir-only", missing}} {
		command := exec.Command(filepath.Join(globalHooksDir(t), "github-pr-protection.sh"), args...)
		output, err := command.CombinedOutput()
		if err != nil || strings.TrimSpace(string(output)) != `{"pr_protected":false}` {
			t.Fatalf("helper %v = %s, %v; want false", args, output, err)
		}
	}
}

func TestProtectionHelperLiveGitHub(t *testing.T) {
	protectedRepo := os.Getenv("DOTFILES_TEST_PROTECTED_REPO")
	unprotectedRepo := os.Getenv("DOTFILES_TEST_UNPROTECTED_REPO")
	if protectedRepo == "" || unprotectedRepo == "" {
		t.Skip("set DOTFILES_TEST_PROTECTED_REPO and DOTFILES_TEST_UNPROTECTED_REPO to GitHub owner/repository values")
	}
	for _, scenario := range []struct {
		repository string
		protected  bool
	}{{protectedRepo, true}, {unprotectedRepo, false}} {
		t.Run(scenario.repository, func(t *testing.T) {
			repo := newSeededRepo(t, "main")
			mustGit(t, repo, "commit", "--allow-empty", "-m", "second")
			mustGit(t, repo, "remote", "add", "origin", "https://github.com/"+scenario.repository+".git")
			mustGit(t, repo, "config", "goodkind.protectedBranchEnforcePath", repo)
			t.Setenv("GH_REPO", "github.com/cli/cli")
			command := exec.Command(filepath.Join(globalHooksDir(t), "github-pr-protection.sh"), filepath.Join(repo, "new", "file.txt"))
			output, err := command.Output()
			if err != nil {
				t.Fatalf("live helper: %v", err)
			}
			var result struct {
				Protected bool `json:"pr_protected"`
			}
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatalf("decode helper: %v", err)
			}
			if result.Protected != scenario.protected {
				t.Fatalf("helper protection = %t, want %t", result.Protected, scenario.protected)
			}
			t.Setenv("CLAUDECODE", "1")
			enableGlobalHooks(t, repo)
			before := gitOutputForTest(t, repo, "rev-parse", "HEAD")
			commitOutput, err := runGitResult(t, repo, "commit", "--allow-empty", "-m", "protection test")
			if scenario.protected {
				if err == nil || !strings.Contains(commitOutput, "commit blocked on protected branch") {
					t.Fatalf("protected commit error = %v, output:\n%s", err, commitOutput)
				}
				if after := gitOutputForTest(t, repo, "rev-parse", "HEAD"); after != before {
					t.Fatalf("protected commit changed HEAD from %s to %s", before, after)
				}
			} else if err != nil {
				t.Fatalf("unprotected commit failed: %v\n%s", err, commitOutput)
			}
			for _, operation := range [][]string{{"rebase", "--force-rebase", "HEAD~1"}, {"stash", "push", "-m", "protection test"}} {
				if operation[0] == "stash" {
					if err := os.WriteFile(filepath.Join(repo, "change.txt"), []byte("stash\n"), 0o600); err != nil {
						t.Fatal(err)
					}
					mustGit(t, repo, "add", "change.txt")
				}
				output, err := runGitResult(t, repo, operation...)
				if scenario.protected {
					if err == nil || !strings.Contains(output, "agent harness cannot") {
						t.Fatalf("protected git %v error = %v, output:\n%s", operation, err, output)
					}
				} else if err != nil {
					t.Fatalf("unprotected git %v failed: %v\n%s", operation, err, output)
				}
			}
		})
	}
}
