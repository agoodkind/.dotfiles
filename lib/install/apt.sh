#!/usr/bin/env bash

# Source centralized package list
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/packages.sh"
source "${SCRIPT_DIR}/../include/colors.sh"

# Check if a package is installed
is_installed() {
	dpkg -s "$1" &>/dev/null
}

# Install zoxide if not installed
if ! command -v zoxide &>/dev/null; then
	color_echo BLUE "Installing zoxide..."
	curl -sSfL https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | sh
else
	color_echo GREEN "zoxide already installed, skipping..."
fi


# Setup GitHub CLI repository if not already configured
if [ ! -f /etc/apt/sources.list.d/github-cli.list ]; then
	color_echo BLUE "Setting up GitHub CLI repository..."
	curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
	echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list >/dev/null
else
	color_echo GREEN "GitHub CLI repository already configured, skipping..."
fi


# Add sasl-xoauth2 PPA if not already added
if ! grep -q "^deb .*sasl-xoauth2/stable" /etc/apt/sources.list /etc/apt/sources.list.d/* 2>/dev/null; then
	color_echo BLUE "Adding sasl-xoauth2 PPA..."
	sudo add-apt-repository ppa:sasl-xoauth2/stable -y
else
	color_echo GREEN "sasl-xoauth2 PPA already added, skipping..."
fi


color_echo YELLOW "Updating package lists..."
sudo apt-get update -y -qq


# Build list of packages to install
PACKAGES_TO_INSTALL=()

# Combine common and apt-specific packages
ALL_APT_PACKAGES=("${COMMON_PACKAGES[@]}" "${APT_SPECIFIC[@]}" "arping")

for package in "${ALL_APT_PACKAGES[@]}"; do
	if ! is_installed "$package"; then
		PACKAGES_TO_INSTALL+=("$package")
	fi
done

# Install packages if any are missing
if [ ${#PACKAGES_TO_INSTALL[@]} -gt 0 ]; then
	color_echo YELLOW "Installing ${#PACKAGES_TO_INSTALL[@]} packages..."
	sudo apt-get install -y -qq "${PACKAGES_TO_INSTALL[@]}"
else
	color_echo GREEN "All packages already installed!"
fi


# Install fastfetch if not installed
if ! command -v fastfetch &>/dev/null; then
	color_echo BLUE "Installing latest Fastfetch..."
	curl -s https://api.github.com/repos/fastfetch-cli/fastfetch/releases/latest \
	| grep "browser_download_url.*linux-amd64.deb" \
	| cut -d : -f 2,3 \
	| tr -d \" \
	| wget -qi - -P /tmp
	sudo dpkg -i /tmp/fastfetch-linux-amd64.deb
	rm -f /tmp/fastfetch-linux-amd64.deb
else
	color_echo GREEN "fastfetch already installed, skipping..."
fi


# Purge nano and link nvim
if is_installed nano; then
	color_echo YELLOW "Purging nano and linking nvim as nano..."
	sudo apt-get purge nano -y -qq
	sudo ln -sf $(which nvim) /usr/bin/nano
elif [ ! -e /usr/bin/nano ]; then
	color_echo YELLOW "Linking nvim as nano..."
	sudo ln -sf $(which nvim) /usr/bin/nano
else
	color_echo GREEN "nano already replaced with nvim, skipping..."
fi


# Link batcat to bat
if [ ! -e ~/.local/bin/bat ]; then
	color_echo YELLOW "Linking batcat to ~/.local/bin/bat..."
	mkdir -p ~/.local/bin
	ln -sf /usr/bin/batcat ~/.local/bin/bat
else
	color_echo GREEN "batcat already linked to bat, skipping..."
fi


color_echo GREEN "All done!"