package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativePreCommitInstallation(t *testing.T) {
	home, dotfiles := isolateNativeHookGit(t)
	if err := requireNativeHookGit(t.Context()); err != nil {
		t.Skip(err)
	}

	binaryDir := filepath.Join(home, "bin with spaces and an apostrophe '")
	binary := filepath.Join(binaryDir, "dots")
	t.Setenv("DOTS_BINARY_DIR", binaryDir)
	if err := os.MkdirAll(binaryDir, 0o755); err != nil {
		t.Fatal(err)
	}

	build := exec.CommandContext(
		t.Context(),
		"go",
		"build",
		"-modcacherw",
		"-buildvcs=false",
		"-o",
		binary,
		"./cmd/dots",
	)
	build.Dir = filepath.Join("..", "..", "..")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build dots: %v\n%s", err, output)
	}

	globalHooks := filepath.Join(home, "custom global hooks")
	runNativeHookGit(t, dotfiles, "config", "--global", "core.hooksPath", globalHooks)
	runNativeHookGit(t, dotfiles, "config", "--global", "hook.other.event", "post-commit")
	runNativeHookGit(t, dotfiles, "config", "--global", "hook.other.command", "/bin/true")
	repository := filepath.Join(home, "repository with local hooks")
	runNativeHookGit(t, home, "init", "--initial-branch=main", repository)
	runNativeHookGit(t, repository, "config", "user.name", "Install Test")
	runNativeHookGit(t, repository, "config", "user.email", "install@example.invalid")
	runNativeHookGit(t, repository, "config", "commit.gpgsign", "false")
	runNativeHookGit(t, repository, "config", "goodkind.allowedName", "Install Test")
	runNativeHookGit(t, repository, "config", "goodkind.allowedEmail", "install@example.invalid")
	runNativeHookGit(t, dotfiles, "config", "--global", "goodkind.protectedBranchEnforcePath", home)
	localHooks := filepath.Join(repository, "custom local hooks")
	if err := os.Mkdir(localHooks, 0o755); err != nil {
		t.Fatal(err)
	}

	localHook := filepath.Join(localHooks, "pre-commit")
	if err := os.WriteFile(
		localHook,
		[]byte("#!/bin/sh\nprintf '%s\\n' ran >> hook-log\n"),
		0o755,
	); err != nil {
		t.Fatal(err)
	}

	defaultHooks := filepath.Join(repository, ".git", "hooks")
	if err := os.MkdirAll(defaultHooks, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(localHook, filepath.Join(defaultHooks, "pre-commit")); err != nil {
		t.Fatal(err)
	}

	runNativeHookGit(t, repository, "config", "--local", "core.hooksPath", localHooks)

	for range 2 {
		if err := SyncGlobalGitHooks(t.Context(), dotfiles, nil); err != nil {
			t.Fatal(err)
		}
	}

	if got := runNativeHookGit(
		t,
		dotfiles,
		"config",
		"--global",
		"--get-all",
		"hook.dotfiles-pre-commit.event",
	); got != "pre-commit\n" {
		t.Fatalf("configured pre-commit events: %q", got)
	}

	if got := runNativeHookGit(
		t,
		dotfiles,
		"config",
		"--global",
		"--get-all",
		"core.hooksPath",
	); got != globalHooks+"\n" {
		t.Fatalf("global core.hooksPath: %q", got)
	}

	if got := runNativeHookGit(
		t,
		repository,
		"config",
		"--local",
		"--get-all",
		"core.hooksPath",
	); got != localHooks+"\n" {
		t.Fatalf("local core.hooksPath: %q", got)
	}

	if got := runNativeHookGit(
		t,
		dotfiles,
		"config",
		"--global",
		"--get-all",
		"hook.other.event",
	); got != "post-commit\n" {
		t.Fatalf("unrelated hook event: %q", got)
	}

	if got := runNativeHookGit(
		t,
		repository,
		"hook",
		"list",
		"-z",
		"pre-commit",
	); got != "dotfiles-pre-commit\x00hook from hookdir\x00" {
		t.Fatalf("effective hooks: %q", got)
	}

	for i := range 2 {
		if i != 0 {
			previous := binary + ".old"
			if err := os.Rename(binary, previous); err != nil {
				t.Fatal(err)
			}

			if output, err := exec.CommandContext(
				t.Context(),
				"cp",
				previous,
				binary,
			).CombinedOutput(); err != nil {
				t.Fatalf("replace dots: %v\n%s", err, output)
			}
		}

		runNativeHookGit(t, repository, "commit", "--allow-empty", "-m", "Check installed hooks")
	}

	log, err := os.ReadFile(filepath.Join(repository, "hook-log"))
	if err != nil || string(log) != "ran\nran\n" {
		t.Fatalf("local hook ran %q: %v", log, err)
	}

	runNativeHookGit(
		t,
		repository,
		"config",
		"--local",
		"hook.dotfiles-pre-commit.enabled",
		"false",
	)
	if err := SyncGlobalGitHooks(t.Context(), dotfiles, nil); err == nil ||
		!strings.Contains(err.Error(), repository) {
		t.Fatalf("expected sync to report the repository that disables the hook, got %v", err)
	}

	if got := runNativeHookGit(
		t,
		repository,
		"config",
		"--local",
		"--get",
		"hook.dotfiles-pre-commit.enabled",
	); got != "false\n" {
		t.Fatalf("local hook enablement: %q", got)
	}

	runNativeHookGit(
		t,
		repository,
		"config",
		"--local",
		"--unset",
		"hook.dotfiles-pre-commit.enabled",
	)

	if err := os.Remove(binary); err != nil {
		t.Fatal(err)
	}

	head := runNativeHookGit(t, repository, "rev-parse", "HEAD")
	commit := exec.CommandContext(
		t.Context(),
		"git",
		"-C",
		repository,
		"commit",
		"--allow-empty",
		"-m",
		"Fail without dots",
	)
	if output, err := commit.CombinedOutput(); err == nil ||
		!strings.Contains(string(output), binary) {
		t.Fatalf("expected git commit to report the missing dots binary: %v\n%s", err, output)
	}

	if got := runNativeHookGit(t, repository, "rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD after failed commit: %q, want %q", got, head)
	}

	for range 2 {
		if err := RemoveNativePreCommit(t.Context()); err != nil {
			t.Fatal(err)
		}
	}

	if got := runNativeHookGit(
		t,
		repository,
		"hook",
		"list",
		"-z",
		"pre-commit",
	); got != "hook from hookdir\x00" {
		t.Fatalf("effective hooks after uninstall: %q", got)
	}

	if got := runNativeHookGit(
		t,
		dotfiles,
		"config",
		"--global",
		"--get",
		"hook.other.command",
	); got != "/bin/true\n" {
		t.Fatalf("unrelated hook command after uninstall: %q", got)
	}
}

