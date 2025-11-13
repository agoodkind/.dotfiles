#!/usr/bin/env bash

# Apt-based installation script for Ubuntu systems
# if not skip install is set, skip installation of packages, --skip-install
if [[ " $* " != *" --skip-install "* ]]; then
    "$DOTDOTFILES/lib/install/apt.sh"
fi

# Script to configure MOTD (Message of the Day) on Ubuntu systems
# Disables default MOTD scripts and optionally sets a custom MOTD

DOTFILES_DIR="${DOTFILES_DIR:-$HOME/.dotfiles}"

echo "Configuring MOTD..."

# Disable all default MOTD scripts except ours
if [ -d /etc/update-motd.d/ ]; then
    echo "Disabling default MOTD scripts in /etc/update-motd.d/"
    for file in /etc/update-motd.d/*; do
        if [[ ! "$file" =~ motd-entrypoint\.sh$ ]]; then
            sudo chmod -x "$file"
        fi
    done
fi

# Copy our custom MOTD entrypoint

echo "Installing custom MOTD entrypoint..."
sudo rm -f /etc/update-motd.d/00-motd-entrypoint.sh
sudo cp "$DOTFILES_DIR/lib/motd-entrypoint.sh" /etc/update-motd.d/00-motd-entrypoint.sh
sudo chmod +x /etc/update-motd.d/00-motd-entrypoint.sh


# Disable motd-news (Ubuntu's dynamic MOTD messages)
if [ -f /etc/default/motd-news ]; then
    echo "Disabling motd-news..."
    sudo sed -i 's/^ENABLED=.*/ENABLED=0/' /etc/default/motd-news
fi

echo "MOTD configuration complete."
