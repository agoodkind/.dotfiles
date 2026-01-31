#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$(cd "$(dirname "$0")/../../.." && pwd)}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"

if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    color_echo YELLOW "  ⏭️  Skipping tokei compilation in CI"
    exit 0
fi

color_echo CYAN "  📦  Installing tokei via cargo-binstall..."
if command -v cargo-binstall &>/dev/null; then
    cargo binstall -y tokei
else
    cargo install tokei
fi
