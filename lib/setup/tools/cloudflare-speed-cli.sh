#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$(cd "$(dirname "$0")/../../.." && pwd)}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"

color_echo CYAN "Installing cloudflare-speed-cli from GitHub releases..."
repo="kavehtehrani/cloudflare-speed-cli"

case "$(uname -m)" in
    x86_64) arch="x86_64" ;;
    arm64|aarch64) arch="aarch64" ;;
    *) color_echo RED "Unsupported architecture for cloudflare-speed-cli binary"; exit 1 ;;
esac

os="unknown-linux-musl"
if [[ "$OSTYPE" == "darwin"* ]]; then
    os="apple-darwin"
fi

version=$(curl -s "https://api.github.com/repos/$repo/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
filename="cloudflare-speed-cli-$arch-$os.tar.xz"
url="https://github.com/kavehtehrani/cloudflare-speed-cli/releases/download/$version/$filename"

curl -L "$url" -o "/tmp/$filename"
mkdir -p "/tmp/cloudflare-speed-cli"
tar -xf "/tmp/$filename" -C "/tmp/cloudflare-speed-cli"

bin_name="cloudflare-speed-cli"
bin_path=$(find "/tmp/cloudflare-speed-cli" -name "$bin_name" -type f | head -n 1)

if [[ -x "$bin_path" ]]; then
    mkdir -p "$HOME/.cargo/bin"
    cp "$bin_path" "$HOME/.cargo/bin/"
    color_echo GREEN "cloudflare-speed-cli installed to ~/.cargo/bin"
else
    color_echo RED "Failed to find binary in cloudflare-speed-cli archive"
    exit 1
fi
