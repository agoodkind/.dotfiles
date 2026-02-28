export PATH="$PATH:$HOME/.local/bin:"
export PATH="$PATH:$HOME/.local/bin/scripts"
export PATH="$PATH:/opt/scripts"
export PATH="$PATH:$HOME/.cargo/bin"
export PATH="$PATH:$HOME/go/bin"
export NVM_LAZY_LOAD=true

# shellcheck shell=bash
source "$DOTDOTFILES/zshrc/core/profile.zsh"

# plugins.zsh uses plain source (zinit turbo mode stores scope references
# that break when sourced inside a function). Timing is done inline instead.
local _t0=$EPOCHREALTIME
source "$DOTDOTFILES/zshrc/core/plugins.zsh"
_PROFILE_TIMES[plugins]=$(( (EPOCHREALTIME - _t0) * 1000 ))

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

# Check for update status from background updater
if [[ -f ~/.cache/dotfiles_update.lock ]]; then
    local update_type
    update_type=$(<~/.cache/dotfiles_update.lock)
    if [[ "$update_type" == "weekly" ]]; then
        print -P "%F{blue}↻ weekly update running in background%f"
    elif [[ "$update_type" == "sync" ]]; then
        print -P "%F{blue}↻ dotfiles sync running in background%f"
    fi
elif [[ -f ~/.cache/dotfiles_local_changes ]]; then
    local msg
    msg=$(<~/.cache/dotfiles_local_changes)
    print -P "%F{yellow}⚠️  ${msg}%f"
    rm -f ~/.cache/dotfiles_local_changes
elif [[ -f ~/.cache/dotfiles_update_error ]]; then
    local err_msg
    err_msg=$(<~/.cache/dotfiles_update_error)
    print -P "%F{red}⚠️  ${err_msg}%f"
    rm -f ~/.cache/dotfiles_update_error
elif [[ -f ~/.cache/dotfiles_weekly_update_success ]]; then
    print -P "%F{green}✓ Weekly full update completed (zinit, nvim, repair)%f"
    rm -f ~/.cache/dotfiles_weekly_update_success
elif [[ -f ~/.cache/dotfiles_update_success ]]; then
    print -P "%F{green}✓ Dotfiles updated in background%f"
    rm -f ~/.cache/dotfiles_update_success
fi

