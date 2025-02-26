#!/usr/bin/env bash

# GUI Applications
echo "Installing GUI applications..."
brew install --cask 1password
brew install --cask 1password-cli
brew install --cask iterm2
brew install --cask keycastr

# Core Utils
echo "Installing core utilities..."
brew install coreutils
brew install moreutils
brew install bash
brew install grep
brew install tree
brew install rename

# Development Tools
echo "Installing development tools..."
brew install git
brew install git-lfs
brew install gh
brew install node
brew install vim 
brew install ack

# Network Tools
echo "Installing network tools..."
brew install openssh
brew install screen
brew install ssh-copy-id
brew install teamookla/speedtest/speedtest

# Navigation
echo "Installing navigation tools..."
brew install zoxide