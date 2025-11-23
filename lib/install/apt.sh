#!/usr/bin/env bash

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Source utilities
source "${DOTDOTFILES}/lib/include/defaults.sh"
source "${DOTDOTFILES}/lib/include/colors.sh"
source "${DOTDOTFILES}/lib/include/packages.sh"

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

# Add backports repository if not already added
color_echo BLUE "Adding backports repository..."
if ! sudo add-apt-repository \
    "deb http://archive.ubuntu.com/ubuntu $(lsb_release -sc)-backports \
    main restricted universe multiverse" -y; then
    color_echo RED "Failed to add backports repository, skipping..."
else
    color_echo GREEN "backports repository added"
fi

color_echo YELLOW "Updating package lists..."
sudo apt-get update -y -qq

# Debug: Show what we're about to process
color_echo YELLOW "Total packages to process: ${#ALL_APT_PACKAGES[@]}"
color_echo CYAN "Packages to process: ${ALL_APT_PACKAGES[*]}"
install_packages "${ALL_APT_PACKAGES[@]}"

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

kill_nano() {
    color_echo YELLOW "Killing nano..."

    if is_installed_via_apt nano; then
        sudo apt-get purge nano -y -qq
    fi

    if is_installed_via_snap nano; then
        sudo snap remove nano
    fi
    
    if command -v nano &>/dev/null; then
        sudo rm -rf "$(which nano)"
    fi

    color_echo YELLOW "Linking nvim as nano..."
    sudo ln -sf "$(which nvim)" /usr/bin/nano
}

kill_nano

# Link batcat to bat
if [ ! -e ~/.local/bin/bat ]; then
    color_echo YELLOW "Linking batcat to ~/.local/bin/bat..."
    mkdir -p ~/.local/bin
    ln -sf "$(which batcat)" ~/.local/bin/bat
else
    color_echo GREEN "batcat already linked to bat, skipping..."
fi

color_echo GREEN "All done!"