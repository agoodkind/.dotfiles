export PATH="$PATH:$HOME/.local/bin:"
export PATH="$PATH:$HOME/.local/bin/scripts"
export PATH="$PATH:/opt/scripts"
export PATH="$PATH:$HOME/.cargo/bin"
export PATH="$PATH:$HOME/go/bin"
export NVM_LAZY_LOAD=true

# shellcheck shell=bash
source "$DOTDOTFILES/zshrc/core/perf.zsh"

# Structural header: all _source/_async calls below become depth-2 children.
# ms is 0 here; .zshrc epilogue patches it with the actual total.
typeset -gi _ZSHRC_TREE_IDX=$(( ${#_PERF_TREE} + 1 ))
_PERF_TREE+=("1:.zshrc:0")

# plugins.zsh uses plain source (zinit turbo mode stores scope references
# that break when sourced inside a function). Timing is done inline instead.
local _t0=$EPOCHREALTIME
source "$DOTDOTFILES/zshrc/core/plugins.zsh"
local _pms=$(( (EPOCHREALTIME - _t0) * 1000 ))
_PROFILE_TIMES[plugins]=$_pms
_PERF_TREE+=("$(( _SOURCE_DEPTH + 2 )):plugins:${_pms}")

_source "$DOTDOTFILES/zshrc/core/utils.zsh"
_source "$DOTDOTFILES/zshrc/commands/prefer.zsh"
_source "$DOTDOTFILES/zshrc/commands/editors.zsh"
_source "$DOTDOTFILES/zshrc/commands/remote.zsh"
_source "$DOTDOTFILES/zshrc/commands/aliases.zsh"
_source "$DOTDOTFILES/zshrc/commands/git.zsh"
_async bash "$DOTDOTFILES/bash/background/dispatch.bash"
_source "$DOTDOTFILES/zshrc/integrations/zoxide.zsh"
_source "$DOTDOTFILES/zshrc/integrations/motd.zsh"
[[ ! -f "$DOTDOTFILES/.zshrc.local" ]] || _source "$DOTDOTFILES/.zshrc.local"

# Show transient "running now" state from background updater
if [[ -f ~/.cache/dotfiles_update.lock ]]; then
    local _update_type
    _update_type=$(<~/.cache/dotfiles_update.lock)
    if [[ "$_update_type" == "weekly" ]]; then
        print -P "%F{blue}↻ weekly update running in background%f"
    elif [[ "$_update_type" == "sync" ]]; then
        print -P "%F{blue}↻ dotfiles sync running in background%f"
    fi
fi

# Display all queued notifications from background processes, one per line
local _notify_file="$HOME/.cache/dotfiles/notifications"
if [[ -f "$_notify_file" ]]; then
    local _level _msg
    while IFS='|' read -r _level _msg; do
        case "$_level" in
            success) print -P "%F{green}✓ ${_msg}%f" ;;
            info)    print -P "%F{blue}↻ ${_msg}%f" ;;
            warn)    print -P "%F{yellow}⚠  ${_msg}%f" ;;
            error)   print -P "%F{red}✗ ${_msg}%f" ;;
        esac
    done < "$_notify_file"
    rm -f "$_notify_file"
fi

