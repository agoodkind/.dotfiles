package workspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinkDotfilesReplacesProfileSymlinkWithLocalFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg-data"))

	dotfiles := t.TempDir()
	repoProfile := filepath.Join(dotfiles, "home", ".profile")
	repoZshrc := filepath.Join(dotfiles, "home", ".zshrc")
	if err := os.MkdirAll(filepath.Join(dotfiles, "home"), 0o755); err != nil {
		t.Fatalf("creating home dir: %v", err)
	}
	if err := os.WriteFile(repoProfile, []byte("repo-profile\n"), 0o644); err != nil {
		t.Fatalf("writing repo profile: %v", err)
	}
	if err := os.WriteFile(repoZshrc, []byte("repo-zshrc\n"), 0o644); err != nil {
		t.Fatalf("writing repo zshrc: %v", err)
	}

	homeProfile := filepath.Join(home, ".profile")
	if err := os.Symlink(repoProfile, homeProfile); err != nil {
		t.Fatalf("creating profile symlink: %v", err)
	}

	logger, _ := newBackupTestLogger(t)
	if err := LinkDotfiles(context.Background(), dotfiles, logger); err != nil {
		t.Fatalf("LinkDotfiles: %v", err)
	}

	info, err := os.Lstat(homeProfile)
	if err != nil {
		t.Fatalf("stat HOME/.profile: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("HOME/.profile is still a symlink")
	}
	body, err := os.ReadFile(homeProfile)
	if err != nil {
		t.Fatalf("reading HOME/.profile: %v", err)
	}
	if !strings.Contains(string(body), repoProfile) {
		t.Fatalf("local profile does not source %s: %q", repoProfile, body)
	}

	zshrc := filepath.Join(home, ".zshrc")
	target, err := os.Readlink(zshrc)
	if err != nil {
		t.Fatalf("HOME/.zshrc was not a symlink: %v", err)
	}
	if target != repoZshrc {
		t.Fatalf("HOME/.zshrc -> %q, want %q", target, repoZshrc)
	}

	dockerBlock := "# The following lines were added by Docker Desktop to add commands to your PATH.\nexport PATH=\"$PATH:/tmp/docker-bin\"\n# End of Docker Desktop section.\n\n"
	if err := os.WriteFile(homeProfile, append([]byte(dockerBlock), body...), 0o644); err != nil {
		t.Fatalf("writing Docker PATH block: %v", err)
	}
	if err := LinkDotfiles(context.Background(), dotfiles, logger); err != nil {
		t.Fatalf("second LinkDotfiles: %v", err)
	}

	info, err = os.Lstat(homeProfile)
	if err != nil {
		t.Fatalf("stat HOME/.profile after second sync: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("second LinkDotfiles re-symlinked HOME/.profile")
	}
	body, err = os.ReadFile(homeProfile)
	if err != nil {
		t.Fatalf("reading HOME/.profile after second sync: %v", err)
	}
	if !strings.Contains(string(body), "End of Docker Desktop section") {
		t.Fatalf("second LinkDotfiles wiped Docker PATH block: %q", body)
	}
}

func TestLinkDotfilesWritesProfileWhenMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg-data"))

	dotfiles := t.TempDir()
	repoProfile := filepath.Join(dotfiles, "home", ".profile")
	if err := os.MkdirAll(filepath.Join(dotfiles, "home"), 0o755); err != nil {
		t.Fatalf("creating home dir: %v", err)
	}
	if err := os.WriteFile(repoProfile, []byte("repo-profile\n"), 0o644); err != nil {
		t.Fatalf("writing repo profile: %v", err)
	}

	logger, _ := newBackupTestLogger(t)
	if err := LinkDotfiles(context.Background(), dotfiles, logger); err != nil {
		t.Fatalf("LinkDotfiles: %v", err)
	}

	homeProfile := filepath.Join(home, ".profile")
	info, err := os.Lstat(homeProfile)
	if err != nil {
		t.Fatalf("stat HOME/.profile: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("HOME/.profile was created as a symlink")
	}
	body, err := os.ReadFile(homeProfile)
	if err != nil {
		t.Fatalf("reading HOME/.profile: %v", err)
	}
	if !strings.Contains(string(body), repoProfile) {
		t.Fatalf("local profile does not source %s: %q", repoProfile, body)
	}
}

func TestLinkDotfilesAppendsSourceToExistingRegularProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg-data"))

	dotfiles := t.TempDir()
	repoProfile := filepath.Join(dotfiles, "home", ".profile")
	if err := os.MkdirAll(filepath.Join(dotfiles, "home"), 0o755); err != nil {
		t.Fatalf("creating home dir: %v", err)
	}
	if err := os.WriteFile(repoProfile, []byte("repo-profile\n"), 0o644); err != nil {
		t.Fatalf("writing repo profile: %v", err)
	}

	homeProfile := filepath.Join(home, ".profile")
	existing := "export FOO=1\n"
	if err := os.WriteFile(homeProfile, []byte(existing), 0o644); err != nil {
		t.Fatalf("writing existing profile: %v", err)
	}

	logger, _ := newBackupTestLogger(t)
	if err := LinkDotfiles(context.Background(), dotfiles, logger); err != nil {
		t.Fatalf("LinkDotfiles: %v", err)
	}

	info, err := os.Lstat(homeProfile)
	if err != nil {
		t.Fatalf("stat HOME/.profile: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("HOME/.profile became a symlink")
	}
	body, err := os.ReadFile(homeProfile)
	if err != nil {
		t.Fatalf("reading HOME/.profile: %v", err)
	}
	if !strings.HasPrefix(string(body), existing) {
		t.Fatalf("existing profile content was not preserved: %q", body)
	}
	if !strings.Contains(string(body), repoProfile) {
		t.Fatalf("local profile does not source %s: %q", repoProfile, body)
	}
}

func TestLinkDotfilesRefreshesStaleProfileSource(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg-data"))

	dotfiles := t.TempDir()
	repoProfile := filepath.Join(dotfiles, "home", ".profile")
	if err := os.MkdirAll(filepath.Join(dotfiles, "home"), 0o755); err != nil {
		t.Fatalf("creating home dir: %v", err)
	}
	if err := os.WriteFile(repoProfile, []byte("repo-profile\n"), 0o644); err != nil {
		t.Fatalf("writing repo profile: %v", err)
	}

	homeProfile := filepath.Join(home, ".profile")
	stale := "# Docker PATH\nexport PATH=\"$PATH:/tmp/docker-bin\"\n" +
		localProfileSentinel + " Docker Desktop may write a PATH block here.\n" +
		"if [ -f '/old/checkout/home/.profile' ]; then\n" +
		"    . '/old/checkout/home/.profile'\n" +
		"fi\n" +
		"# extra after managed block\n"
	if err := os.WriteFile(homeProfile, []byte(stale), 0o644); err != nil {
		t.Fatalf("writing stale profile: %v", err)
	}

	logger, _ := newBackupTestLogger(t)
	if err := LinkDotfiles(context.Background(), dotfiles, logger); err != nil {
		t.Fatalf("LinkDotfiles: %v", err)
	}

	body, err := os.ReadFile(homeProfile)
	if err != nil {
		t.Fatalf("reading HOME/.profile: %v", err)
	}
	text := string(body)
	if !strings.Contains(text, "export PATH=\"$PATH:/tmp/docker-bin\"") {
		t.Fatalf("refresh wiped Docker PATH: %q", body)
	}
	if strings.Contains(text, "/old/checkout/home/.profile") {
		t.Fatalf("stale source path remains: %q", body)
	}
	if !strings.Contains(text, repoProfile) {
		t.Fatalf("refreshed profile does not source %s: %q", repoProfile, body)
	}
	if !strings.Contains(text, "# extra after managed block") {
		t.Fatalf("refresh dropped content after the managed block: %q", body)
	}
}

func TestLinkDotfilesReplacesForeignProfileSymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg-data"))

	dotfiles := t.TempDir()
	repoProfile := filepath.Join(dotfiles, "home", ".profile")
	if err := os.MkdirAll(filepath.Join(dotfiles, "home"), 0o755); err != nil {
		t.Fatalf("creating home dir: %v", err)
	}
	if err := os.WriteFile(repoProfile, []byte("repo-profile\n"), 0o644); err != nil {
		t.Fatalf("writing repo profile: %v", err)
	}

	other := filepath.Join(home, "other-profile")
	wrapper := localProfileSentinel + " Docker Desktop may write a PATH block here.\n" +
		"if [ -f " + shellSingleQuote(repoProfile) + " ]; then\n" +
		"    . " + shellSingleQuote(repoProfile) + "\n" +
		"fi\n"
	if err := os.WriteFile(other, []byte(wrapper), 0o600); err != nil {
		t.Fatalf("writing other profile: %v", err)
	}
	homeProfile := filepath.Join(home, ".profile")
	if err := os.Symlink(other, homeProfile); err != nil {
		t.Fatalf("creating foreign profile symlink: %v", err)
	}

	logger, _ := newBackupTestLogger(t)
	if err := LinkDotfiles(context.Background(), dotfiles, logger); err != nil {
		t.Fatalf("LinkDotfiles: %v", err)
	}

	info, err := os.Lstat(homeProfile)
	if err != nil {
		t.Fatalf("stat HOME/.profile: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("HOME/.profile is still a symlink")
	}
	body, err := os.ReadFile(homeProfile)
	if err != nil {
		t.Fatalf("reading HOME/.profile: %v", err)
	}
	if !strings.Contains(string(body), repoProfile) {
		t.Fatalf("replaced profile does not source %s: %q", repoProfile, body)
	}
}

func TestShellSingleQuote(t *testing.T) {
	got := shellSingleQuote(`foo'bar`)
	want := `'foo'\''bar'`
	if got != want {
		t.Fatalf("shellSingleQuote(%q) = %q, want %q", `foo'bar`, got, want)
	}
}
