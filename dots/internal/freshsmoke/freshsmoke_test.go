package freshsmoke

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestInstallArgsIncludesSkipGitBeforeExtraArgs(t *testing.T) {
	got := installArgs("/repo/install.sh", "--strict")
	want := []string{"/repo/install.sh", "--use-defaults", "--skip-git", "--strict"}
	if !slices.Equal(got, want) {
		t.Fatalf("installArgs() = %#v, want %#v", got, want)
	}
}

func TestAssertSmokeSubmodulesPresentAcceptsCheckedOutSubmodules(t *testing.T) {
	repoRoot := t.TempDir()
	for _, submodule := range requiredSmokeSubmodules {
		submoduleDir := filepath.Join(repoRoot, submodule)
		if err := os.MkdirAll(submoduleDir, 0o755); err != nil {
			t.Fatalf("creating submodule dir %s: %v", submodule, err)
		}
		if err := os.WriteFile(filepath.Join(submoduleDir, ".git"), []byte("gitdir: ../../.git/modules/"+submodule+"\n"), 0o600); err != nil {
			t.Fatalf("writing submodule gitdir %s: %v", submodule, err)
		}
	}

	if err := AssertSmokeSubmodulesPresent(repoRoot); err != nil {
		t.Fatalf("AssertSmokeSubmodulesPresent() returned error: %v", err)
	}
}

func TestAssertSmokeSubmodulesPresentReportsMissingSubmodules(t *testing.T) {
	err := AssertSmokeSubmodulesPresent(t.TempDir())
	if err == nil {
		t.Fatal("AssertSmokeSubmodulesPresent() returned nil, want missing submodule error")
	}
	for _, submodule := range requiredSmokeSubmodules {
		if !strings.Contains(err.Error(), submodule) {
			t.Fatalf("missing submodule error did not mention %s: %v", submodule, err)
		}
	}
	if !strings.Contains(err.Error(), "git submodule update --init --recursive") {
		t.Fatalf("missing submodule error did not mention repair command: %v", err)
	}
}
