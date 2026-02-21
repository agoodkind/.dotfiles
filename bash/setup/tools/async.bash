#!/usr/bin/env bash
# package: async-cmd
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/bash/core/colors.bash"

if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    color_echo YELLOW "  ⏭️  Skipping async-cmd compilation in CI"
    exit 0
fi

color_echo CYAN "  📦  Installing async-cmd via cargo..."
cargo install async-cmd
