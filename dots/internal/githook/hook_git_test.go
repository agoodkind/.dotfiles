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
	"slices"
	"strings"
	"testing"
)

func TestHumanCheckoutFeatureOnPrimaryIsAllowed(t *testing.T) {
	repo := newSeededRepo(t)
	enableGlobalHooks(t, repo)
	mustGit(t, repo, "switch", "-c", "feature")
	assertGitOutput(t, repo, "feature", "branch", "--show-current")
}

func TestCursorCheckoutOnPrimaryUsesRemoteDefault(t *testing.T) {
	repo := newSeededRepo(t)
	bare := t.TempDir()
	mustGit(t, bare, "init", "--bare", "--initial-branch=trunk")
	mustGit(t, repo, "branch", "trunk")
	mustGit(t, repo, "remote", "add", "origin", bare)
	mustGit(t, repo, "push", "origin", "trunk")
	mustGit(t, repo, "remote", "set-head", "origin", "trunk")
	mustGit(t, repo, "switch", "-c", "feature")

	enableGlobalHooks(t, repo)
	t.Setenv("CURSOR_AGENT", "1")
	assertGitRefusal(t, repo, msgLeaveDefault, "switch", "main")
	assertGitOutput(t, repo, "feature", "branch", "--show-current")
	mustGit(t, repo, "switch", "trunk")
	assertGitRefusal(t, repo, msgLeaveDefault, "switch", "feature")
	assertGitOutput(t, repo, "trunk", "branch", "--show-current")
}

func TestClaudeCommitFetchAndResetToOriginMainAreAllowed(t *testing.T) {
	root := newGitFixture(t)
	origin := newOriginRepo(t, root, "upstream")
	repo := filepath.Join(root, "clone")
	mustGit(t, root, "clone", origin.bare, repo)
	remoteHead := commitToOrigin(t, origin.work, "remote change")

	enableAgentPolicy(t, root)
	mustGit(t, repo, "commit", "--allow-empty", "-m", "local change")
	assertGitOutput(t, repo, "local change", "log", "-1", "--format=%s")
	mustGit(t, repo, "fetch", "origin")
	mustGit(t, repo, "reset", "--hard", "origin/main")
	assertGitOutput(t, repo, remoteHead, "rev-parse", "HEAD")
	assertGitOutput(t, repo, "main", "branch", "--show-current")
}

func TestCursorCanAddWorktreeButCannotSwitchItToDefault(t *testing.T) {
	repo := newSeededRepo(t)
	mustGit(t, repo, "switch", "-c", "existing-feature")
	before := gitOutputForTest(t, repo, "rev-parse", "HEAD")
	linked := filepath.Join(filepath.Dir(repo), "linked")
	enableGlobalHooks(t, repo)
	t.Setenv("CURSOR_AGENT", "1")

	mustGit(t, repo, "worktree", "add", "-b", "linked-feature", linked)
	assertGitOutput(t, repo, "existing-feature", "branch", "--show-current")
	assertGitOutput(t, linked, "linked-feature", "branch", "--show-current")
	assertGitOutput(t, linked, before, "rev-parse", "HEAD")
	assertGitRefusal(t, linked, msgLinkedDefault, "switch", "main")
	assertGitOutput(t, linked, "linked-feature", "branch", "--show-current")
}

func TestClaudeSubmoduleUpdateChecksOutPinnedCommit(t *testing.T) {
	fixture := newSubmoduleFixture(t)
	enableAgentPolicy(t, fixture.root)
	output, err := runGitResult(
		t,
		fixture.superproject,
		"submodule",
		"update",
		"--init",
		submodulePath,
	)
	if err != nil {
		t.Fatalf("git submodule update --init: %v\n%s", err, output)
	}

	assertHeadAtGitlink(t, fixture.superproject, submodulePath)
}

func TestClaudeSubmoduleUpdateChecksOutPinnedCommitAtNonASCIIPath(t *testing.T) {
	fixture := newSubmoduleFixtureAt(t, nonASCIISubmodulePath)
	enableAgentPolicy(t, fixture.root)
	output, err := runGitResult(
		t,
		fixture.superproject,
		"submodule",
		"update",
		"--init",
		nonASCIISubmodulePath,
	)
	if err != nil {
		t.Fatalf("git submodule update --init: %v\n%s", err, output)
	}

	assertHeadAtGitlink(t, fixture.superproject, nonASCIISubmodulePath)
}

