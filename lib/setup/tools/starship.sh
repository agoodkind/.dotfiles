#!/usr/bin/env bash
set -e
set -o pipefail

# Sourcing from DOTDOTFILES or calculating relative path
export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"

color_echo CYAN "Installing starship via official installer..."
curl -sS https://starship.rs/install.sh | sh -s -- --yes
