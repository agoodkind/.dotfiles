#!/usr/bin/env bash
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
		
		# Check if this package requires git installation
		if git_details=$(get_cargo_git_details "$package"); then
			IFS='|' read -r git_url features <<< "$git_details"
			if [[ -n "$features" ]]; then
				cargo install --git "$git_url" --features "$features"
			else
				cargo install --git "$git_url"
			fi
		else
			cargo install "$package"
		fi
	done
else
	color_echo GREEN "All cargo packages already installed!"
fi

color_echo GREEN "Rust setup complete!"
