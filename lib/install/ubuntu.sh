#!/usr/bin/env bash

# Source utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../include/defaults.sh"

# Apt-based installation script for Ubuntu systems
# if not skip install is set, skip installation of packages, --skip-install
if [[ " $* " != *" --skip-install "* ]]; then
    # Check if timezone is already configured
    # A timezone is considered configured if /etc/localtime is a symlink (not a regular file)
    # or if /etc/timezone exists and contains a non-empty value
    TZ_CONFIGURED=false
    if [[ -L /etc/localtime ]]; then
        # /etc/localtime is a symlink, timezone has been configured
        TZ_CONFIGURED=true
    elif [[ -f /etc/timezone ]]; then
        TZ_VALUE=$(cat /etc/timezone 2>/dev/null | tr -d '\n' | tr -d '[:space:]' || echo "")
        if [[ -n "$TZ_VALUE" ]]; then
            TZ_CONFIGURED=true
        fi
    fi
    
    # Only reconfigure timezone if not already configured
    if [[ "$TZ_CONFIGURED" == "false" ]]; then
        sudo dpkg-reconfigure tzdata
    else
        CURRENT_TZ=$(timedatectl show -p Timezone --value 2>/dev/null || cat /etc/timezone 2>/dev/null | tr -d '\n' || echo "unknown")
        echo "Timezone already configured: $CURRENT_TZ (skipping reconfiguration)"
    fi
    
    run_with_defaults "$DOTDOTFILES/lib/install/apt.sh"
fi

# Script to configure MOTD (Message of the Day) on Ubuntu systems
# Disables default MOTD scripts and optionally sets a custom MOTD

DOTFILES_DIR="${DOTFILES_DIR:-$HOME/.dotfiles}"

echo "Configuring MOTD..."

# Disable all default MOTD scripts except ours
if [ -d /etc/update-motd.d/ ]; then
    echo "Disabling default MOTD scripts in /etc/update-motd.d/"
    for file in /etc/update-motd.d/*; do
        basename_file=$(basename "$file")
        if [[ "$basename_file" != "00-motd-entrypoint.sh" ]]; then
            sudo chmod -x "$file"
        fi
    done
fi


# Disable motd-news (Ubuntu's dynamic MOTD messages)
if [ -f /etc/default/motd-news ]; then
    echo "Disabling motd-news..."
    sudo sed -i 's/^ENABLED=.*/ENABLED=0/' /etc/default/motd-news
fi

# Copy our custom MOTD entrypoint
echo "Installing custom MOTD entrypoint..."
sudo rm -f /etc/update-motd.d/00-motd-entrypoint.sh
sudo cp "$DOTFILES_DIR/lib/include/.zshrc.motd" /etc/update-motd.d/00-motd-entrypoint.sh
sudo chmod +x /etc/update-motd.d/00-motd-entrypoint.sh

echo "MOTD configuration complete."
