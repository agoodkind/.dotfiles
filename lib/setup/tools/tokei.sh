#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"

color_echo CYAN "  📦  Installing tokei from GitHub releases..."

repo="XAMPPRocky/tokei"
arch=$(uname -m)
os_type=$(uname -s | tr '[:upper:]' '[:lower:]')

# Find latest release with assets
release_data=$(curl -s "https://api.github.com/repos/$repo/releases" | jq -c '.[] | select(.assets | length > 0) | select(.prerelease == false)' | head -n 1)
if [[ -z "$release_data" ]]; then
    # Fallback to absolute latest if no stable releases with assets found
    release_data=$(curl -s "https://api.github.com/repos/$repo/releases" | jq -c '.[] | select(.assets | length > 0)' | head -n 1)
fi

tag=$(echo "$release_data" | jq -r .tag_name)

case "$os_type" in
    darwin)
        # Tokei often only provides x86_64 for Darwin which works on arm64
        filename=$(echo "$release_data" | jq -r '.assets[].name | select(contains("apple-darwin") and contains("x86_64"))' | head -n 1)
        ;;
    linux)
        case "$arch" in
            x86_64) filename=$(echo "$release_data" | jq -r '.assets[].name | select(contains("unknown-linux-gnu") and contains("x86_64"))' | head -n 1) ;;
            arm64|aarch64) filename=$(echo "$release_data" | jq -r '.assets[].name | select(contains("unknown-linux-gnu") and contains("aarch64"))' | head -n 1) ;;
        esac
        ;;
esac

if [[ -z "$filename" ]]; then
    color_echo RED "  ❌  Could not find a compatible binary for $os_type/$arch in tokei $tag"
    exit 1
fi

url="https://github.com/$repo/releases/download/$tag/$filename"

mkdir -p "$HOME/.cargo/bin"
color_echo YELLOW "  📥  Downloading $filename..."
curl -L "$url" | tar -xz -C "$HOME/.cargo/bin"
chmod +x "$HOME/.cargo/bin/tokei"

color_echo GREEN "  ✅  tokei $tag installed to ~/.cargo/bin"
