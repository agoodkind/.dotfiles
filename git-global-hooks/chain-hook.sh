#!/usr/bin/env bash
# Chain to per-repo hook, then ~/.git-hooks/ fallback
# Usage: chain_hook <hook_name> "$@"

chain_hook() {
    local hook_name="$1"
    shift

    local repo_hook
    repo_hook="$(git rev-parse --git-dir)/hooks/$hook_name"
    if [[ -x "$repo_hook" ]]; then
        "$repo_hook" "$@" || exit $?
    fi

    local user_hook="$HOME/.git-hooks/$hook_name"
    if [[ -x "$user_hook" ]]; then
        "$user_hook" "$@" || exit $?
    fi
}
