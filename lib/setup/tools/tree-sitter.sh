#!/usr/bin/env bash
# package: tree-sitter-cli
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"
source "${DOTDOTFILES}/lib/setup/helpers/tools.sh"

get_system_info

# Map architecture/OS for tree-sitter naming
case "$ARCH" in
    x86_64) arch_tag="x64" ;;
    arm64|aarch64) arch_tag="arm64" ;;
esac

case "$OS_NAME" in
    macos) os_tag="macos" ;;
    linux) os_tag="linux" ;;
esac

color_echo CYAN "  📦  Installing tree-sitter-cli..."
install_from_github "tree-sitter/tree-sitter" "contains(\"$os_tag\") and contains(\"$arch_tag\") and endswith(\".gz\")" "tree-sitter"