func TestNativePreCommitRejectsMissingBinaryBeforeRegistration(t *testing.T) {
	home, dotfiles := isolateNativeHookGit(t)
	if err := requireNativeHookGit(t.Context()); err != nil {
		t.Skip(err)
	}

	missing := filepath.Join(home, "missing dots")
	t.Setenv("DOTS_BINARY", missing)
	if err := SyncGlobalGitHooks(t.Context(), dotfiles, nil); err == nil ||
		!strings.Contains(err.Error(), missing) {
		t.Fatalf("expected sync to report the missing dots binary, got %v", err)
	}

	assertNativeHookConfigAbsent(t, dotfiles)
}

func TestNativePreCommitRejectsUnsupportedGitBeforeRegistration(t *testing.T) {
	_, dotfiles := isolateNativeHookGit(t)
	if err := requireNativeHookGit(t.Context()); err == nil {
		t.Skip("Git supports configured hooks")
	}

	if err := SyncGlobalGitHooks(t.Context(), dotfiles, nil); err == nil ||
		!strings.Contains(err.Error(), "Git 2.54 or newer") {
		t.Fatalf("expected the Git version requirement, got %v", err)
	}

	assertNativeHookConfigAbsent(t, dotfiles)
}

func isolateNativeHookGit(t *testing.T) (string, string) {
	t.Helper()
	goBinary, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}

	gitBinary, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv(
		"PATH",
		strings.Join(
			[]string{filepath.Dir(gitBinary), filepath.Dir(goBinary), "/usr/bin", "/bin"},
			string(os.PathListSeparator),
		),
	)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_COUNT", "0")

	for _, name := range []string{
		"GIT_CONFIG",
		"GIT_CONFIG_PARAMETERS",
		"GIT_DIR",
		"GIT_COMMON_DIR",
		"GIT_WORK_TREE",
		"GIT_INDEX_FILE",
		"CLAUDECODE",
		"CODEX_CI",
		"CURSOR_AGENT",
		"GEMINI_CLI",
		"DOTS_BINARY",
		"DOTS_BINARY_DIR",
		"XDG_CACHE_HOME",
	} {
		if previous, ok := os.LookupEnv(name); ok {
			t.Setenv(name, previous)
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
		}
	}

	dotfiles := filepath.Join(home, "dotfiles with spaces")
	runNativeHookGit(t, home, "init", dotfiles)
	if err := os.Mkdir(filepath.Join(dotfiles, "git-global-hooks"), 0o755); err != nil {
		t.Fatal(err)
	}

	return home, dotfiles
}

func runNativeHookGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}

	return string(output)
}

func assertNativeHookConfigAbsent(t *testing.T, dotfiles string) {
	t.Helper()
	if _, err := os.Stat(os.Getenv("GIT_CONFIG_GLOBAL")); !os.IsNotExist(err) {
		t.Fatalf("setup should not create a global config for %s: %v", dotfiles, err)
	}
}
