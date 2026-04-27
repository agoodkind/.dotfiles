#!/usr/bin/env bash
# Chain to per-repo hook, then ~/.git-hooks/ fallback
# Usage: chain_hook <hook_name> "$@"

chain_hook() {
    local hook_name="$1"
    shift

    local repo_hook
    repo_hook="$(git rev-parse --git-dir)/hooks/$hook_name"
    if [[ -x "$repo_hook" ]]; then
        if [[ "$hook_name" == "pre-push" && -n "${CHAIN_HOOK_STDIN_FILE:-}" ]]; then
            "$repo_hook" "$@" <"$CHAIN_HOOK_STDIN_FILE" || exit $?
        else
            "$repo_hook" "$@" || exit $?
        fi
    fi

    local user_hook="$HOME/.git-hooks/$hook_name"
    if [[ -x "$user_hook" ]]; then
        if [[ "$hook_name" == "pre-push" && -n "${CHAIN_HOOK_STDIN_FILE:-}" ]]; then
            "$user_hook" "$@" <"$CHAIN_HOOK_STDIN_FILE" || exit $?
        else
            "$user_hook" "$@" || exit $?
        fi
    fi
}
