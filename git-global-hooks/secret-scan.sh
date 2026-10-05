#!/usr/bin/env bash
# Scans staged changes with ggshield and gitleaks.
# Usage: secret_scan "$@"

secret_scan() {
    if command -v ggshield &>/dev/null; then
        if ggshield api-status --no-check-for-updates >/dev/null 2>&1; then
            ggshield secret scan pre-commit --no-check-for-updates "$@" || return 1
        else
            echo "warning: ggshield is not authenticated; GitGuardian scan skipped" >&2
            echo "hint: run 'ggshield auth login' to finish GitGuardian setup" >&2
        fi
    else
        echo "warning: ggshield not found; GitGuardian scan skipped (install: brew install ggshield)" >&2
    fi

    if ! command -v gitleaks &>/dev/null; then
        echo "warning: gitleaks not found; secret scan skipped (install: brew install gitleaks)" >&2
        return 0
    fi

    # A repository .gitleaks.toml replaces the dotfiles config for that repository.
    local repo_root gitleaks_config
    repo_root="$(git rev-parse --show-toplevel)" || return 1
    gitleaks_config="$HOME/.dotfiles/.gitleaks.toml"
    if [[ -f "$repo_root/.gitleaks.toml" ]]; then
        gitleaks_config="$repo_root/.gitleaks.toml"
    fi
    if [[ -f "$gitleaks_config" ]]; then
        gitleaks protect --staged --no-banner --redact --config "$gitleaks_config" || return 1
    else
        gitleaks protect --staged --no-banner --redact || return 1
    fi
}
