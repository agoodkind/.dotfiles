package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestZinitUpdateScriptKeepsUpdateExitNonfatal(t *testing.T) {
	requiredFragments := []string{
		`zsh -c 'source "$1"; zinit update --all --quiet' zsh "$1"`,
		`printf "[zinit-update-exit: %d]\n" "$update_rc"`,
		`printf "[zinit-compile-exit: %d]\n" "$compile_rc"`,
		`printf "[zinit-plugins-dir: %s]\n" "$plugins_dir"`,
		`(( self_update_rc == 0 && compile_rc == 0 ))`,
	}

	for _, fragment := range requiredFragments {
		if !strings.Contains(zinitUpdateScript, fragment) {
			t.Fatalf("zinitUpdateScript missing fragment %q", fragment)
		}
	}

	if strings.Contains(zinitUpdateScript, "&& update_rc == 0") {
		t.Fatal("zinitUpdateScript treats zinit update's unreliable exit code as fatal")
	}
}

func TestZinitUpdateScriptIgnoresUpdateExitAfterCleanVerification(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is not available")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}

	tempDir := t.TempDir()
	pluginsDir := filepath.Join(tempDir, "plugins")
	pluginDir := filepath.Join(pluginsDir, "zsh-users---zsh-completions")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatalf("creating fake plugin dir: %v", err)
	}

	initCmd := exec.Command("git", "-C", pluginDir, "init")
	if output, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("initializing fake plugin repo: %v\n%s", err, output)
	}

	zinitPath := filepath.Join(tempDir, "zinit.zsh")
	zinitStub := fmt.Sprintf(`
typeset -gA ZINIT
ZINIT[PLUGINS_DIR]=%q

zinit() {
    case "$1" in
        self-update)
            return 0
            ;;
        update)
            if [[ "$2" = --all && "$3" = --quiet ]]; then
                return 1
            fi
            ;;
        compile)
            if [[ "$2" = --all ]]; then
                setopt NO_GLOB
                return 0
            fi
            ;;
    esac
    printf "unexpected zinit call: %%s\n" "$*"
    return 2
}
`, pluginsDir)
	if err := os.WriteFile(zinitPath, []byte(zinitStub), 0o644); err != nil {
		t.Fatalf("writing fake zinit init file: %v", err)
	}

	cmd := exec.Command("zsh", "-c", zinitUpdateScript, "zsh", zinitPath)
	output, err := cmd.CombinedOutput()
	outputText := string(output)
	if err != nil {
		t.Fatalf("running zinit update script: %v\n%s", err, outputText)
	}

	requiredOutput := []string{
		"[zinit-self-update-exit: 0]",
		"[zinit-update-exit: 1]",
		"[zinit-compile-exit: 0]",
		"[zinit-plugins-dir: " + pluginsDir + "]",
	}
	for _, fragment := range requiredOutput {
		if !strings.Contains(outputText, fragment) {
			t.Fatalf("zinit update script output missing %q:\n%s", fragment, outputText)
		}
	}

	if err := verifyZinitPlugins(context.Background(), pluginsDir, nil); err != nil {
		t.Fatalf("verifying zinit plugins: %v", err)
	}
	updateExitCode, ok := zinitUpdateExitFromOutput(outputText)
	if !ok {
		t.Fatalf("zinit update script output missing parsed update exit:\n%s", outputText)
	}
	if updateExitCode != 1 {
		t.Fatalf("zinitUpdateExitFromOutput() = %d, want 1", updateExitCode)
	}
}

func TestVerifyZinitPluginsDetectsDetachedHead(t *testing.T) {
	pluginsDir := t.TempDir()
	pluginGitDir := filepath.Join(pluginsDir, "zsh-users---zsh-completions", ".git")
	if err := os.MkdirAll(pluginGitDir, 0o755); err != nil {
		t.Fatalf("creating fake git dir: %v", err)
	}
	headPath := filepath.Join(pluginGitDir, "HEAD")
	if err := os.WriteFile(headPath, []byte("abc123\n"), 0o644); err != nil {
		t.Fatalf("writing detached git state: %v", err)
	}

	err := verifyZinitPlugins(context.Background(), pluginsDir, nil)
	if err == nil {
		t.Fatal("verifyZinitPlugins() succeeded, want detached plugin failure")
	}
}

func TestZinitPluginHeadPathSupportsGitFile(t *testing.T) {
	pluginDir := t.TempDir()
	actualGitDir := filepath.Join(pluginDir, "actual-git")
	if err := os.MkdirAll(actualGitDir, 0o755); err != nil {
		t.Fatalf("creating actual git dir: %v", err)
	}
	headPath := filepath.Join(actualGitDir, "HEAD")
	if err := os.WriteFile(headPath, []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		t.Fatalf("writing git state: %v", err)
	}
	gitFile := filepath.Join(pluginDir, ".git")
	if err := os.WriteFile(gitFile, []byte("gitdir: actual-git\n"), 0o644); err != nil {
		t.Fatalf("writing .git file: %v", err)
	}

	got, ok, err := zinitPluginHeadPath(pluginDir)
	if err != nil {
		t.Fatalf("zinitPluginHeadPath() error = %v", err)
	}
	if !ok {
		t.Fatal("zinitPluginHeadPath() ok = false, want true")
	}
	if got != headPath {
		t.Fatalf("zinitPluginHeadPath() = %q, want %q", got, headPath)
	}
}

func TestZinitUpdateScriptAgainstInstalledZinit(t *testing.T) {
	if os.Getenv("DOTFILES_RUN_REAL_ZINIT_TEST") != "1" {
		t.Skip("set DOTFILES_RUN_REAL_ZINIT_TEST=1 to run against installed zinit")
	}
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is not available")
	}

	dotfiles := os.Getenv("DOTDOTFILES")
	if dotfiles == "" {
		t.Fatal("DOTDOTFILES must point at the dotfiles checkout")
	}
	zinitPath := filepath.Join(dotfiles, "lib", "zinit", "zinit.zsh")
	if _, err := os.Stat(zinitPath); err != nil {
		t.Fatalf("checking installed zinit init file: %v", err)
	}

	cmd := exec.Command("zsh", "-c", zinitUpdateScript, "zsh", zinitPath)
	output, err := cmd.CombinedOutput()
	outputText := string(output)
	if err != nil {
		t.Fatalf("running installed zinit update script: %v\n%s", err, outputText)
	}

	pluginsDir, ok := zinitMarkerValue(outputText, "zinit-plugins-dir")
	if !ok {
		t.Fatalf("installed zinit update script did not report plugins dir:\n%s", outputText)
	}
	if err := verifyZinitPlugins(context.Background(), pluginsDir, nil); err != nil {
		t.Fatalf("verifying installed zinit plugins: %v\n%s", err, outputText)
	}
}
