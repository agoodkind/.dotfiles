#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"
source "${DOTDOTFILES}/lib/setup/helpers/tools.sh"

get_system_info

case "$ARCH" in
    x86_64) arch_tag="x86_64" ;;
    arm64|aarch64) arch_tag="aarch64" ;;
esac

case "$OS_NAME" in
    macos) os_tag="apple-darwin" ;;
    linux) os_tag="unknown-linux-musl" ;;
esac

color_echo CYAN "  📦  Installing xh..."
install_from_github "ducaale/xh" "contains(\"$arch_tag\") and contains(\"$os_tag\") and endswith(\".tar.gz\")" "xh"
