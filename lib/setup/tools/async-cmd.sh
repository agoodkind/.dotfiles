#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$(cd "$(dirname "$0")/../../.." && pwd)}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"

if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    color_echo YELLOW "⏭️  Skipping async-cmd compilation in CI"
    exit 0
fi

color_echo CYAN "Installing async-cmd via cargo..."
cargo install async-cmd
