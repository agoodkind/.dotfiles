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

source "$DOTDOTFILES/lib/zsh/plugins.zsh"
# Dotfiles async update - source directly to avoid zinit plugin management
source "$DOTDOTFILES/lib/zsh/utils.zsh"
(zsh "$DOTDOTFILES/lib/zsh/updater.zsh" >/dev/null 2>&1 &)
source "$DOTDOTFILES/lib/zsh/zoxide.zsh"
source "$DOTDOTFILES/lib/zsh/motd.zsh"
# Load local zshrc customizations that are not to be tracked by git
[[ ! -f "$DOTDOTFILES/.zshrc.local" ]] || source "$DOTDOTFILES/.zshrc.local"

# Check for update status from background updater
if [[ -f ~/.cache/dotfiles_update_error ]]; then
    print -P "%F{red}⚠️  Dotfiles update failed. See: ~/.cache/dotfiles_update.log%f"
    rm -f ~/.cache/dotfiles_update_error
elif [[ -f ~/.cache/dotfiles_weekly_update_success ]]; then
    print -P "%F{green}✓ Weekly full update completed (zinit, nvim, repair)%f"
    rm -f ~/.cache/dotfiles_weekly_update_success
elif [[ -f ~/.cache/dotfiles_update_success ]]; then
    print -P "%F{green}✓ Dotfiles updated in background%f"
    rm -f ~/.cache/dotfiles_update_success
fi

