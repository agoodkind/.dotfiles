#!/usr/bin/env bash

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

cd "$DOTDOTFILES" || echo "Failed to cd to $DOTDOTFILES" && exit 1

# Source utilities
source "${DOTDOTFILES}/lib/include/defaults.sh"
source "${DOTDOTFILES}/lib/include/colors.sh"
source "${DOTDOTFILES}/lib/include/packages.sh"

color_echo BLUE "📁  Creating SSH sockets directory..."
mkdir -p "$HOME/.ssh/sockets"

color_echo BLUE "🗝️  Starting SSH agent..."
eval "$(ssh-agent -s)"

color_echo BLUE "➕  Adding SSH keys..."
# Auto-add SSH key if not already in agent
ssh-add -l  || true
ssh-add ~/.ssh/id_ed25519 || true

color_echo BLUE "🔧  Setting up git configuration..."
run_with_defaults "$DOTDOTFILES/lib/install/git.sh"

color_echo BLUE "🛠️  Running sync script..."
run_with_defaults "$DOTDOTFILES/sync.sh"

# Set up passwordless sudo for current user (macOS and Ubuntu)
if [[ "$OSTYPE" == "darwin"* ]]; then
    SUDOERS_FILE="/private/etc/sudoers.d/$(whoami)"
else
    SUDOERS_FILE="/etc/sudoers.d/$(whoami)"
fi

# ask user if they want to configure passwordless sudo
read_with_default "Configure passwordless sudo? (y/n) " "n"
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

color_echo BLUE "🔧  Changing shell to zsh..."
ZSH_PATH=$(which zsh)
if [[ "$OSTYPE" == "darwin"* ]]; then
    CURRENT_SHELL=$(dscl . -read "/Users/$(whoami)" UserShell 2>/dev/null | cut -d' ' -f2 || echo "$SHELL")
else
    CURRENT_SHELL=$(getent passwd "$(whoami)" 2>/dev/null | cut -d: -f7 || echo "$SHELL")
fi

# Check if current shell is zsh (by path match or by basename)
if [[ "$CURRENT_SHELL" == "$ZSH_PATH" ]] || [[ "$(basename "$CURRENT_SHELL")" == "zsh" ]]; then
    color_echo GREEN "  ✅  Shell is already zsh"
else
    chsh -s "$ZSH_PATH"
    color_echo GREEN "  ✅  Shell changed to zsh, you may need to restart your terminal or log out and back in to use it"
fi

# force zinit to update and install
color_echo BLUE "🔧  Initializing zinit..."
(cd "$HOME" && \
 PAGER=cat GIT_PAGER=cat \
 source "$DOTDOTFILES/lib/zinit/zinit-install.zsh" && \
 zinit self-update && \
 zinit update) || color_echo RED "Failed to initialize zinit" && exit 1

color_echo GREEN "✅  Installation complete!"

