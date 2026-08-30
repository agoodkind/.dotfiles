#!/usr/bin/env bash
set -euo pipefail

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
export DOTDOTFILES="$(cd "$HOOK_DIR/.." && pwd)"
source "$DOTDOTFILES/dots/bootstrap-go.sh"
run_dots_go_command git-hook "$@"
