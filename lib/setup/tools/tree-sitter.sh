#!/usr/bin/env bash
# package: tree-sitter-cli
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$(cd "$(dirname "$0")/../../.." && pwd)}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"

if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    color_echo YELLOW "  ⏭️  Skipping tree-sitter compilation in CI"
    exit 0
fi

color_echo CYAN "  📦  Installing tree-sitter via cargo-binstall..."
if command -v cargo-binstall &>/dev/null; then
    cargo binstall -y tree-sitter-cli
else
    cargo install tree-sitter-cli
fi
