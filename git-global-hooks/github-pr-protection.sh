#!/usr/bin/env bash

set -euo pipefail

fail() {
    printf 'error: GitHub pull request protection lookup failed: %s\n' "$1" >&2
    exit 1
}

main() {
    local directory_only=false
    local target_path
    local origin_url
    local repository
    local encoded_branch
    local protected
    local page_result
    local page_results
    local remote_names

    if [[ "${1:-}" == --dir-only ]]; then
        directory_only=true
        shift
    fi
    if [[ "$#" -ne 1 || -z "$1" ]]; then
        fail 'provide one file or directory path, optionally after --dir-only.'
    fi
    target_path="$1"
    if [[ ! -d "$target_path" ]]; then
        if [[ "$directory_only" == true ]]; then
            printf '{"pr_protected":false}\n'
            return
        fi
        while [[ ! -d "$target_path" ]]; do
            target_path="$(dirname "$target_path")"
        done
    fi
    if ! remote_names="$(git -C "$target_path" remote)"; then
        fail "cannot read Git remotes for $target_path"
    fi
    if [[ $'\n'"$remote_names"$'\n' != *$'\norigin\n'* ]]; then
        printf '{"pr_protected":false}\n'
        return
    fi
    if ! origin_url="$(git -C "$target_path" remote get-url origin)"; then
        fail "cannot read the origin URL for $target_path"
    fi
    case "$origin_url" in
        https://github.com/* | http://github.com/*)
            repository="${origin_url#*://github.com/}"
            ;;
        git@github.com:*)
            repository="${origin_url#git@github.com:}"
            ;;
        ssh://git@github.com/*)
            repository="${origin_url#ssh://git@github.com/}"
            ;;
        *)
            printf '{"pr_protected":false}\n'
            return
            ;;
    esac
    repository="${repository%/}"
    repository="${repository%.git}"
    if [[ ! "$repository" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
        fail "origin does not specify a valid GitHub owner and repository: $origin_url"
    fi
    if ! encoded_branch="$(gh repo view "github.com/$repository" --json defaultBranchRef --jq '.defaultBranchRef.name | select(type == "string" and length > 0) | @uri')"; then
        fail "cannot read the default branch of $repository"
    fi
    if [[ -z "$encoded_branch" ]]; then
        fail "GitHub did not return a default branch for $repository"
    fi
    if ! page_results="$(gh api --hostname github.com "repos/$repository/rules/branches/$encoded_branch" --paginate --jq 'any(.[]; .type == "pull_request")')"; then
        fail "cannot read active rules for the default branch of $repository"
    fi
    protected=false
    while IFS= read -r page_result; do
        case "$page_result" in
            true)
                protected=true
                ;;
            false) ;;
            *)
                fail "GitHub returned an invalid protection result for $repository"
                ;;
        esac
    done <<<"$page_results"
    printf '{"pr_protected":%s}\n' "$protected"
}

main "$@"
