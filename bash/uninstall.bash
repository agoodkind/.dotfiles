#!/usr/bin/env bash
set -euo pipefail

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

. "$DOTDOTFILES/lib/dotfilesctl/bootstrap-go.sh"

run_dotfiles_go_command uninstall "$@"