func TestClaudeRecursiveSubmoduleUpdateChecksOutNestedPinnedCommit(t *testing.T) {
	fixture := newSubmoduleFixture(t)
	enableAgentPolicy(t, fixture.root)
	output, err := runGitResult(
		t,
		fixture.superproject,
		"submodule",
		"update",
		"--init",
		"--recursive",
	)
	if err != nil {
		t.Fatalf("git submodule update --init --recursive: %v\n%s", err, output)
	}

	assertHeadAtGitlink(t, fixture.superproject, submodulePath)
	assertHeadAtGitlink(t, filepath.Join(fixture.superproject, submodulePath), nestedSubmodulePath)
}

func TestClaudeDetachSubmoduleAwayFromGitlinkIsRefused(t *testing.T) {
	fixture := newSubmoduleFixture(t)
	enableAgentPolicy(t, fixture.root)
	if output, err := runGitResult(
		t,
		fixture.superproject,
		"submodule",
		"update",
		"--init",
		submodulePath,
	); err != nil {
		t.Fatalf("git submodule update --init: %v\n%s", err, output)
	}

	submodule := filepath.Join(fixture.superproject, submodulePath)
	if output, err := runGitResult(t, submodule, "checkout", "main"); err != nil {
		t.Fatalf("git checkout main in submodule: %v\n%s", err, output)
	}

	output, err := runGitResult(t, submodule, "checkout", "--detach", fixture.submoduleBase)
	if err == nil {
		t.Fatalf(
			"git checkout --detach %s in submodule succeeded, want refusal\n%s",
			fixture.submoduleBase,
			output,
		)
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
	if output, err := runGitResult(
		t,
		fixture.superproject,
		"submodule",
		"update",
		"--init",
		submodulePath,
	); err != nil {
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

func newGitFixture(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve Git fixture: %v", err)
	}

	home := filepath.Join(root, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}

	if config := os.Getenv("GH_CONFIG_DIR"); config == "" {
		if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
			t.Setenv("GH_CONFIG_DIR", filepath.Join(configHome, "gh"))
		} else {
			t.Setenv("GH_CONFIG_DIR", filepath.Join(os.Getenv("HOME"), ".config", "gh"))
		}
	}

	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(root, "gitconfig"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_COUNT", "0")
	t.Setenv("GIT_EMAIL_RULES", filepath.Join(root, "missing-email-rules"))
	t.Setenv("DOTFILES_LOG", filepath.Join(root, "dots.log"))
	t.Setenv("DOTDOTFILES", filepath.Join(home, ".dotfiles"))

	for _, setting := range [][2]string{
		{"user.name", "Test User"},
		{"user.email", "test@example.invalid"},
		{"commit.gpgsign", "false"},
		{"init.defaultBranch", "main"},
		{"protocol.file.allow", "always"},
		{"core.hooksPath", filepath.Join(root, "no-hooks")},
	} {
		mustGit(t, root, "config", "--global", setting[0], setting[1])
	}

	return root
}

func newSeededRepo(t *testing.T) string {
	t.Helper()
	root := newGitFixture(t)
	repo := filepath.Join(root, "repo")
	mustGit(t, root, "init", "--quiet", repo)
	mustGit(t, repo, "commit", "--allow-empty", "-m", "seed")

	return repo
}

func enableGlobalHooks(t *testing.T, repo string) {
	t.Helper()
	root := filepath.Dir(os.Getenv("GIT_CONFIG_GLOBAL"))
	binary := dotsBinaryForTest(t)
	mustGit(t, repo, "config", "--global", "core.hooksPath", referenceHooksForTest(t))
	mustGit(t, repo, "config", "--global", "goodkind.protectedBranchEnforcePath", root)
	t.Setenv("DOTS_BINARY", binary)
	t.Setenv("DOTS_BINARY_DIR", filepath.Dir(binary))
	t.Setenv("DOTS_BUILD_HASH_FILE", filepath.Join(filepath.Dir(binary), ".dots.build.hash"))
	t.Setenv("DOTS_BUILD_LOCK_FILE", filepath.Join(filepath.Dir(binary), ".dots.build.lock"))
	t.Setenv("GO_LOCAL_ROOT", filepath.Join(root, "go"))

	for key, value := range dotsGoEnv {
		t.Setenv(key, value)
	}
}

func enableNativePreCommit(t *testing.T, repo string, dependencies ...string) {
	t.Helper()
	binary := dotsBinaryForTest(t)
	bin := t.TempDir()

	for _, executable := range append([]string{"git"}, dependencies...) {
		path, err := exec.LookPath(executable)
		if err != nil {
			t.Fatal(err)
		}

		if err := os.Symlink(path, filepath.Join(bin, executable)); err != nil {
			t.Fatal(err)
		}
	}

	quote := func(value string) string {
		return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
	}
	command := "PATH=" + quote(bin) + " " + quote(binary) + " git-hook pre-commit"
	mustGit(t, repo, "config", "--global", "hook.dotfiles-pre-commit.event", "pre-commit")
	mustGit(t, repo, "config", "--global", "hook.dotfiles-pre-commit.command", command)
	output, err := runGitResult(t, repo, "hook", "list", "-z", "pre-commit")
	if err != nil && strings.Contains(output, "usage: git hook") {
		t.Skip("Git 2.54 or newer is required for configured hooks")
	}

	if err != nil || !slices.Contains(strings.Split(output, "\x00"), "dotfiles-pre-commit") {
		t.Fatalf("Git did not select the configured pre-commit hook: %v\n%s", err, output)
	}
}

func sourceRootForTest() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func globalHooksDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(sourceRootForTest(), "git-global-hooks")
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

// newSubmoduleFixture creates two levels of submodules.
// Each origin has a base commit, a pinned commit, and a newer tip on main.
func newSubmoduleFixture(t *testing.T) submoduleFixture {
	t.Helper()
	return newSubmoduleFixtureAt(t, submodulePath)
}

// newSubmoduleFixtureAt uses path for the top-level submodule.
func newSubmoduleFixtureAt(t *testing.T, path string) submoduleFixture {
	t.Helper()
	root := newGitFixture(t)

	nestedOrigin := newOriginRepo(t, root, "grammar")
	nestedPin := commitToOrigin(t, nestedOrigin.work, "pin")
	commitToOrigin(t, nestedOrigin.work, "tip")

	submoduleOrigin := newOriginRepo(t, root, "gksyntax")
	mustGit(t, submoduleOrigin.work, "submodule", "add", nestedOrigin.bare, nestedSubmodulePath)
	mustGit(
		t,
		filepath.Join(submoduleOrigin.work, nestedSubmodulePath),
		"checkout",
		"--quiet",
		nestedPin,
	)
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

// newOriginRepo pushes a base commit and sets main to track origin/main.
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

func enableAgentPolicy(t *testing.T, root string) {
	t.Helper()
	enableGlobalHooks(t, root)
	enableNativePreCommit(t, root)
	t.Setenv("CLAUDECODE", "1")
}

func referenceHooksForTest(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	directory := filepath.Join(root, "git-global-hooks")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{
		"reference-transaction",
		"pre-rebase",
		"chain-hook.sh",
		"protected-branch-policy.sh",
		"invoke-dots-git-hook.sh",
	} {
		if err := os.Symlink(
			filepath.Join(globalHooksDir(t), name),
			filepath.Join(directory, name),
		); err != nil {
			t.Fatal(err)
		}
	}

	for _, name := range []string{"config", "dots"} {
		if err := os.Symlink(
			filepath.Join(sourceRootForTest(), name),
			filepath.Join(root, name),
		); err != nil {
			t.Fatal(err)
		}
	}

	return directory
}

func assertHeadAtGitlink(t *testing.T, superproject string, path string) {
	t.Helper()
	entry := gitOutputForTest(t, superproject, "ls-files", "--stage", "--", path)
	fields := strings.Fields(entry)
	if len(fields) != 4 || fields[0] != gitlinkMode {
		t.Fatalf(
			"ls-files --stage %s in %s = %q, want one gitlink entry",
			path,
			superproject,
			entry,
		)
	}

	head := gitOutputForTest(t, filepath.Join(superproject, path), "rev-parse", "HEAD")
	if head != fields[1] {
		t.Fatalf("%s HEAD = %s, want gitlink %s", path, head, fields[1])
	}
}

func assertGitOutput(t *testing.T, dir, want string, args ...string) {
	t.Helper()
	if got := gitOutputForTest(t, dir, args...); got != want {
		t.Fatalf("git %v = %q, want %q", args, got, want)
	}
}

func assertGitRefusal(t *testing.T, dir, message string, args ...string) {
	t.Helper()
	output, err := runGitResult(t, dir, args...)
	if err == nil || !strings.Contains(output, message) {
		t.Fatalf("git %v error = %v, want refusal %q:\n%s", args, err, message, output)
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
		t.Run(
			strings.Join(operation, " "),
			func(t *testing.T) {
				repo := newSeededRepo(t)
				mustGit(t, repo, "config", "goodkind.protectedBranchEnforcePath", repo)
				t.Setenv("CLAUDECODE", "1")
				enableGlobalHooks(t, repo)
				output, err := runGitResult(t, repo, operation...)
				if err == nil || !strings.Contains(output, msgLeaveDefault) {
					t.Fatalf(
						"git %v error = %v, want default branch refusal:\n%s",
						operation,
						err,
						output,
					)
				}

				if branch := gitOutputForTest(
					t,
					repo,
					"symbolic-ref",
					"--short",
					"HEAD",
				); branch != "main" {
					t.Fatalf("branch after refused operation = %q, want main", branch)
				}
			},
		)
	}
}

func TestClaudeUnprotectedPrimaryMutations(t *testing.T) {
	for _, origin := range []string{
		"",
		"https://gitlab.com/example/project.git",
		"https://github.com/invalid",
	} {
		t.Run(
			origin,
			func(t *testing.T) {
				repo := newSeededRepo(t)
				mustGit(t, repo, "config", "goodkind.protectedBranchEnforcePath", repo)
				if origin != "" {
					mustGit(t, repo, "remote", "add", "origin", origin)
				}

				t.Setenv("CLAUDECODE", "1")
				enableGlobalHooks(t, repo)
				enableNativePreCommit(t, repo)
				output, err := runGitResult(t, repo, "commit", "--allow-empty", "-m", "allowed")
				if err != nil {
					t.Fatalf("unprotected commit failed: %v\n%s", err, output)
				}

				if origin == "https://github.com/invalid" &&
					!strings.Contains(output, "lookup failed") {
					t.Fatalf("failed lookup did not report its error:\n%s", output)
				}

				if err := os.WriteFile(
					filepath.Join(repo, "change.txt"),
					[]byte("stash\n"),
					0o600,
				); err != nil {
					t.Fatal(err)
				}

				mustGit(t, repo, "add", "change.txt")
				mustGit(t, repo, "stash", "push", "-m", "allowed")
				mustGit(t, repo, "stash", "pop")
			},
		)
	}
}

func TestNativePreCommitIdentityPrecedence(t *testing.T) {
	repo := newSeededRepo(t)
	enableNativePreCommit(t, repo)
	rules := filepath.Join(filepath.Dir(repo), "email-rules")
	t.Setenv("GIT_EMAIL_RULES", rules)
	writeTestFile(
		t,
		rules,
		"* fallback@example.invalid Fallback User\n"+
			repo+" base@example.invalid Base User\n",
	)
	writeTestFile(t, rules+".local", repo+" test@example.invalid Test User\n")
	commit := []string{"commit", "--allow-empty", "-m", "identity test"}
	mustGit(t, repo, commit...)

	before := gitOutputForTest(t, repo, "rev-parse", "HEAD")
	mustGit(t, repo, "config", "goodkind.allowedEmail", "configured@example.invalid")
	mustGit(t, repo, commit...)
	assertGitOutput(t, repo, before, "rev-parse", "HEAD^")

	before = gitOutputForTest(t, repo, "rev-parse", "HEAD")
	mustGit(t, repo, "config", "goodkind.allowedName", "Configured User")
	assertGitRefusal(
		t,
		repo,
		"Allowed Git identity: \"Configured User\" <configured@example.invalid>",
		commit...,
	)
	assertGitOutput(t, repo, before, "rev-parse", "HEAD")

	mustGit(t, repo, "config", "user.email", "configured@example.invalid")
	mustGit(t, repo, "config", "user.name", "Configured User")
	mustGit(t, repo, commit...)
	assertGitOutput(
		t,
		repo,
		"Configured User <configured@example.invalid>",
		"log",
		"-1",
		"--format=%an <%ae>",
	)
}

func TestNativePreCommitChainsHooksOnce(t *testing.T) {
	repo := newSeededRepo(t)
	enableNativePreCommit(t, repo)
	linked := filepath.Join(filepath.Dir(repo), "linked")
	mustGit(t, repo, "worktree", "add", "-b", "feature", linked)
	commonDir := gitOutputForTest(
		t,
		linked,
		"rev-parse",
		"--path-format=absolute",
		"--git-common-dir",
	)
	gitDir := gitOutputForTest(t, linked, "rev-parse", "--absolute-git-dir")
	homeHook := filepath.Join(os.Getenv("HOME"), ".git-hooks", "pre-commit")
	commonHook := filepath.Join(commonDir, "hooks", "pre-commit")
	worktreeHook := filepath.Join(gitDir, "hooks", "pre-commit")
	pwd, err := exec.LookPath("pwd")
	if err != nil {
		t.Fatal(err)
	}

	for _, hook := range []string{homeHook, commonHook, worktreeHook} {
		if err := os.MkdirAll(filepath.Dir(hook), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := os.Symlink(pwd, hook); err != nil {
			t.Fatal(err)
		}
	}

	commit := []string{"commit", "--allow-empty", "-m", "chaining test"}
	assertOnce := func() {
		t.Helper()
		output, err := runGitResult(t, linked, commit...)
		if err != nil || strings.Count(output, linked+"\n") != 1 {
			t.Fatalf("git commit error = %v, want one hook invocation:\n%s", err, output)
		}
	}
	assertOnce()
	mustGit(t, linked, "config", "core.hooksPath", filepath.Dir(commonHook))
	assertOnce()
}

func TestNativePreCommitGitleaksScansStagedContent(t *testing.T) {
	if _, err := exec.LookPath("gitleaks"); errors.Is(err, exec.ErrNotFound) {
		t.Skip("Gitleaks is not installed")
	} else if err != nil {
		t.Fatal(err)
	}

	repo := newSeededRepo(t)
	enableNativePreCommit(t, repo, "gitleaks")
	writeTestFile(
		t,
		filepath.Join(repo, ".gitleaks.toml"),
		`[[rules]]
id = 'dotfiles-synthetic-test'
description = 'The test rule matches a synthetic marker.'
regex = 'fixture-mark-[0-9]{6}'
`,
	)
	const syntheticMarker = "fixture-mark-804291"
	fixture := filepath.Join(repo, "fixture.txt")
	writeTestFile(t, fixture, syntheticMarker+"\n")
	mustGit(t, repo, "add", "fixture.txt")
	writeTestFile(t, fixture, "clean content\n")
	before := gitOutputForTest(t, repo, "rev-parse", "HEAD")
	commit := []string{"commit", "-m", "scanner test"}
	output, err := runGitResult(t, repo, commit...)
	if strings.Contains(output, syntheticMarker) {
		t.Fatal("Gitleaks printed the synthetic match without redaction")
	}

	if err == nil || !strings.Contains(output, "leaks found: 1") {
		t.Fatalf("git commit error = %v, want one Gitleaks finding:\n%s", err, output)
	}

	assertGitOutput(t, repo, before, "rev-parse", "HEAD")

	mustGit(t, repo, "add", "fixture.txt")
	writeTestFile(t, fixture, syntheticMarker+"\n")
	mustGit(t, repo, commit...)
	assertGitOutput(t, repo, "clean content", "show", "HEAD:fixture.txt")
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestProtectionHelperTargets(t *testing.T) {
	repo := newSeededRepo(t)
	file := filepath.Join(repo, "file.txt")
	if err := os.WriteFile(file, []byte("test\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	missing := filepath.Join(repo, "new", "file.txt")
	for _, args := range [][]string{
		{repo},
		{file},
		{missing},
		{"--dir-only", repo},
		{"--dir-only", file},
		{"--dir-only", missing},
	} {
		command := exec.Command(
			filepath.Join(globalHooksDir(t), "github-pr-protection.sh"),
			args...,
		)
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
		t.Skip(
			"set DOTFILES_TEST_PROTECTED_REPO and DOTFILES_TEST_UNPROTECTED_REPO to GitHub owner/repository values",
		)
	}

	for _, scenario := range []struct {
		repository string
		protected  bool
	}{
		{protectedRepo, true},
		{unprotectedRepo, false},
	} {
		t.Run(
			scenario.repository,
			func(t *testing.T) {
				repo := newSeededRepo(t)
				mustGit(t, repo, "commit", "--allow-empty", "-m", "second")
				mustGit(
					t,
					repo,
					"remote",
					"add",
					"origin",
					"https://github.com/"+scenario.repository+".git",
				)
				mustGit(t, repo, "config", "goodkind.protectedBranchEnforcePath", repo)
				t.Setenv("GH_REPO", "github.com/cli/cli")
				command := exec.Command(
					filepath.Join(globalHooksDir(t), "github-pr-protection.sh"),
					filepath.Join(repo, "new", "file.txt"),
				)
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
					t.Fatalf(
						"helper protection = %t, want %t",
						result.Protected,
						scenario.protected,
					)
				}

				enableGlobalHooks(t, repo)
				enableNativePreCommit(t, repo, "gh")

				for _, provider := range []struct {
					name       string
					marker     string
					restricted bool
				}{
					{name: "human"},
					{name: "Cursor", marker: "CURSOR_AGENT"},
					{name: "Claude", marker: "CLAUDECODE", restricted: true},
					{name: "Codex", marker: "CODEX_CI", restricted: true},
					{name: "Gemini", marker: "GEMINI_CLI", restricted: true},
				} {
					t.Run(
						provider.name,
						func(t *testing.T) {
							if provider.marker != "" {
								t.Setenv(provider.marker, "1")
							}

							before := gitOutputForTest(t, repo, "rev-parse", "HEAD")
							commit := []string{
								"commit",
								"--allow-empty",
								"-m",
								"protection test for " + provider.name,
							}
							if scenario.protected && provider.restricted {
								assertGitRefusal(
									t,
									repo,
									"commit blocked on protected branch",
									commit...,
								)
								assertGitOutput(t, repo, before, "rev-parse", "HEAD")

								return
							}

							output, err := runGitResult(t, repo, commit...)
							if err != nil || strings.Contains(output, "lookup failed") {
								t.Fatalf("git commit error = %v, output:\n%s", err, output)
							}

							assertGitOutput(t, repo, before, "rev-parse", "HEAD^")
						},
					)
				}

				t.Setenv("CLAUDECODE", "1")

				rebase := []string{"rebase", "--force-rebase", "HEAD~1"}
				if scenario.protected {
					assertGitRefusal(t, repo, msgPrimaryWrite, rebase...)
				} else {
					assertGitRefusal(t, repo, msgLeaveDefault, rebase...)
					mustGit(t, repo, "rebase", "--abort")
				}

				assertGitOutput(t, repo, "main", "branch", "--show-current")

				writeTestFile(t, filepath.Join(repo, "change.txt"), "stash\n")
				mustGit(t, repo, "add", "change.txt")
				stash := []string{"stash", "push", "-m", "protection test"}
				if scenario.protected {
					assertGitRefusal(t, repo, msgPrimaryStash, stash...)
				} else {
					mustGit(t, repo, stash...)
				}
			},
		)
	}
}
