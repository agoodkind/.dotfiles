#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"

color_echo CYAN "  📦  Installing xh via official installer..."
# xh installer supports --bin-dir and --no-modify-path to be non-interactive
curl -sfL https://raw.githubusercontent.com/ducaale/xh/master/install.sh | sh -s -- --bin-dir "$HOME/.cargo/bin" --no-modify-path
