package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"goodkind.io/.dotfiles/internal/telemetry"
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

func TestParseBrewPermissionDeniedPaths(t *testing.T) {
	output := strings.Join([]string{
		"==> Running `brew cleanup`...",
		"Removing: /Users/x/Library/Caches/Homebrew/foo... (1KB)",
		"Error: Permission denied @ apply2files - /opt/homebrew/lib/python3.14/site-packages/__pycache__/typing_extensions.cpython-314.pyc",
		"Error: Permission denied @ rb_sysopen - /opt/homebrew/var/log/keep",
		"Error: Permission denied @ apply2files - /opt/homebrew/lib/python3.14/site-packages/__pycache__/typing_extensions.cpython-314.pyc",
	}, "\n")
	got := parseBrewPermissionDeniedPaths(output)
	want := []string{
		"/opt/homebrew/lib/python3.14/site-packages/__pycache__/typing_extensions.cpython-314.pyc",
		"/opt/homebrew/var/log/keep",
	}
	if len(got) != len(want) {
		t.Fatalf("parsed %d paths, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("path %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestRestoreOwnershipChainsRepairsChainOnly exercises the real sudo-backed
// repair end to end. It makes a nested cache dir and file root-owned, then
// asserts the repair restores that broken chain to the current user, reports
// the two repaired paths, and leaves a sibling file untouched. It is gated
// because it requires passwordless sudo and mutates ownership.
func TestRestoreOwnershipChainsRepairsChainOnly(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("requires unix chown semantics")
	}
	if os.Getenv("DOTFILES_RUN_REAL_SUDO_TEST") != "1" {
		t.Skip("set DOTFILES_RUN_REAL_SUDO_TEST=1 to run the sudo-backed ownership repair test")
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		t.Skip("sudo is not available")
	}
	current, err := user.Current()
	if err != nil {
		t.Fatalf("resolving current user: %v", err)
	}
	uid, err := strconv.Atoi(current.Uid)
	if err != nil {
		t.Fatalf("parsing current uid %q: %v", current.Uid, err)
	}

	root := t.TempDir()
	sibling := filepath.Join(root, "sibling.txt")
	if err := os.WriteFile(sibling, []byte("keep"), 0o644); err != nil {
		t.Fatalf("writing sibling: %v", err)
	}
	cacheDir := filepath.Join(root, "site-packages", "__pycache__")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("creating cache dir: %v", err)
	}
	target := filepath.Join(cacheDir, "mod.pyc")
	if err := os.WriteFile(target, []byte("orphan"), 0o644); err != nil {
		t.Fatalf("writing target: %v", err)
	}
	if output, err := exec.Command("sudo", "-n", "chown", "-R", "root", cacheDir).CombinedOutput(); err != nil {
		t.Skipf("cannot chown to root without a password prompt: %v\n%s", err, output)
	}

	logPath := filepath.Join(t.TempDir(), "repair.log")
	logger, err := telemetry.NewLogger(logPath)
	if err != nil {
		t.Fatalf("creating logger: %v", err)
	}
	defer logger.Close()

	count, err := restoreOwnershipChains(context.Background(), logger, []string{target})
	if err != nil {
		t.Fatalf("restoreOwnershipChains returned error: %v", err)
	}
	if count != 2 {
		t.Fatalf("restored %d paths, want 2 (the file and its __pycache__ ancestor)", count)
	}

	for _, repaired := range []string{target, cacheDir} {
		info, err := os.Stat(repaired)
		if err != nil {
			t.Fatalf("stat %s after repair: %v", repaired, err)
		}
		if int(info.Sys().(*syscall.Stat_t).Uid) != uid {
			t.Fatalf("%s uid = %d after repair, want %d", repaired, info.Sys().(*syscall.Stat_t).Uid, uid)
		}
	}

	info, err := os.Stat(sibling)
	if err != nil {
		t.Fatalf("stat sibling: %v", err)
	}
	if int(info.Sys().(*syscall.Stat_t).Uid) != uid {
		t.Fatal("sibling ownership changed; repair touched a path outside the broken chain")
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
