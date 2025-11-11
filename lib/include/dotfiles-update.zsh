# ~/.dotfiles/lib/include/dotfiles-update.zsh
DOTFILES_GIT_HASH_FILE="$HOME/.cache/dotfiles-git-hash"
DOTFILES_GIT_DIR="$DOTDOTFILES/.git"
DOTFILES_FETCH_IN_PROGRESS="$HOME/.cache/dotfiles-fetch-in-progress"
DOTFILES_FETCH_DONE="$HOME/.cache/dotfiles-fetch-done"
if [[ ! -f "$DOTFILES_FETCH_IN_PROGRESS" && ! -f "$DOTFILES_FETCH_DONE" ]]; then
    touch "$DOTFILES_FETCH_IN_PROGRESS"
    (
        if git --git-dir="$DOTFILES_GIT_DIR" --work-tree="$DOTDOTFILES" fetch --quiet --all 2>/dev/null; then
            git --git-dir="$DOTFILES_GIT_DIR" rev-parse HEAD 2>/dev/null >| "$DOTFILES_GIT_HASH_FILE"
            touch "$DOTFILES_FETCH_DONE"
        fi
        rm -f "$DOTFILES_FETCH_IN_PROGRESS"
    ) &
fi
if [[ -f "$DOTFILES_FETCH_DONE" ]]; then
    echo "Dotfiles fetch completed in background. You may want to reload."
    rm -f "$DOTFILES_FETCH_DONE"
fi
# Only update the hash file after a successful fetch (in the background block above)
