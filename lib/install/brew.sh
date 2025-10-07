#!/usr/bin/env bash

/usr/bin/which -s brew || /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

brew install xcodes
brew install showwin/speedtest/speedtest 
brew install wireguard-tools

# Core Utils
echo "Installing core utilities..."
brew install coreutils moreutils bash grep tree rename most less nvim

# Development Tools
echo "Installing development tools..."
brew install git git-lfs gh node vim ack git-delta

# Network Tools
echo "Installing network tools..."
brew install openssh screen ssh-copy-id sshuttle aria2

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

# Cask/Custom Applications
echo "Installing Cask + Custom applications..."
brew install --cask 1password
brew install --cask 1password-cli
brew install --cask iterm2
brew install --cask keycastr
brew install --cask visual-studio-code
brew install --cask google-chrome
brew install --cask font-jetbrains-mono-nerd-font
brew install --cask font-jetbrains-mono
brew install --cask cyberduck
brew install --cask utm
brew install --cask vlc
brew install --cask stats
brew install --cask xcodes-app