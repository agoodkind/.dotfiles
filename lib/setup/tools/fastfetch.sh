#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/lib/setup/helpers/tools.sh"

linux_only "fastfetch (deb) is Linux-only, skipping..."

color_echo CYAN "  📦  Installing fastfetch from GitHub release..."
FASTFETCH_URL="https://api.github.com/repos/fastfetch-cli/fastfetch/releases/latest"
FASTFETCH_DEB_URL=$(curl -s "$FASTFETCH_URL" \
    | grep "browser_download_url.*linux-amd64.deb" \
    | cut -d : -f 2,3 \
    | tr -d \" \
    | tr -d ' ')
if [[ -n "$FASTFETCH_DEB_URL" ]]; then
    curl -fsSL "$FASTFETCH_DEB_URL" -o /tmp/fastfetch-linux-amd64.deb
    sudo dpkg -i /tmp/fastfetch-linux-amd64.deb
    rm -f /tmp/fastfetch-linux-amd64.deb
else
    color_echo RED "  ❌  Failed to get fastfetch download URL"
    exit 1
fi
