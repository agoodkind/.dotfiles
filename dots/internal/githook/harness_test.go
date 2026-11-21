package githook

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
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
		t.Run(
			name,
			func(t *testing.T) {
				t.Setenv("CLAUDECODE", "")
				t.Setenv("CODEX_CI", "")
				t.Setenv("GEMINI_CLI", "")
				t.Setenv(name, "1")
				if !harnessActive() {
					t.Fatalf("harnessActive() = false, want true when %s is set", name)
				}
			},
		)
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

var (
	dotsBuildRoot  string
	dotsBuildEnv   []string
	dotsGoEnv      map[string]string
	buildDotsOnce  sync.Once
	buildDotsError error
)

func dotsBinaryForTest(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(dotsBuildRoot, "dots")
	buildDotsOnce.Do(func() {
		command := exec.Command("go", "env", "-json", "GOCACHE", "GOMODCACHE", "GOPATH")
		command.Env = dotsBuildEnv
		output, err := command.Output()
		if err != nil {
			buildDotsError = fmt.Errorf("read Go paths: %w", err)

			return
		}

		if err := json.Unmarshal(output, &dotsGoEnv); err != nil {
			buildDotsError = err

			return
		}

		command = exec.Command("go", "build", "-buildvcs=false", "-o", binary, "./cmd/dots")
		command.Dir = filepath.Join(sourceRootForTest(), "dots")
		command.Env = dotsBuildEnv
		if output, err := command.CombinedOutput(); err != nil {
			buildDotsError = fmt.Errorf("build dots: %w\n%s", err, output)
		}
	})
	if buildDotsError != nil {
		t.Fatal(buildDotsError)
	}

	return binary
}

func TestMain(m *testing.M) {
	for _, name := range []string{
		"CLAUDECODE",
		"CODEX_CI",
		"GEMINI_CLI",
		"CURSOR_AGENT",
		"DOTFILES_AGENT_SHELL",
	} {
		if err := os.Unsetenv(name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	var err error
	dotsBuildRoot, err = os.MkdirTemp("", "dots-hook-tests-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	dotsBuildEnv = os.Environ()
	code := m.Run()
	if err := os.RemoveAll(dotsBuildRoot); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	os.Exit(code)
}
