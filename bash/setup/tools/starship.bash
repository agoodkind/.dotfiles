#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/bash/core/colors.bash"

color_echo CYAN "  📦  Installing starship via official installer..."
curl -sS https://starship.rs/install.sh | sh -s -- --yes
