#!/usr/bin/env bash

export DOTDOTFILES="$HOME/.dotfiles"

# Source color utilities
source "${DOTDOTFILES}/lib/include/colors.sh"

color_echo BLUE "🔧  Making scripts executable..."
chmod +x "$DOTDOTFILES/update.sh"
chmod +x "$DOTDOTFILES/repair.sh"
find "$DOTDOTFILES/lib/install" -name "*.sh" -type f -exec chmod +x {} \;
color_echo GREEN "  ✅  All install scripts are now executable"

color_echo BLUE "📁  Creating SSH sockets directory..."
mkdir -p "$HOME/.ssh/sockets"

color_echo BLUE "🗝️  Starting SSH agent..."
eval "$(ssh-agent -s)"

color_echo BLUE "➕  Adding SSH keys..."
# Auto-add SSH key if not already in agent
ssh-add -l  || true
ssh-add ~/.ssh/id_ed25519 || true

color_echo BLUE "🔧  Setting up git configuration..."
"$DOTDOTFILES/lib/install/git.sh"

color_echo BLUE "🛠️  Running repair script..."
"$DOTDOTFILES/update.sh"

# Set up passwordless sudo for current user (macOS and Ubuntu)
if [[ "$OSTYPE" == "darwin"* ]]; then
    SUDOERS_FILE="/private/etc/sudoers.d/$(whoami)"
else
    SUDOERS_FILE="/etc/sudoers.d/$(whoami)"
fi
# ask user if they want to configure passwordless sudo
read -p "Configure passwordless sudo? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    if [[ -f "$SUDOERS_FILE" ]]; then
        color_echo GREEN "🔓  Sudo already passwordless for $(whoami)"
    else
        color_echo YELLOW "🔓  Configuring passwordless sudo for $(whoami)"
        SUDOERS_LINE="$(whoami) ALL=(ALL) NOPASSWD:ALL"
        echo "$SUDOERS_LINE" | sudo tee "$SUDOERS_FILE" >/dev/null
        sudo chmod 0440 "$SUDOERS_FILE"
        color_echo GREEN "✅  Passwordless sudo configured for $(whoami)"
    fi
fi

chsh -s "$(which zsh)"

color_echo GREEN "✅  Installation complete!"

