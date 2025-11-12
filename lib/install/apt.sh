#!/usr/bin/env bash

# install zoxide
curl -sSfL https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | sh

# install github cli
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list >/dev/null

sudo add-apt-repository ppa:sasl-xoauth2/stable -y

sudo apt-get update -y
sudo apt-get upgrade -y

# install email tools
sudo apt-get install sasl-xoauth2 postfix -y

# install tools
sudo apt-get install gh tree ack ack-grep -y

# System utilities
sudo apt-get install speedtest-cli moreutils bash grep rename aria2 net-tools -y

# Shell and terminal tools
sudo apt-get install zsh thefuck fzf eza neovim -y

# Text processing and paging tools
sudo apt-get install pandoc grc less most -y

# install languages
sudo apt-get install python3 ruby rbenv golang-go nodejs -y

# install network tools
sudo apt-get install gping fping ansible -y

sudo apt-get remove nano -y
sudo ln -sf $(which nvim) /usr/bin/nano

# install bat and link to ~/.local/bin (to avoid conflicts)
sudo apt-get install bat -y
mkdir -p ~/.local/bin
ln -sf /usr/bin/batcat ~/.local/bin/bat