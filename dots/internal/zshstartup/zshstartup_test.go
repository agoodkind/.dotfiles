package zshstartup

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestZinitDeferredLoadersWorkWithNoGlob(t *testing.T) {
	repoRoot := repositoryRoot(t)
	homeDirectory := t.TempDir()
	pluginsDirectory := filepath.Join(homeDirectory, ".local", "share", "zinit", "plugins")

	plugins := []struct {
		directory string
		file      string
		body      string
	}{
		{
			directory: "zsh-users---zsh-autosuggestions",
			file:      "zsh-autosuggestions.plugin.zsh",
			body:      "function _zsh_autosuggest_start() { :; }\n",
		},
		{
			directory: "zdharma-continuum---fast-syntax-highlighting",
			file:      "fast-syntax-highlighting.plugin.zsh",
			body:      "function _dotfiles_fast_syntax_loaded() { :; }\n",
		},
		{
			directory: "Aloxaf---fzf-tab",
			file:      "fzf-tab.plugin.zsh",
			body:      "function _dotfiles_fzf_tab_loaded() { :; }\n",
		},
		{
			directory: "Freed-Wu---fzf-tab-source",
			file:      "fzf-tab-source.plugin.zsh",
			body:      "function _dotfiles_fzf_tab_source_loaded() { :; }\n",
		},
		{
			directory: "zsh-users---zsh-completions",
			file:      "zsh-completions.plugin.zsh",
			body:      "function _dotfiles_zsh_completions_loaded() { :; }\n",
		},
	}

	for _, plugin := range plugins {
		pluginDirectory := filepath.Join(pluginsDirectory, plugin.directory)
		if err := os.MkdirAll(pluginDirectory, 0o755); err != nil {
			t.Fatalf("creating fake plugin directory %s: %v", plugin.directory, err)
		}
		pluginPath := filepath.Join(pluginDirectory, plugin.file)
		if err := os.WriteFile(pluginPath, []byte(plugin.body), 0o644); err != nil {
			t.Fatalf("writing fake plugin %s: %v", plugin.directory, err)
		}
	}

	scriptPath := filepath.Join(t.TempDir(), "zinit-noglob-regression.zsh")
	script := `
zmodload zsh/datetime
zmodload zsh/sched

START_TIME=$EPOCHREALTIME
typeset -gA _PROFILE_TIMES
typeset -ga _PERF_TREE _ZSH_ARR

function _write_startup_log() {
    :
}

function _zsplit_colon() {
    _ZSH_ARR=("${(@s.:.)1}")
}

function _require_function() {
    local function_name=$1
    if (( ${+functions[$function_name]} == 0 )); then
        print -r -- "$function_name was not loaded"
        exit 1
    fi
}

setopt noglob no_nomatch
source "$DOTDOTFILES/zshrc/core/plugins.zsh"

_load_tier1
_require_function _zsh_autosuggest_start

_load_tier2
_require_function _dotfiles_fast_syntax_loaded
_require_function _dotfiles_fzf_tab_loaded
_require_function _dotfiles_fzf_tab_source_loaded

_load_tier3
_require_function _dotfiles_zsh_completions_loaded

if [[ ! -o noglob ]]; then
    print -r -- "noglob was not restored after local zinit loaders"
    exit 1
fi

print -r -- "zinit deferred loaders work with noglob"
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("writing regression script: %v", err)
	}

	cmd := exec.Command("zsh", "-f", scriptPath)
	cmd.Env = append(
		os.Environ(),
		"DOTDOTFILES="+repoRoot,
		"HOME="+homeDirectory,
		"LS_COLORS=",
		"TERM=xterm-256color",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("running zinit noglob regression script: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "zinit deferred loaders work with noglob") {
		t.Fatalf("regression script output missing success marker:\n%s", output)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	directory, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	for {
		pluginsPath := filepath.Join(directory, "zshrc", "core", "plugins.zsh")
		zinitPath := filepath.Join(directory, "lib", "zinit", "zinit.zsh")
		if _, err := os.Stat(pluginsPath); err == nil {
			if _, err := os.Stat(zinitPath); err == nil {
				return directory
			}
		}

		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("repository root with zshrc/core/plugins.zsh was not found")
		}
		directory = parent
	}
}
