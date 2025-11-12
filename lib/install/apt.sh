#!/usr/bin/env bash

# install zoxide
curl -sSfL https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | sh

# install github cli
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list >/dev/null

sudo add-apt-repository ppa:sasl-xoauth2/stable -y


sudo apt-get update -y
sudo apt-get upgrade -y

# Core utilities and shells
sudo apt-get install -y \
	bash grep rename moreutils net-tools aria2 \
	zsh thefuck fzf eza neovim nvim \
	gh tree ack ack-grep \
	speedtest-cli \
	grc less most pandoc \
	python3 ruby rbenv golang-go nodejs \
	sasl-xoauth2 postfix \
	gping fping ansible bat jq

sudo apt-get purge nano -y
sudo ln -sf $(which nvim) /usr/bin/nano

# link bat to ~/.local/bin (to avoid conflicts)
mkdir -p ~/.local/bin
ln -sf /usr/bin/batcat ~/.local/bin/bat