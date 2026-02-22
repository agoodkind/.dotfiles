export PATH="$PATH:$HOME/.local/bin:"
export PATH="$PATH:$HOME/.local/bin/scripts"
export PATH="$PATH:/opt/scripts"
export PATH="$PATH:$HOME/.cargo/bin"
export PATH="$PATH:$HOME/go/bin"
export NVM_LAZY_LOAD=true

# shellcheck shell=bash
# Check if profiling was requested and load zprof module early if needed
SHOULD_PROFILE=false
if [[ -f ~/.cache/zsh_profile_next ]]; then
    SHOULD_PROFILE=true
    zmodload zsh/zprof
    rm ~/.cache/zsh_profile_next

    do_profile() {
        echo "Zsh performance profiling results:"
        zprof
        printf "Zsh initialization time: %.0f ms\n" "$(( (EPOCHREALTIME - START_TIME) * 1000 ))"
    }
fi
export SHOULD_PROFILE

source "$DOTDOTFILES/zshrc/core/plugins.zsh"
source "$DOTDOTFILES/zshrc/core/utils.zsh"
source "$DOTDOTFILES/zshrc/commands/prefer.zsh"
source "$DOTDOTFILES/zshrc/commands/editors.zsh"
source "$DOTDOTFILES/zshrc/commands/remote.zsh"
source "$DOTDOTFILES/zshrc/commands/aliases.zsh"
source "$DOTDOTFILES/zshrc/commands/git.zsh"
(bash "$DOTDOTFILES/bash/updater.bash" >/dev/null 2>&1 &)
source "$DOTDOTFILES/zshrc/integrations/zoxide.zsh"
source "$DOTDOTFILES/zshrc/integrations/motd.zsh"
# Load local zshrc customizations that are not to be tracked by git
[[ ! -f "$DOTDOTFILES/.zshrc.local" ]] || source "$DOTDOTFILES/.zshrc.local"

# Check for update status from background updater
if [[ -f ~/.cache/dotfiles_update.lock ]]; then
    local update_type
    update_type=$(<~/.cache/dotfiles_update.lock)
    if [[ "$update_type" == "weekly" ]]; then
        print -P "%F{blue}↻ weekly update running in background%f"
    elif [[ "$update_type" == "sync" ]]; then
        print -P "%F{blue}↻ dotfiles sync running in background%f"
    else
        print -P "%F{blue}↻ checking for dotfiles updates...%f"
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

