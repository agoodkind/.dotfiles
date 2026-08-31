#!/usr/bin/env bash
set -euo pipefail

source "$(dirname "$0")/protected-branch-policy.sh"

if ! protected_branch_policy_applies; then
    exit 0
fi

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
export DOTDOTFILES="$(cd "$HOOK_DIR/.." && pwd)"
source "$DOTDOTFILES/dots/bootstrap-go.sh"
run_dots_go_command git-hook "$@"
