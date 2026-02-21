#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/bash/core/colors.bash"
source "${DOTDOTFILES}/bash/core/tools.bash"

get_system_info

case "$ARCH" in
    x86_64) arch_tag="x86_64" ;;
    arm64|aarch64) arch_tag="aarch64" ;;
esac

case "$OS_NAME" in
    macos) os_tag="apple-darwin" ;;
    linux) os_tag="unknown-linux-musl" ;;
esac

color_echo CYAN "  📦  Installing cloudflare-speed-cli..."
pattern="contains(\"$arch_tag\") and contains(\"$os_tag\") and endswith(\".tar.xz\")"
install_from_github "kavehtehrani/cloudflare-speed-cli" "$pattern" "cloudflare-speed-cli"
