#!/usr/bin/env bash
# Resolve allowed email and name for repo at $PWD

resolve_allowed_identity() {
    local rules_file="${GIT_EMAIL_RULES:-}"
    local repo_path
    repo_path="$(git rev-parse --show-toplevel \
        2>/dev/null || pwd)"

    if [[ -z "$rules_file" ]]; then
        if [[ -f "$HOME/.config/git/email-rules" ]]; then
            rules_file="$HOME/.config/git/email-rules"
        else
            rules_file="$(dirname "$0")/email-rules"
        fi
    fi

    if [[ ! -f "$rules_file" ]]; then
        return
    fi

    local local_rules_file="${rules_file}.local"
    local match_email="" match_name=""

    local f
    for f in "$rules_file" "$local_rules_file"; do
        if [[ ! -f "$f" ]]; then
            continue
        fi
        while IFS= read -r line; do
            line="${line%%#*}"
            if [[ -z "${line// /}" ]]; then
                continue
            fi
            local prefix email name
            read -r prefix email name <<< "$line"
            prefix="${prefix/#\~/$HOME}"
            if [[ "$prefix" == "*" ]]; then
                if [[ -z "$match_email" ]]; then
                    match_email="$email"
                    match_name="$name"
                fi
            elif [[ "$repo_path" == "$prefix"* ]]; then
                match_email="$email"
                match_name="$name"
            fi
        done < "$f"
    done

    echo "$match_email"
    echo "$match_name"
}
