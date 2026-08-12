package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSyncUsesDryRunInLinkedWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	homeDirectory := t.TempDir()
	t.Setenv("HOME", homeDirectory)
	t.Setenv("DOTFILES_LOG", filepath.Join(homeDirectory, ".cache", "dotfiles", "dots.log"))
	canonicalRoot := newDotsTestRepo(t)
	worktreeRoot := filepath.Join(t.TempDir(), "linked")
	runDotsTestGit(t, canonicalRoot, "worktree", "add", "-b", "feature", worktreeRoot)
	t.Setenv("DOTDOTFILES", worktreeRoot)

	exitCode := 1
	output := captureDotsStdout(t, func() {
		exitCode = run([]string{"sync"})
	})
	if exitCode != 0 {
		t.Fatalf("run(sync) exit code = %d, want 0\n%s", exitCode, output)
	}
	if !strings.Contains(output, "dry-run: no changes applied") {
		t.Fatalf("output = %q, want dry-run output", output)
	}
	if !strings.Contains(output, "linked worktree") || !strings.Contains(output, "--allow-worktree") {
		t.Fatalf("output = %q, want linked-worktree guidance", output)
	}

	notificationPath := filepath.Join(homeDirectory, ".cache", "dotfiles", "notifications")
	if _, err := os.Stat(notificationPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("notification path error = %v, want not exist", err)
	}
}

func captureDotsStdout(t *testing.T, action func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}
	os.Stdout = writer
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	outputChannel := make(chan string, 1)
	go func() {
		output, _ := io.ReadAll(reader)
		outputChannel <- string(output)
	}()

	action()
	if err := writer.Close(); err != nil {
		t.Fatalf("closing stdout writer: %v", err)
	}
	output := <-outputChannel
	if err := reader.Close(); err != nil {
		t.Fatalf("closing stdout reader: %v", err)
	}
	return output
}

func newDotsTestRepo(t *testing.T) string {
	t.Helper()

	repoRoot := t.TempDir()
	runDotsTestGit(t, repoRoot, "init")
	hooksDirectory := filepath.Join(repoRoot, ".git", "hooks-disabled")
	if err := os.MkdirAll(hooksDirectory, 0o755); err != nil {
		t.Fatalf("creating hooks directory: %v", err)
	}
	runDotsTestGit(t, repoRoot, "config", "core.hooksPath", hooksDirectory)
	runDotsTestGit(t, repoRoot, "config", "user.name", "Test")
	runDotsTestGit(t, repoRoot, "config", "user.email", "test@example.invalid")
	runDotsTestGit(t, repoRoot, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("test\n"), 0o600); err != nil {
		t.Fatalf("writing README: %v", err)
	}
	runDotsTestGit(t, repoRoot, "add", "README.md")
	runDotsTestGit(t, repoRoot, "commit", "-m", "Initial commit")
	return repoRoot
}

func runDotsTestGit(t *testing.T, directory string, args ...string) {
	t.Helper()

	command := exec.Command("git", args...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, directory, err, output)
	}
}
