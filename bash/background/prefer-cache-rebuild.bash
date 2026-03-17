#!/usr/bin/env bash
DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "$DOTDOTFILES/bash/core/tools.bash"
dotfiles_log_init "prefer-cache-rebuild"

PREFER_CACHE_FILE="$HOME/.cache/zsh_prefer_aliases.zsh"
INVALIDATE_MARKER="$HOME/.cache/zsh_prefer_invalidate"

needs_rebuild() {
    [[ -f "$INVALIDATE_MARKER" ]] && return 0
    [[ ! -f "$PREFER_CACHE_FILE" ]] && return 0

    for f in \
        "$DOTDOTFILES/zshrc/commands/prefer.zsh" \
        "$DOTDOTFILES/home/.zshrc" \
        "$DOTDOTFILES/.zshrc.local"; do
        [[ -f "$f" ]] && [[ "$f" -nt "$PREFER_CACHE_FILE" ]] && return 0
    done

    return 1
}

rebuild() {
    dotfiles_log "rebuilding prefer cache"
    rm -f "$PREFER_CACHE_FILE" "$INVALIDATE_MARKER"

    command -v zsh &>/dev/null || return 1

    dotfiles_run zsh -c '
        export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
        source "$DOTDOTFILES/zshrc/core/perf.zsh"
        source "$DOTDOTFILES/zshrc/core/utils.zsh"
        source "$DOTDOTFILES/zshrc/commands/prefer.zsh"
        source "$DOTDOTFILES/home/.zshrc"
    '
    dotfiles_log "done"
}

if [[ "${1:-}" == "--force" ]]; then
    rebuild
    exit $?
fi

if needs_rebuild; then
    rebuild
else
    dotfiles_log "cache up to date, skipping"
fi
