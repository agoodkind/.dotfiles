#!/usr/bin/env bash

# Brew
$DOTDOTFILES/lib/install/brew.sh

# Finder
# enable hidden files
defaults write com.apple.finder AppleShowAllFiles -bool true

# show path bar
defaults write com.apple.finder ShowPathbar -bool true

# show status bar
defaults write com.apple.finder ShowStatusBar -bool true

# disable google chrome's built-in dns client
defaults write com.google.Chrome BuiltInDnsClientEnabled -bool false

