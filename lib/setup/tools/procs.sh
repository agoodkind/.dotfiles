#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"

color_echo CYAN "  📦  Installing procs from GitHub releases..."

repo="dalance/procs"
arch=$(uname -m)
os_type=$(uname -s | tr '[:upper:]' '[:lower:]')

# Map OS/Arch for procs naming convention
case "$os_type" in
    darwin) os="mac" ;;
    linux) os="linux" ;;
esac

case "$arch" in
    x86_64) arch="x86_64" ;;
    arm64|aarch64) arch="aarch64" ;;
esac

tag=$(curl -s "https://api.github.com/repos/$repo/releases/latest" | jq -r .tag_name)
filename="procs-$tag-$arch-$os.zip"
url="https://github.com/$repo/releases/download/$tag/$filename"

mkdir -p "$HOME/.cargo/bin"
curl -L "$url" -o "/tmp/procs.zip"
unzip -o "/tmp/procs.zip" -d "/tmp/procs-extract"
mv "/tmp/procs-extract/procs" "$HOME/.cargo/bin/"
chmod +x "$HOME/.cargo/bin/procs"
rm -rf "/tmp/procs.zip" "/tmp/procs-extract"

color_echo GREEN "  ✅  procs installed to ~/.cargo/bin"
