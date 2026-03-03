#!/usr/bin/env bash
# Rebuild ~/.cache/zsh_startup/path_cache.zsh if the path_helper inputs have changed.
# This is macOS-only: path_helper and /etc/paths.d don't exist on Linux.
[[ "$(uname -s)" != "Darwin" ]] && exit 0
[[ ! -x /usr/libexec/path_helper ]] && exit 0

cache_dir="$HOME/.cache/zsh_startup"
cache_file="$cache_dir/path_cache.zsh"
mkdir -p "$cache_dir"

needs_rebuild() {
    [[ ! -f "$cache_file" ]] && return 0
    # Rebuild if any paths.d entry is newer than the cache.
    [[ /etc/paths -nt "$cache_file" ]] && return 0
    local f
    for f in /etc/paths.d/*; do
        [[ "$f" -nt "$cache_file" ]] && return 0
    done
    return 1
}

needs_rebuild || exit 0

/usr/libexec/path_helper -s > "$cache_file"
