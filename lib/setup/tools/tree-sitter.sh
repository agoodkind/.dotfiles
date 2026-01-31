#!/usr/bin/env bash
# package: tree-sitter-cli
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$(cd "$(dirname "$0")/../../.." && pwd)}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"

color_echo CYAN "  📦  Installing tree-sitter-cli from GitHub releases..."

repo="tree-sitter/tree-sitter"
arch=$(uname -m)
os_type=$(uname -s | tr '[:upper:]' '[:lower:]')

# Map architecture
case "$arch" in
    x86_64) arch="x64" ;;
    arm64|aarch64) arch="arm64" ;;
esac

# Map OS
case "$os_type" in
    darwin) os="macos" ;;
    linux) os="linux" ;;
esac

tag=$(curl -s "https://api.github.com/repos/$repo/releases/latest" | jq -r .tag_name)
filename="tree-sitter-$os-$arch.gz"
url="https://github.com/$repo/releases/download/$tag/$filename"

mkdir -p "$HOME/.cargo/bin"
curl -L "$url" | gunzip -c > "$HOME/.cargo/bin/tree-sitter"
chmod +x "$HOME/.cargo/bin/tree-sitter"

color_echo GREEN "  ✅  tree-sitter installed to ~/.cargo/bin"
