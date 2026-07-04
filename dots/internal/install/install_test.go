package install

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestSSHPublicKeyCandidatesPrefersDefaultKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		t.Fatalf("creating ssh dir: %v", err)
	}
	for _, path := range []string{
		filepath.Join(sshDir, "work.pub"),
		filepath.Join(sshDir, "id_ed25519.pub"),
		filepath.Join(sshDir, "personal.pub"),
	} {
		if err := os.WriteFile(path, []byte("key"), 0o600); err != nil {
			t.Fatalf("writing %s: %v", path, err)
		}
	}

	got := sshPublicKeyCandidates()
	want := []string{
		filepath.Join(sshDir, "id_ed25519.pub"),
		filepath.Join(sshDir, "personal.pub"),
		filepath.Join(sshDir, "work.pub"),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("sshPublicKeyCandidates() = %#v, want %#v", got, want)
	}
}

func TestDisplayPathUsesTildeForHome(t *testing.T) {
	t.Setenv("HOME", "/tmp/example-home")
	got := displayPath("/tmp/example-home/.ssh/id_ed25519.pub")
	if got != "~/.ssh/id_ed25519.pub" {
		t.Fatalf("displayPath() = %q, want %q", got, "~/.ssh/id_ed25519.pub")
	}
}

func TestRunHelpSkipsInstallLockAndSummary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Run(context.Background(), "--help"); err != nil {
		t.Fatalf("Run(--help) returned error: %v", err)
	}

	logPath := filepath.Join(home, ".cache", "dotfiles", "install.log")
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading install log: %v", err)
	}
	logText := string(logBytes)
	if !strings.Contains(logText, "Usage: dots install") {
		t.Fatalf("install log did not include usage output:\n%s", logText)
	}
	if strings.Contains(logText, "The installer will set up this machine") {
		t.Fatalf("install log unexpectedly included install summary:\n%s", logText)
	}

	lockPath := filepath.Join(home, ".cache", "dotfiles_install.flock")
	if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("install help should not create %s, got err=%v", lockPath, err)
	}

	statusPath := filepath.Join(home, ".cache", "dotfiles_install.lock")
	if _, err := os.Stat(statusPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("install help should not create %s, got err=%v", statusPath, err)
	}
}

func TestRunHelpIgnoresActiveInstallLock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	lockFile, releaseStatus, alreadyRunning, err := acquireInstallLock(context.Background())
	if err != nil {
		t.Fatalf("acquireInstallLock() returned error: %v", err)
	}
	if alreadyRunning {
		t.Fatal("acquireInstallLock() reported already running for a fresh temp home")
	}
	defer releaseStatus()
	defer lockFile.Close()

	if err := Run(context.Background(), "--help"); err != nil {
		t.Fatalf("Run(--help) returned error: %v", err)
	}

	logPath := filepath.Join(home, ".cache", "dotfiles", "install.log")
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading install log: %v", err)
	}
	logText := string(logBytes)
	if !strings.Contains(logText, "Usage: dots install") {
		t.Fatalf("install log did not include usage output:\n%s", logText)
	}
	if strings.Contains(logText, "Another dotfiles install is already running in a different terminal.") {
		t.Fatalf("install help unexpectedly respected the active install lock:\n%s", logText)
	}
	if strings.Contains(logText, "The installer will set up this machine") {
		t.Fatalf("install log unexpectedly included install summary:\n%s", logText)
	}
}

func TestInstallScriptGuardsMissingTargetDirectoryBeforeFind(t *testing.T) {
	repoRoot := repoRootFromInstallTests(t)
	scriptPath := filepath.Join(repoRoot, "install.sh")
	scriptBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("reading install.sh: %v", err)
	}
	scriptText := string(scriptBytes)
	if !strings.Contains(scriptText, `if [ -d "$DOTDOTFILES" ]; then`) {
		t.Fatalf("install.sh should guard missing target directories before running find:\n%s", scriptText)
	}
	if !strings.Contains(scriptText, `find "$DOTDOTFILES"`) {
		t.Fatalf("install.sh no longer contains the target directory probe this test expects:\n%s", scriptText)
	}
}

func repoRootFromInstallTests(t *testing.T) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	return filepath.Clean(filepath.Join(workingDirectory, "..", "..", ".."))
}
