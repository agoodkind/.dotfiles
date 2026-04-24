export PATH="$PATH:/opt/scripts"
export PATH="$PATH:$HOME/.cargo/bin"
export PATH="$PATH:$HOME/go/bin"
export NVM_LAZY_LOAD=true

# shellcheck shell=bash
source "$DOTDOTFILES/zshrc/core/perf.zsh"

# Structural header: all _source/_async calls below become depth-2 children.
# ms is 0 here; .zshrc epilogue patches it with the actual total.
typeset -gi _ZSHRC_TREE_IDX=$((${#_PERF_TREE} + 1))
_PERF_TREE+=("1:.zshrc:0")

# plugins.zsh uses plain source (zinit turbo mode stores scope references
# that break when sourced inside a function). Timing is done inline instead.
local _t0=$EPOCHREALTIME
source "$DOTDOTFILES/zshrc/core/plugins.zsh"
local _pms=$(((EPOCHREALTIME - _t0) * 1000))
_PROFILE_TIMES[plugins]=$_pms
_PERF_TREE+=("$((_SOURCE_DEPTH + 2)):plugins:${_pms}")

_source "$DOTDOTFILES/zshrc/core/utils.zsh"
_source "$DOTDOTFILES/zshrc/core/prefer.zsh"
_source "$DOTDOTFILES/zshrc/commands/editors.zsh"
_source "$DOTDOTFILES/zshrc/commands/remote.zsh"
_source "$DOTDOTFILES/zshrc/commands/aliases.zsh"
_source "$DOTDOTFILES/zshrc/integrations/zoxide.zsh"
_source "$DOTDOTFILES/zshrc/commands/prefer-decls.zsh"
_async bash -lc "builtin cd \"$DOTDOTFILES/dots\" && source \"$DOTDOTFILES/dots/bootstrap-go.sh\" && run_dots_go_command dispatch"
_source "$DOTDOTFILES/zshrc/integrations/motd.zsh"
if [[ -f "$DOTDOTFILES/.zshrc.local" ]]; then
    _source "$DOTDOTFILES/.zshrc.local"
fi

# Show transient "running now" state from background dispatch
if [[ -d ~/.cache/dotfiles_dispatch.lock ]]; then
    local _update_type=""
    if [[ -f ~/.cache/dotfiles_dispatch.lock/status ]]; then
        _update_type=$(<~/.cache/dotfiles_dispatch.lock/status)
    fi
    if [[ "$_update_type" == "weekly" ]]; then
        print -P "%F{blue}↻ weekly update running in background%f"
    elif [[ "$_update_type" == "sync" ]]; then
        print -P "%F{blue}↻ dotfiles sync running in background%f"
    fi
fi

# Display all queued notifications from background processes, one per line.
# Format on disk: level|logfile|message
local _notify_file="$HOME/.cache/dotfiles/notifications"
if [[ -f "$_notify_file" ]]; then
    local _level _logfile _msg _line
    while IFS= read -r _line; do
        _level="${_line%%|*}"
        _line="${_line#*|}"
        _logfile="${_line%%|*}"
        _msg="${_line#*|}"
        case "$_level" in
            success) print -P "%F{green}✓ ${_msg}%f" ;;
            info) print -P "%F{blue}↻ ${_msg}%f" ;;
            warn) print -P "%F{yellow}⚠  ${_msg}%f" ;;
            error) print -P "%F{red}✗ ${_msg}%f" ;;
        esac
        if [[ -n "$_logfile" && -f "$_logfile" ]]; then
            print -P "  %F{242}log: ${_logfile}%f"
        fi
    done <"$_notify_file"
    rm -f "$_notify_file"
fi
