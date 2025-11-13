#!/usr/bin/env bash

/usr/bin/which -s brew || /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

brew install xcodes
brew install showwin/speedtest/speedtest 
brew install wireguard-tools

brew install ack ack-grep ansible aria2 bash bat coreutils curl eza fail2ban \
  fastfetch ffmpeg figlet fping gh git git-delta git-lfs glow gping grc grep \
  htop imagemagick jq less mdless most moreutils node nvim openssh pandoc \
  paper pnpm python3 rename rsync ruby screen ssh-copy-id sshuttle thefuck \
  tree vim watch wget zoxide zsh

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
brew install --cask pingplotter