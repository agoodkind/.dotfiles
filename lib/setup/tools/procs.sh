#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"
source "${DOTDOTFILES}/lib/setup/helpers/tools.sh"

get_system_info

# Map OS/Arch for procs naming convention
case "$OS_NAME" in
    macos) os_tag="mac" ;;
    linux) os_tag="linux" ;;
esac

case "$ARCH" in
    x86_64) arch_tag="x86_64" ;;
    arm64|aarch64) arch_tag="aarch64" ;;
esac

color_echo CYAN "  📦  Installing procs..."
install_from_github "dalance/procs" "contains(\"$os_tag\") and contains(\"$arch_tag\") and endswith(\".zip\")" "procs" "unzip -o"
