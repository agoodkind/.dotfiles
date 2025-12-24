#!/usr/bin/env bash

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Source utilities
source "${DOTDOTFILES}/lib/bash/colors.sh"
source "${DOTDOTFILES}/lib/bash/defaults.sh"
source "${DOTDOTFILES}/lib/bash/packages.sh"

# Install Rust via rustup if not present
if ! command -v rustup &>/dev/null; then
	color_echo BLUE "Installing Rust via rustup..."
	curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
	
	# Source cargo env to make it available in this session
	if [[ -f "$HOME/.cargo/env" ]]; then
		source "$HOME/.cargo/env"
	fi
	
	color_echo GREEN "Rust installed successfully"
else
	color_echo GREEN "Rust already installed, skipping..."
fi

# Ensure cargo is available
if ! command -v cargo &>/dev/null; then
	if [[ -f "$HOME/.cargo/env" ]]; then
		source "$HOME/.cargo/env"
	fi
fi

# Verify cargo is now available
if ! command -v cargo &>/dev/null; then
	color_echo RED "cargo not found after installation, skipping cargo packages"
	exit 0
fi

# Check if cargo package is installed
is_cargo_installed() {
	local package="$1"
	cargo install --list | grep -q "^${package} v"
}

# Install cargo packages
CARGO_TO_INSTALL=()

color_echo YELLOW "Checking cargo packages..."

for package in "${CARGO_PACKAGES[@]}"; do
	if ! is_cargo_installed "$package"; then
		CARGO_TO_INSTALL+=("$package")
	fi
done

# Install packages if any are missing
if [ ${#CARGO_TO_INSTALL[@]} -gt 0 ]; then
	color_echo YELLOW "Installing ${#CARGO_TO_INSTALL[@]} cargo packages..."
	for package in "${CARGO_TO_INSTALL[@]}"; do
		color_echo CYAN "Installing $package..."
		cargo install "$package"
	done
else
	color_echo GREEN "All cargo packages already installed!"
fi

color_echo GREEN "Rust setup complete!"
