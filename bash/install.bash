#!/usr/bin/env bash
set -euo pipefail

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

source "$DOTDOTFILES/lib/dotfilesctl/bootstrap-go.sh"

run_dotfiles_go_command install "$@"
