#!/usr/bin/env bash

# GUI Applications
echo "Installing GUI applications..."
brew install --cask 1password
brew install --cask 1password-cli
brew install --cask iterm2
brew install --cask keycastr

# Core Utils
echo "Installing core utilities..."
brew install coreutils moreutils bash grep tree rename most less

# Development Tools
echo "Installing development tools..."
brew install git git-lfs gh node vim ack git-delta

# Network Tools
echo "Installing network tools..."
brew install openssh screen ssh-copy-id teamookla/speedtest/speedtest sshuttle speedtest-cli

# Navigation
echo "Installing navigation tools..."
brew install fzf eza zoxide

# Shell
echo "Installing shell tools..."
brew install zsh thefuck

# Languages
echo "Installing languages tools..."
brew install python3 node pnpm ruby

# Other
echo "Installing other tools..."
brew install ack ack-grep ffmpeg imagemagick bat pandoc glow paper mdless grc
