#!/usr/bin/env bash
# Chain to per-repo hook, then ~/.git-hooks/ fallback
# Usage: chain_hook <hook_name> "$@"

chain_hook() {
    local hook_name="$1"
    shift

    local git_dir repo_hook
    git_dir="$(git rev-parse --git-dir 2>/dev/null || true)"
    repo_hook=""
    if [[ -n "$git_dir" ]]; then
        repo_hook="$git_dir/hooks/$hook_name"
    fi
    if [[ -n "$repo_hook" && -x "$repo_hook" ]]; then
        if [[ -n "${CHAIN_HOOK_STDIN_FILE:-}" ]]; then
            "$repo_hook" "$@" <"$CHAIN_HOOK_STDIN_FILE" || exit $?
        else
            "$repo_hook" "$@" || exit $?
        fi
    fi

    local user_hook="$HOME/.git-hooks/$hook_name"
    if [[ -x "$user_hook" ]]; then
        if [[ -n "${CHAIN_HOOK_STDIN_FILE:-}" ]]; then
            "$user_hook" "$@" <"$CHAIN_HOOK_STDIN_FILE" || exit $?
        else
            "$user_hook" "$@" || exit $?
        fi
    fi
}
