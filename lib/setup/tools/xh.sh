#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"

color_echo CYAN "Installing xh via official installer..."
curl -sfL https://raw.githubusercontent.com/ducaale/xh/master/install.sh | sh
