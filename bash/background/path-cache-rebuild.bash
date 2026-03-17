#!/usr/bin/env bash
# Rebuild ~/.cache/zsh_startup/path_cache.zsh if the path_helper inputs have changed.
# This is macOS-only: path_helper and /etc/paths.d don't exist on Linux.
[[ "$(uname -s)" != "Darwin" ]] && exit 0
[[ ! -x /usr/libexec/path_helper ]] && exit 0

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "$DOTDOTFILES/bash/core/tools.bash"
dotfiles_log_init "path-cache-rebuild"

cache_dir="$HOME/.cache/zsh_startup"
cache_file="$cache_dir/path_cache.zsh"
mkdir -p "$cache_dir"

needs_rebuild() {
    [[ ! -f "$cache_file" ]] && return 0
    [[ /etc/paths -nt "$cache_file" ]] && return 0
    local f
    for f in /etc/paths.d/*; do
        [[ "$f" -nt "$cache_file" ]] && return 0
    done
    return 1
}

if ! needs_rebuild; then
    dotfiles_log "cache up to date, skipping"
    exit 0
fi

dotfiles_log "rebuilding path cache"
dotfiles_run /usr/libexec/path_helper -s > "$cache_file"
dotfiles_log "done"
