#!/usr/bin/env bash

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Source utilities
source "${DOTDOTFILES}/lib/include/defaults.sh"
source "${DOTDOTFILES}/lib/include/colors.sh"
source "${DOTDOTFILES}/lib/include/packages.sh"

# Install Homebrew if not present
if ! /usr/bin/which -s brew; then
	color_echo BLUE "Installing Homebrew..."
	/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
else
	color_echo GREEN "Homebrew already installed, skipping..."
fi

# Check if a brew package/formula is installed
is_brew_installed() {
	brew list --formula "$1" &>/dev/null
}

# Check if a cask is installed
is_cask_installed() {
	brew list --cask "$1" &>/dev/null
}

# Install special tap packages if not installed
if ! is_brew_installed xcodes; then
	color_echo BLUE "Installing xcodes..."
	brew install xcodes
else
	color_echo GREEN "xcodes already installed, skipping..."
fi

if ! is_brew_installed speedtest; then
	color_echo BLUE "Installing speedtest..."
	brew install showwin/speedtest/speedtest
else
	color_echo GREEN "speedtest already installed, skipping..."
fi

# Build list of packages to install
PACKAGES_TO_INSTALL=()

# Combine common and brew-specific packages
ALL_BREW_PACKAGES=("${COMMON_PACKAGES[@]}" "${BREW_SPECIFIC[@]}")

color_echo YELLOW "Checking formula packages..."
for package in "${ALL_BREW_PACKAGES[@]}"; do
	if ! is_brew_installed "$package"; then
		PACKAGES_TO_INSTALL+=("$package")
	fi
done

# Install packages if any are missing
if [ ${#PACKAGES_TO_INSTALL[@]} -gt 0 ]; then
	color_echo YELLOW "Installing ${#PACKAGES_TO_INSTALL[@]} formula packages..."
	brew install "${PACKAGES_TO_INSTALL[@]}"
else
	color_echo GREEN "All formula packages already installed!"
fi

# Install cask applications
CASKS_TO_INSTALL=()

color_echo YELLOW "Checking cask applications..."
for cask in "${BREW_CASKS[@]}"; do
	if ! is_cask_installed "$cask"; then
		CASKS_TO_INSTALL+=("$cask")
	fi
done

# Install casks if any are missing
if [ ${#CASKS_TO_INSTALL[@]} -gt 0 ]; then
	color_echo YELLOW "Installing ${#CASKS_TO_INSTALL[@]} cask applications..."
	brew install --cask "${CASKS_TO_INSTALL[@]}"
else
	color_echo GREEN "All cask applications already installed!"
fi

color_echo GREEN "All done!"