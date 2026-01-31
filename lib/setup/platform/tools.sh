#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Source utilities
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"
source "${DOTDOTFILES}/lib/setup/helpers/defaults.sh"

color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
color_echo BLUE "🔧  Installing Custom Tools..."
color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check if a tool is installed
is_tool_installed() {
    local tool="$1"
    local binary="$tool"
    
    # Mapping for tools where binary name != package name
    case "$tool" in
        "tree-sitter-cli") binary="tree-sitter" ;;
        "cloudflare-speed-cli") binary="cloudflare-speed-cli" ;;
        "async-cmd") binary="async" ;;
    esac
    
    command -v "$binary" &>/dev/null
}

for tool_script in "${DOTDOTFILES}/lib/setup/tools"/*.sh; do
    [[ -x "$tool_script" ]] || continue
    
    tool_name=$(basename "$tool_script" .sh)
    
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
