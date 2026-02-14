#!/usr/bin/env bash
# Resolve allowed email for repo at $PWD

resolve_allowed_email() {
    local rules_file="${GIT_EMAIL_RULES:-}"
    local repo_path
    repo_path="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

    if [[ -z "$rules_file" ]]; then
        if [[ -f "$HOME/.config/git/email-rules" ]]; then
            rules_file="$HOME/.config/git/email-rules"
        else
            rules_file="$(dirname "$0")/email-rules"
        fi
    fi

    [[ -f "$rules_file" ]] || { echo ""; return; }

    local match=""
    while IFS= read -r line; do
        line="${line%%#*}"
        [[ -z "${line// /}" ]] && continue
        local prefix email
        read -r prefix email <<< "$line"
        prefix="${prefix/#\~/$HOME}"
        if [[ "$prefix" == "*" ]]; then
            [[ -z "$match" ]] && match="$email"
        elif [[ "$repo_path" == "$prefix"* ]]; then
            match="$email"
        fi
    done < "$rules_file"

    echo "$match"
}
