#!/usr/bin/env bash

echo "$DOTDOTFILES"

# Brew
"$DOTDOTFILES/lib/install/brew.sh"

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

killall Finder Dock

# disable google chrome's built-in dns client
defaults write com.google.Chrome BuiltInDnsClientEnabled -bool false

# enable sudo-touchid
# if installed 
[[ -f /opt/homebrew/opt/sudo-touchid/bin/sudo-touchid ]] && /opt/homebrew/opt/sudo-touchid/bin/sudo-touchid -q -y
