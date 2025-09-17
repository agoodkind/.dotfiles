#!/usr/bin/env bash

# install zoxide
curl -sSfL https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | sh

# install github cli
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list >/dev/null
sudo apt update -y
sudo apt upgrade -y
sudo apt install gh

# install neovim
sudo apt install neovim -y

# install tools
sudo apt install speedtest-cli moreutils bash grep tree rename ack ack-grep -y

# install shell tools
sudo apt install zsh thefuck fzf eza bat pandoc grc less most -y

# install languages
sudo apt install python3 ruby -y
