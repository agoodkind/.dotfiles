#!/usr/bin/env bash
set -e
set -u
set -o pipefail

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Pull in shell before the binary runs so a broken binary can never block its
# own update. Skipped when --quick is passed (background dispatch path).
_skip_git=0
for _arg in "$@"; do
    if [ "$_arg" = "--quick" ]; then
        _skip_git=1
        break
    fi
done

if [ "$_skip_git" -eq 0 ]; then
    git -C "$DOTDOTFILES" fetch origin --prune 2>/dev/null || true
    git -C "$DOTDOTFILES" pull --ff-only origin main 2>/dev/null || true
fi

source "$DOTDOTFILES/dots/bootstrap-go.sh"
bootstrap_and_run sync "$@"
