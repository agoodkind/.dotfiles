#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/bash/core/colors.bash"

color_echo CYAN "  📦  Installing zoxide via official installer..."
curl -sSfL \
    https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh \
    | sh
