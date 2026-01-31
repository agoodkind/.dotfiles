#!/usr/bin/env bash
# Go packages installation script
# Requires bash 4.0+ for associative arrays in packages.sh

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Ensure we're running bash 4+
if [[ "${BASH_VERSINFO[0]}" -lt 4 ]]; then
	echo "Error: This script requires bash 4.0 or later (current: $BASH_VERSION)"
	echo "Please install modern bash first"
	exit 1
fi

# Source utilities
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"
source "${DOTDOTFILES}/lib/setup/helpers/defaults.sh"
source "${DOTDOTFILES}/lib/setup/helpers/packages.sh"

# Check if Go is installed
if ! command -v go &>/dev/null; then
	color_echo YELLOW "Go not installed, skipping Go packages"
	exit 0
fi

# Ensure GOPATH/bin is in PATH for checking installed packages
export PATH="$PATH:$(go env GOPATH)/bin"

# Check if go package binary is installed
is_go_installed() {
	local package="$1"
	command -v "$package" &>/dev/null
}

# Install Go packages
GO_TO_INSTALL=()

color_echo YELLOW "Checking Go packages..."

for package in "${!GO_PACKAGES[@]}"; do
	if ! is_go_installed "$package"; then
		GO_TO_INSTALL+=("$package")
	fi
done

# Install packages if any are missing
if [ ${#GO_TO_INSTALL[@]} -gt 0 ]; then
	color_echo YELLOW "Installing ${#GO_TO_INSTALL[@]} Go packages..."
	for package in "${GO_TO_INSTALL[@]}"; do
		install_path="${GO_PACKAGES[$package]}"
		color_echo CYAN "Installing $package from $install_path..."
		go install "$install_path"
	done
else
	color_echo GREEN "All Go packages already installed!"
fi

color_echo GREEN "Go packages setup complete!"
