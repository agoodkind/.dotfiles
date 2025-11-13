
#!/usr/bin/env bash

# Color functions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

color_echo() {
	color="$1"; shift
	echo -e "${!color}$*${NC}"
}

color_echo BLUE "Installing zoxide..."
curl -sSfL https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | sh

color_echo BLUE "Setting up GitHub CLI repository..."
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list >/dev/null


color_echo BLUE "Adding sasl-xoauth2 PPA..."
sudo add-apt-repository ppa:sasl-xoauth2/stable -y

color_echo YELLOW "Updating and upgrading packages..."
sudo apt-get update -y
sudo apt-get upgrade -y

color_echo YELLOW "Installing core and extra packages..."
sudo apt-get install -y \
	bash grep rename moreutils net-tools aria2 \
	zsh thefuck fzf eza neovim \
	gh tree ack ack-grep \
	speedtest-cli \
	grc less most pandoc \
	python3 ruby rbenv golang-go nodejs \
	sasl-xoauth2 postfix \
	gping fping ansible bat jq \
	htop curl wget gpg rsyslog locales \
	coreutils watch git git-lfs git-delta \
	openssh-client openssh-server screen sshuttle \
	ffmpeg imagemagick wireguard


color_echo YELLOW "Purging nano and linking nvim as nano..."
sudo apt-get purge nano -y
sudo ln -sf $(which nvim) /usr/bin/nano


color_echo YELLOW "Linking batcat to ~/.local/bin/bat..."
mkdir -p ~/.local/bin
ln -sf /usr/bin/batcat ~/.local/bin/bat

color_echo GREEN "All done!"