#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

echo "Installing dotfiles to $DOTDOTFILES"
cd "$DOTDOTFILES" || { echo "Failed to cd to $DOTDOTFILES" && exit 1; }

# Source utilities
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"
color_echo BLUE "📁  Sourcing utilities..."
source "${DOTDOTFILES}/lib/setup/helpers/defaults.sh"
source "${DOTDOTFILES}/lib/setup/helpers/packages.sh"

color_echo BLUE "📁  Creating SSH sockets directory..."
mkdir -p "$HOME/.ssh/sockets"

color_echo BLUE "🗝️  Starting SSH agent..."
eval "$(ssh-agent -s)"

color_echo BLUE "🔧  Setting up git configuration..."
run_with_defaults "$DOTDOTFILES/lib/setup/platform/git.sh"

color_echo BLUE "🛠️  Running sync script..."
run_with_defaults "$DOTDOTFILES/sync.sh" "$@"

# Set up passwordless sudo for current user (macOS and Ubuntu)
if [[ "$OSTYPE" == "darwin"* ]]; then
    SUDOERS_FILE="/private/etc/sudoers.d/$(whoami)"
else
    SUDOERS_FILE="/etc/sudoers.d/$(whoami)"
fi

# ask user if they want to configure passwordless sudo (current user only)
CURRENT_USER=$(whoami)
color_echo YELLOW "🔓  This will configure passwordless sudo for user '$CURRENT_USER' only"
read_with_default "Configure passwordless sudo for $CURRENT_USER? (y/n) " "n"
if [[ $REPLY =~ ^[Yy]$ ]]; then
    if [[ -f "$SUDOERS_FILE" ]]; then
        color_echo GREEN "🔓  Sudo already passwordless for $CURRENT_USER"
    else
        color_echo YELLOW "🔓  Configuring passwordless sudo for $CURRENT_USER..."
        # Validate username doesn't contain dangerous characters
        if [[ "$CURRENT_USER" =~ [^a-zA-Z0-9_-] ]]; then
            color_echo RED "❌  Username contains invalid characters, skipping"
        else
            SUDOERS_LINE="$CURRENT_USER ALL=(ALL) NOPASSWD:ALL"
            echo "$SUDOERS_LINE" | sudo tee "$SUDOERS_FILE" >/dev/null
            sudo chmod 0440 "$SUDOERS_FILE"
            # Verify the file is valid
            if sudo visudo -c -f "$SUDOERS_FILE" >/dev/null 2>&1; then
                color_echo GREEN "✅  Passwordless sudo configured for $CURRENT_USER"
            else
                color_echo RED "❌  Invalid sudoers file, removing..."
                sudo rm -f "$SUDOERS_FILE"
            fi
        fi
    fi
fi

color_echo BLUE "🔧  Checking login shell..."
ZSH_PATH=$(command -v zsh)

if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS: use dscl to get login shell, extract path after "UserShell: "
    CURRENT_SHELL=$(dscl . -read "/Users/$(whoami)" UserShell 2>/dev/null | awk '{print $2}')
else
    # Linux: use getent to get login shell from passwd
    CURRENT_SHELL=$(getent passwd "$(whoami)" 2>/dev/null | cut -d: -f7)
fi

# Fallback if detection failed
if [[ -z "$CURRENT_SHELL" ]]; then
    CURRENT_SHELL="unknown"
fi

color_echo YELLOW "  📍  Current login shell: $CURRENT_SHELL"
color_echo YELLOW "  📍  Target zsh path: $ZSH_PATH"

# Check if current shell is zsh (by path match or by basename)
if [[ "$CURRENT_SHELL" == "$ZSH_PATH" ]] || [[ "$(basename "$CURRENT_SHELL" 2>/dev/null)" == "zsh" ]]; then
    color_echo GREEN "  ✅  Shell is already zsh"
    color_echo GREEN "  ✅  Installation complete!"
else
    color_echo YELLOW "  🔄  Changing login shell to zsh..."
    if chsh -s "$ZSH_PATH"; then
        color_echo GREEN "  ✅  Login shell changed to zsh"
        color_echo YELLOW "  💡  Log out and back in, or run: exec zsh"
    else
        color_echo RED "  ❌  Failed to change shell (may need sudo)"
    fi
fi
