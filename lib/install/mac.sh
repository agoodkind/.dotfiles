#!/usr/bin/env bash

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Source utilities
source "${DOTDOTFILES}/lib/bash/colors.sh"
source "${DOTDOTFILES}/lib/bash/defaults.sh"
source "${DOTDOTFILES}/lib/bash/packages.sh"

if [[ " $* " != *" --skip-install "* ]]; then
    "$DOTDOTFILES/lib/install/brew.sh"
fi

# # copy MOTD to /etc/motd
# DOTFILES_DIR="${DOTFILES_DIR:-$HOME/.dotfiles}"
# sudo rm -f /etc/motd
# sudo cp  "$DOTFILES_DIR/lib/motd-entrypoint.mac.sh" /etc/motd
# sudo chmod +x /etc/motd

# Finder
# enable hidden files
defaults write com.apple.finder AppleShowAllFiles -bool true

# show path bar
defaults write com.apple.finder ShowPathbar -bool true

# show status bar
defaults write com.apple.finder ShowStatusBar -bool true

# show all filename extensions
defaults write NSGlobalDomain AppleShowAllExtensions -bool true

# avoid creating .DS_Store files on network or usb volumes
defaults write com.apple.desktopservices DSDontWriteNetworkStores -bool true
defaults write com.apple.desktopservices DSDontWriteUSBStores -bool true

# highlight dock
defaults write com.apple.dock mouse-over-hilite-stack -bool true

# minimize effect
defaults write com.apple.dock mineffect -string suck

# change screenshot location
defaults write com.apple.screencapture location ~/Documents/Screenshots

killall Finder Dock SystemUIServer

# disable google chrome's built-in dns client
defaults write com.google.Chrome BuiltInDnsClientEnabled -bool false

# enable sudo-touchid
# if installed 
sh <( curl -sL git.io/sudo-touch-id )

echo "Mac setup complete!"
