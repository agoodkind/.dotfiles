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
			body: `(( ! ${+ZSH_AUTOSUGGEST_ACCEPT_WIDGETS} )) && typeset -ga ZSH_AUTOSUGGEST_ACCEPT_WIDGETS=(forward-char end-of-line vi-forward-char vi-end-of-line vi-add-eol)
function _zsh_autosuggest_start() { :; }
function _zsh_autosuggest_bind_widgets() { :; }
`,
		},
		{
			directory: "zsh-users---zsh-syntax-highlighting",
			file:      "zsh-syntax-highlighting.plugin.zsh",
			body:      "function _dotfiles_zsh_syntax_loaded() { :; }\n",
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

	binDirectory := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDirectory, 0o755); err != nil {
		t.Fatalf("creating fake bin directory: %v", err)
	}
	fzfPath := filepath.Join(binDirectory, "fzf")
	fzfScript := `#!/usr/bin/env bash
if [[ "$1" == "--zsh" ]]; then
    cat <<'EOF'
function fzf-completion() { :; }
zle -N fzf-completion
bindkey '^I' fzf-completion
function fzf-tab-complete() { :; }
zle -N fzf-tab-complete
bindkey -M emacs '^I' fzf-tab-complete
EOF
    exit 0
fi
exit 1
`
	if err := os.WriteFile(fzfPath, []byte(fzfScript), 0o755); err != nil {
		t.Fatalf("writing fake fzf binary: %v", err)
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

function _require_value() {
    local label=$1
    local expected=$2
    local actual=$3

    if [[ "$actual" != "$expected" ]]; then
        print -r -- "$label expected $expected but got $actual"
        exit 1
    fi
}

function _bound_widget() {
    local keymap=$1
    local binding

    binding=$(builtin bindkey -M "$keymap" '^I')
    print -r -- "${binding##* }"
}

setopt noglob no_nomatch
bindkey -v
source "$DOTDOTFILES/zshrc/core/plugins.zsh"

_load_tier1
_require_function _zsh_autosuggest_start
if [[ -n ${ZSH_AUTOSUGGEST_ACCEPT_WIDGETS[(r)forward-char]} ]]; then
    print -r -- "forward-char stayed in ZSH_AUTOSUGGEST_ACCEPT_WIDGETS"
    exit 1
fi
if [[ -n ${ZSH_AUTOSUGGEST_ACCEPT_WIDGETS[(r)vi-forward-char]} ]]; then
    print -r -- "vi-forward-char stayed in ZSH_AUTOSUGGEST_ACCEPT_WIDGETS"
    exit 1
fi

_load_tier2
_require_function _dotfiles_zsh_syntax_loaded
_require_function _dotfiles_fzf_tab_loaded
_require_function _dotfiles_fzf_tab_source_loaded
_require_value "emacs tab widget" "_dotfiles_tab_accept_or_complete" "$(_bound_widget emacs)"
_require_value "viins tab widget" "_dotfiles_tab_accept_or_complete" "$(_bound_widget viins)"

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
		"PATH="+binDirectory+string(os.PathListSeparator)+os.Getenv("PATH"),
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
