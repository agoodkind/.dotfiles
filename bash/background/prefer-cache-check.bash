#!/usr/bin/env bash
DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
PREFER_CACHE_FILE="$HOME/.cache/zsh_prefer_aliases.zsh"

[[ -f "$PREFER_CACHE_FILE" ]] || exit 0

# If any source file is newer than the cache, invalidate immediately.
# This covers local edits that don't change git HEAD.
for f in \
    "$DOTDOTFILES/zshrc/commands/prefer.zsh" \
    "$DOTDOTFILES/home/.zshrc" \
    "$DOTDOTFILES/.zshrc.local"; do
    [[ "$f" -nt "$PREFER_CACHE_FILE" ]] \
        && { rm -f "$PREFER_CACHE_FILE"; exit 0; }
done

# Resolve current git HEAD to a commit hash.
# HEAD is either a raw sha or "ref: refs/heads/branch",
# so we follow the indirection if needed.
head_hash=""
if [[ -f "$DOTDOTFILES/.git/HEAD" ]]; then
    read -r head_hash < "$DOTDOTFILES/.git/HEAD"
    if [[ "$head_hash" == ref:* ]]; then
        ref="${head_hash#ref: }"
        [[ -f "$DOTDOTFILES/.git/$ref" ]] && \
            read -r head_hash < "$DOTDOTFILES/.git/$ref"
    fi
fi

# The cache file's first line stores "# HASH: <commit>-<mtime>".
# Extract just the commit portion for comparison.
# If dotfiles moved to a different commit, the cached
# alias resolutions may be stale (new/removed binaries,
# changed prefer definitions), so we delete it and let
# the next shell rebuild.
cached_line=""
read -r cached_line < "$PREFER_CACHE_FILE"
cached_line="${cached_line#\# HASH: }"
cached_commit="${cached_line%%-*}"

[[ "$head_hash" != "$cached_commit" ]] && rm -f "$PREFER_CACHE_FILE"
