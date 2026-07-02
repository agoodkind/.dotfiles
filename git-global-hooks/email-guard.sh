#!/usr/bin/env bash
# Resolve allowed email and name for repo at $PWD

resolve_allowed_identity() {
    local configured_email configured_name
    configured_email="$(git config --get goodkind.allowedEmail 2>/dev/null || true)"
    configured_name="$(git config --get goodkind.allowedName 2>/dev/null || true)"
    if [[ -n "$configured_email" && -n "$configured_name" ]]; then
        echo "$configured_email"
        echo "$configured_name"
        return
    fi

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
            read -r prefix email name <<<"$line"
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
        done <"$f"
    done

    echo "$match_email"
    echo "$match_name"
}

resolve_allowed_identity_fields() {
    local identity
    identity="$(resolve_allowed_identity)"
    if [[ -z "$identity" ]]; then
        return 1
    fi

    RESOLVED_ALLOWED_EMAIL="${identity%%$'\n'*}"
    if [[ "$identity" == *$'\n'* ]]; then
        RESOLVED_ALLOWED_NAME="${identity#*$'\n'}"
    else
        RESOLVED_ALLOWED_NAME=""
    fi

    return 0
}

default_allowed_authors() {
    printf '%s\n' \
        'Codex <codex@openai.com>' \
        'Claude <noreply@anthropic.com>' \
        'Cursor <cursoragent@cursor.com>' \
        'Cursor Agent <cursoragent@cursor.com>'
}

resolve_allowed_authors() {
    if resolve_allowed_identity_fields; then
        printf '%s <%s>\n' "$RESOLVED_ALLOWED_NAME" "$RESOLVED_ALLOWED_EMAIL"
    fi

    # Keep the committer human-scoped, but allow well-known AI authors on
    # pushed commits so signed human commits can carry explicit agent authorship.
    default_allowed_authors
    git config --get-all goodkind.allowedAuthor 2>/dev/null || true
}

author_is_allowed() {
    local current_author="$1"
    local allowed_authors="$2"
    local allowed_author

    while IFS= read -r allowed_author; do
        if [[ -z "$allowed_author" ]]; then
            continue
        fi
        if [[ "$current_author" == "$allowed_author" ]]; then
            return 0
        fi
    done <<<"$allowed_authors"

    return 1
}
