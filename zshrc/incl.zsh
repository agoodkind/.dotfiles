# PATH additions and lazy-load knobs apply to every shell, so they run first.
export PATH="$PATH:$HOME/.cargo/bin:$HOME/go/bin"
export NVM_LAZY_LOAD=true

# shellcheck shell=bash
# perf.zsh defines _source/_async and must be a plain top-level source, since it
# is the harness every later call depends on.
source "$DOTDOTFILES/zshrc/core/perf.zsh"

# Structural header: every _source/_async below is a depth-2 child of this node.
# ms is 0 here; the .zshrc epilogue patches it with the real total.
typeset -gi _ZSHRC_TREE_IDX=$((${#_PERF_TREE} + 1))
_PERF_TREE+=("1:.zshrc:0")

_source "$DOTDOTFILES/zshrc/core/utils.zsh"

# Machine-local overrides load for every shell and before the interactive gate,
# so wrappers like claude/codex exist in agent and non-TTY shells too.
if [[ -f "$DOTDOTFILES/.zshrc.local" ]]; then
    local _zl_t0=$EPOCHREALTIME
    if source "$DOTDOTFILES/.zshrc.local" 2>/dev/null; then
        local _zl_ms=$(((EPOCHREALTIME - _zl_t0) * 1000))
        _PROFILE_TIMES['.zshrc.local']=$_zl_ms
        _PERF_TREE+=("$((_SOURCE_DEPTH + 2)):.zshrc.local:${_zl_ms}")
    fi
fi

# Agents and non-TTY shells need none of the interactive machinery, so they apply
# agent shell options if requested and then stop here.
if ((!DOTFILES_INTERACTIVE)); then
    if ((DOTFILES_AGENT_SHELL)) && (($+functions[dotfiles_apply_agent_shell_options])); then
        dotfiles_apply_agent_shell_options
    fi
    return 0 2>/dev/null || true
fi

# Interactive-only startup below.
_source "$DOTDOTFILES/zshrc/core/startup.zsh"

# A live install may be writing shell state, so fall back to minimal startup.
if _dotfiles_install_in_progress; then
    return 0 2>/dev/null || true
fi

# plugins.zsh must be a top-level plain source: zinit turbo stores scope
# references that break inside a function, so timing stays inline.
local _t0=$EPOCHREALTIME
source "$DOTDOTFILES/zshrc/core/plugins.zsh"
local _pms=$(((EPOCHREALTIME - _t0) * 1000))
_PROFILE_TIMES[plugins]=$_pms
_PERF_TREE+=("$((_SOURCE_DEPTH + 2)):plugins:${_pms}")

_source "$DOTDOTFILES/zshrc/core/prefer.zsh"
_source "$DOTDOTFILES/zshrc/commands/editors.zsh"
_source "$DOTDOTFILES/zshrc/commands/remote.zsh"
_source "$DOTDOTFILES/zshrc/commands/aliases.zsh"
_source "$DOTDOTFILES/zshrc/integrations/zoxide.zsh"
_source "$DOTDOTFILES/zshrc/commands/prefer-decls.zsh"

# Background maintenance; _async backgrounds it and records a dispatch node.
_async dots_dispatch

_source "$DOTDOTFILES/zshrc/integrations/motd.zsh"

_dotfiles_show_dispatch_banner
_dotfiles_show_notifications
