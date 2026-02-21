#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Source utilities
source "${DOTDOTFILES}/bash/core/colors.bash"
source "${DOTDOTFILES}/bash/core/defaults.bash"

color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
color_echo BLUE "🔧  Installing Custom Tools..."
color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check if a tool is installed
is_tool_installed() {
    local binary="$1"
    command -v "$binary" &>/dev/null
}

for tool_script in "${DOTDOTFILES}/bash/setup/tools"/*.bash; do
    [[ -x "$tool_script" ]] || continue
    
    tool_name=$(basename "$tool_script" .bash)
    
    if is_tool_installed "$tool_name"; then
        color_echo GREEN "  ✅  $tool_name is already installed"
        continue
    fi
    
    color_echo YELLOW "  📦  Installing $tool_name..."
    if "$tool_script"; then
        color_echo GREEN "  ✅  $tool_name installed successfully"
    else
        color_echo RED "  ❌  Failed to install $tool_name"
    fi
done

color_echo GREEN "Custom tools installation complete!"
