#!/usr/bin/env bash
set -e
set -u
set -o pipefail

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

. "$DOTDOTFILES/lib/dotfilesctl/bootstrap-go.sh"
bootstrap_and_run install "$@"
