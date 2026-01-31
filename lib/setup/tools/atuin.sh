#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"

color_echo CYAN "Installing atuin via official installer..."
curl --proto '=https' --tlsv1.2 -sSf https://setup.atuin.sh | sh -s -- --yes
