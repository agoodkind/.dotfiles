#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$(cd "$(dirname "$0")/../../.." && pwd)}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"

color_echo CYAN "  📦  Installing tokei from GitHub releases..."

repo="XAMPPRocky/tokei"
arch=$(uname -m)
os_type=$(uname -s | tr '[:upper:]' '[:lower:]')

# Tokei has a bit of a mixed naming convention, using stable v12.1.2
tag="v12.1.2"

case "$os_type" in
    darwin)
        # Use x86_64 for mac as it works on arm64 via Rosetta and is more available
        filename="tokei-x86_64-apple-darwin.tar.gz"
        ;;
    linux)
        case "$arch" in
            x86_64) filename="tokei-x86_64-unknown-linux-gnu.tar.gz" ;;
            arm64|aarch64) filename="tokei-aarch64-unknown-linux-gnu.tar.gz" ;;
        esac
        ;;
esac

url="https://github.com/$repo/releases/download/$tag/$filename"

mkdir -p "$HOME/.cargo/bin"
curl -L "$url" | tar -xz -C "$HOME/.cargo/bin"
chmod +x "$HOME/.cargo/bin/tokei"

color_echo GREEN "  ✅  tokei installed to ~/.cargo/bin"
