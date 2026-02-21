#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Source utilities
source "${DOTDOTFILES}/bash/core/colors.bash"
source "${DOTDOTFILES}/bash/core/defaults.bash"
source "${DOTDOTFILES}/bash/core/packages.bash"

# Build brew.sh args from passed flags
BREW_ARGS=()
for arg in "$@"; do
    case "$arg" in
        --skip-casks) BREW_ARGS+=("--skip-casks") ;;
    esac
done

if [[ " $* " != *" --skip-install "* ]]; then
    "$DOTDOTFILES/bash/setup/platform/brew.bash" "${BREW_ARGS[@]}"
    "$DOTDOTFILES/bash/setup/platform/rust.bash"
fi

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
