package githook

import (
	"os"
	"testing"
)

func TestHarnessActiveIgnoresCursorAndDotfilesShell(t *testing.T) {
	t.Setenv("CLAUDECODE", "")
	t.Setenv("CODEX_CI", "")
	t.Setenv("GEMINI_CLI", "")
	t.Setenv("CURSOR_AGENT", "1")
	t.Setenv("DOTFILES_AGENT_SHELL", "1")
	if harnessActive() {
		t.Fatal("harnessActive() = true, want false for Cursor-only env")
	}
}

func TestHarnessActiveDetectsAgentCLIs(t *testing.T) {
	cases := []string{"CLAUDECODE", "CODEX_CI", "GEMINI_CLI"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CLAUDECODE", "")
			t.Setenv("CODEX_CI", "")
			t.Setenv("GEMINI_CLI", "")
			t.Setenv(name, "1")
			if !harnessActive() {
				t.Fatalf("harnessActive() = false, want true when %s is set", name)
			}
		})
	}
}

func TestHarnessActiveClaudeWithCursorStillActive(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	t.Setenv("CURSOR_AGENT", "1")
	t.Setenv("CODEX_CI", "")
	t.Setenv("GEMINI_CLI", "")
	if !harnessActive() {
		t.Fatal("harnessActive() = false, want true when CLAUDECODE is set with CURSOR_AGENT")
	}
}

func TestMain(m *testing.M) {
	_ = os.Unsetenv("CLAUDECODE")
	_ = os.Unsetenv("CODEX_CI")
	_ = os.Unsetenv("GEMINI_CLI")
	os.Exit(m.Run())
}
