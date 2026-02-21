#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/bash/core/colors.bash"
source "${DOTDOTFILES}/bash/core/tools.bash"

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
pattern="contains(\"$os_tag\") and contains(\"$arch_tag\") and endswith(\".zip\")"
install_from_github "dalance/procs" "$pattern" "procs"
