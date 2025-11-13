#!/usr/bin/env bash

set -euo pipefail

# Color and emoji setup
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

color_echo() {
    color="$1"; shift
    echo -e "${!color}$*${NC}"
}

export DOTDOTFILES="$HOME/.dotfiles"

color_echo BLUE "🔧  Making scripts executable..."
chmod +x "$DOTDOTFILES/repair.sh"
chmod +x "$DOTDOTFILES/lib/install/git.sh"
chmod +x "$DOTDOTFILES/lib/install/apt.sh"
chmod +x "$DOTDOTFILES/lib/install/mac.sh"
chmod +x "$DOTDOTFILES/lib/install/brew.sh"

color_echo BLUE "📁  Creating SSH sockets directory..."
mkdir -p "$HOME/.ssh/sockets"

color_echo BLUE "🗝️  Starting SSH agent..."
eval $(ssh-agent -s)

color_echo BLUE "➕  Adding SSH keys..."
# Auto-add SSH key if not already in agent
ssh-add -l  || true
ssh-add ~/.ssh/id_ed25519 || true

color_echo BLUE "🔧  Setting up git configuration..."
"$DOTDOTFILES/lib/install/git.sh"

color_echo BLUE "🛠️  Running repair script..."
"$DOTDOTFILES/repair.sh"

# Set up passwordless sudo for current user (macOS and Ubuntu)
if [[ "$OSTYPE" == "darwin"* ]]; then
    SUDOERS_FILE="/private/etc/sudoers.d/$(whoami)"
else
    SUDOERS_FILE="/etc/sudoers.d/$(whoami)"
fi

if [[ -f "$SUDOERS_FILE" ]]; then
    color_echo GREEN "🔓  Sudo already passwordless for $(whoami)"
else
    color_echo YELLOW "🔓  Configuring passwordless sudo for $(whoami)"
    SUDOERS_LINE="$(whoami) ALL=(ALL) NOPASSWD:ALL"
    echo "$SUDOERS_LINE" | sudo tee "$SUDOERS_FILE" >/dev/null
    sudo chmod 0440 "$SUDOERS_FILE"
    color_echo GREEN "✅  Passwordless sudo configured for $(whoami)"
fi

# if Ubuntu or Debian, install apt
if [[ -f /etc/os-release ]] && grep -qiE 'ubuntu|debian' /etc/os-release; then
    color_echo YELLOW "🐧  Ubuntu/Debian detected"
    color_echo YELLOW "📦  Installing apt packages..."
    "$DOTDOTFILES/lib/install/apt.sh"
fi

# run mac last because it calls brew which takes forever
# if mac, install brew
if [[ "$OSTYPE" == "darwin"* ]]; then
    color_echo YELLOW "🍏  macOS detected"
    color_echo YELLOW "🍺  Installing mac packages..."
    "$DOTDOTFILES/lib/install/mac.sh"
fi

color_echo GREEN "✅  Installation complete!"

