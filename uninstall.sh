#!/usr/bin/env bash
set -e
set -u
set -o pipefail

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

source "$DOTDOTFILES/dots/bootstrap-go.sh"
bootstrap_and_run uninstall "$@"
