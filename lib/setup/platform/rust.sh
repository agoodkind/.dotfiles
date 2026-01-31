#!/usr/bin/env bash
set -e
set -o pipefail
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
	# Map package name to binary name if different
	local binary="$package"
	case "$package" in
		"tree-sitter-cli") binary="tree-sitter" ;;
		"cloudflare-speed-cli") binary="cloudflare-speed-cli" ;;
		"async-cmd") binary="async" ;;
	esac
	command -v "$binary" &>/dev/null
}

install_cloudflare_speed_cli() {
	color_echo CYAN "Installing cloudflare-speed-cli from GitHub releases..."
	local repo="kavehtehrani/cloudflare-speed-cli"
	local arch
	case "$(uname -m)" in
		x86_64) arch="x86_64" ;;
		arm64|aarch64) arch="aarch64" ;;
		*) color_echo RED "Unsupported architecture for cloudflare-speed-cli binary"; return 1 ;;
	esac
	
	local os="unknown-linux-musl"
	if [[ "$OSTYPE" == "darwin"* ]]; then
		os="apple-darwin"
	fi

	local version
	version=$(curl -s "https://api.github.com/repos/$repo/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
	local filename="cloudflare-speed-cli-$arch-$os.tar.xz"
	local url="https://github.com/kavehtehrani/cloudflare-speed-cli/releases/download/$version/$filename"
	
	curl -L "$url" -o "/tmp/$filename"
	mkdir -p "/tmp/cloudflare-speed-cli"
	tar -xf "/tmp/$filename" -C "/tmp/cloudflare-speed-cli"
	
	local bin_name="cloudflare-speed-cli"
	# macOS tar might preserve directory structure or just extract files
	local bin_path
	bin_path=$(find "/tmp/cloudflare-speed-cli" -name "$bin_name" -type f | head -n 1)
	
	if [[ -x "$bin_path" ]]; then
		mkdir -p "$HOME/.cargo/bin"
		cp "$bin_path" "$HOME/.cargo/bin/"
		color_echo GREEN "cloudflare-speed-cli installed to ~/.cargo/bin"
	else
		color_echo RED "Failed to find binary in cloudflare-speed-cli archive"
		return 1
	fi
}

# Install cargo packages
CARGO_TO_INSTALL=()

color_echo YELLOW "Checking cargo packages..."

for package in "${CARGO_PACKAGES[@]}"; do
	if ! is_cargo_installed "$package"; then
		CARGO_TO_INSTALL+=("$package")
	fi
done

# Install cargo-binstall if not present
if ! command -v cargo-binstall &>/dev/null; then
	color_echo BLUE "Installing cargo-binstall..."
	curl -L --proto '=https' --tlsv1.2 -sSf https://raw.githubusercontent.com/cargo-bins/cargo-binstall/main/install-from-binstall-release.sh | bash
fi

# Install packages if any are missing
if [ ${#CARGO_TO_INSTALL[@]} -gt 0 ]; then
	color_echo YELLOW "Installing ${#CARGO_TO_INSTALL[@]} cargo packages..."
	for package in "${CARGO_TO_INSTALL[@]}"; do
		color_echo CYAN "Checking $package..."
		
		# 1. Try First-Party Binary Installers
		case "$package" in
			starship)
				color_echo CYAN "Installing starship via official installer..."
				curl -sS https://starship.rs/install.sh | sh -s -- --yes
				continue
				;;
			atuin)
				color_echo CYAN "Installing atuin via official installer..."
				curl --proto '=https' --tlsv1.2 -sSf https://setup.atuin.sh | sh -s -- --yes
				continue
				;;
			xh)
				color_echo CYAN "Installing xh via official installer..."
				curl -sfL https://raw.githubusercontent.com/ducaale/xh/master/install.sh | sh
				continue
				;;
			cloudflare-speed-cli)
				if install_cloudflare_speed_cli; then
					continue
				fi
				;;
			async-cmd)
				if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
					color_echo YELLOW "⏭️  Skipping async-cmd compilation in CI"
					continue
				fi
				;;
		esac

		# 2. Fallback to cargo-binstall or cargo install
		color_echo CYAN "Installing $package via cargo/binstall..."
		
		# Check if this package requires git installation
		if git_details=$(get_cargo_git_details "$package"); then
			IFS='|' read -r git_url features <<< "$git_details"
			if [[ -n "$features" ]]; then
				cargo install --git "$git_url" --features "$features"
			else
				cargo install --git "$git_url"
			fi
		else
			if command -v cargo-binstall &>/dev/null; then
				cargo binstall -y "$package"
			else
				cargo install "$package"
			fi
		fi
	done
else
	color_echo GREEN "All cargo packages already installed!"
fi

color_echo GREEN "Rust setup complete!"
