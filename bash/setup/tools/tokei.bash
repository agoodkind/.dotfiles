#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/bash/core/colors.bash"
source "${DOTDOTFILES}/bash/core/tools.bash"

get_system_info

# Tokei often only provides x86_64 for Darwin which works on arm64
case "$OS_NAME" in
    macos) 
        pattern="contains(\"apple-darwin\") and contains(\"x86_64\")" 
        ;;
    linux)
        case "$ARCH" in
            x86_64) 
                pattern="contains(\"unknown-linux-gnu\") and contains(\"x86_64\")" 
                ;;
            arm64|aarch64) 
                pattern="contains(\"unknown-linux-gnu\") and contains(\"aarch64\")" 
                ;;
        esac
        ;;
esac

color_echo CYAN "  📦  Installing tokei..."
install_from_github "XAMPPRocky/tokei" "$pattern" "tokei"
