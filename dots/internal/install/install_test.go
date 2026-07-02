package install

import (
	"os"
	"path/filepath"
	"slices"
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
