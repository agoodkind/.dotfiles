#!/usr/bin/env bash
# NOTE: This file must be bash 3.2 compatible (macOS default)

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Source utilities
source "${DOTDOTFILES}/lib/bash/colors.sh"
source "${DOTDOTFILES}/lib/bash/defaults.sh"
source "${DOTDOTFILES}/lib/bash/packages.sh"

# Install Homebrew if not present
if ! /usr/bin/which -s brew; then
	color_echo BLUE "Installing Homebrew..."
	/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
else
	color_echo GREEN "Homebrew already installed, skipping..."
fi

color_echo YELLOW "Updating Homebrew metadata..."
brew update --quiet

# Fast check if command exists (no brew calls)
cmd_exists() {
	command -v "$1" &>/dev/null
}

# Fast check if cask app exists (no brew calls)
app_exists() {
	local cask="$1"
	local app_name
	app_name=$(get_cask_app_name "$cask")
	
	# Empty app_name means CLI-only or font (check via brew as fallback)
	[[ -z "$app_name" ]] && return 1
	
	[[ -d "/Applications/${app_name}" ]] || [[ -d "$HOME/Applications/${app_name}" ]]
}

# Cached Homebrew state - using regular arrays for bash 3.2 compatibility
INSTALLED_FORMULAE_LIST=""
OUTDATED_FORMULAE_LIST=""
INSTALLED_CASKS_LIST=""
OUTDATED_CASKS_LIST=""

refresh_brew_state() {
	INSTALLED_FORMULAE_LIST=$(brew list --formula 2>/dev/null | tr '\n' ' ')
	OUTDATED_FORMULAE_LIST=$(brew outdated --formula --quiet 2>/dev/null | tr '\n' ' ')
	INSTALLED_CASKS_LIST=$(brew list --cask 2>/dev/null | tr '\n' ' ')
	OUTDATED_CASKS_LIST=$(brew outdated --cask --quiet 2>/dev/null | tr '\n' ' ')
}

refresh_brew_state

# Check if formula is in installed list
is_brew_installed() {
	local pkg="$1"
	[[ " $INSTALLED_FORMULAE_LIST " == *" $pkg "* ]]
}

# Check if cask is in installed list
is_cask_installed() {
	local cask="$1"
	[[ " $INSTALLED_CASKS_LIST " == *" $cask "* ]]
}

# Check if formula is outdated
is_formula_outdated() {
	local pkg="$1"
	[[ " $OUTDATED_FORMULAE_LIST " == *" $pkg "* ]]
}

# Check if cask is outdated
is_cask_outdated() {
	local cask="$1"
	[[ " $OUTDATED_CASKS_LIST " == *" $cask "* ]]
}

# Check if formula is installed (fast or slow based on quick_mode)
check_formula_installed() {
	local pkg="$1"
	if [[ "${quick_mode:-false}" == "true" ]]; then
		cmd_exists "$pkg"
	else
		is_brew_installed "$pkg"
	fi
}

# Check if cask is installed (fast or slow based on quick_mode)
check_cask_installed() {
	local cask="$1"
	if [[ "${quick_mode:-false}" == "true" ]]; then
		app_exists "$cask"
	else
		is_cask_installed "$cask"
	fi
}

# Install special tap packages if not installed
if ! check_formula_installed xcodes; then
	color_echo BLUE "Installing xcodes..."
	brew install xcodes
else
	color_echo GREEN "xcodes already installed, skipping..."
fi

if ! check_formula_installed speedtest; then
	color_echo BLUE "Installing speedtest..."
	brew install showwin/speedtest/speedtest
else
	color_echo GREEN "speedtest already installed, skipping..."
fi

refresh_brew_state

# Build list of packages to install
PACKAGES_TO_INSTALL=()
PACKAGES_TO_UPGRADE=()

# Combine common and brew-specific packages
ALL_BREW_PACKAGES=("${COMMON_PACKAGES[@]}" "${BREW_SPECIFIC[@]}")

color_echo YELLOW "Checking formula packages..."
for package in "${ALL_BREW_PACKAGES[@]}"; do
	if ! check_formula_installed "$package"; then
		PACKAGES_TO_INSTALL+=("$package")
		continue
	fi

	if is_formula_outdated "$package"; then
		PACKAGES_TO_UPGRADE+=("$package")
	fi
done

# Install packages if any are missing
if [ ${#PACKAGES_TO_INSTALL[@]} -gt 0 ]; then
	color_echo YELLOW "Installing ${#PACKAGES_TO_INSTALL[@]} formula packages..."
	brew install "${PACKAGES_TO_INSTALL[@]}"
else
	color_echo GREEN "All formula packages already installed!"
fi

if [ ${#PACKAGES_TO_UPGRADE[@]} -gt 0 ]; then
	color_echo YELLOW "Upgrading ${#PACKAGES_TO_UPGRADE[@]} formula packages..."
	brew upgrade "${PACKAGES_TO_UPGRADE[@]}"
else
	color_echo GREEN "No formula upgrades needed!"
fi

refresh_brew_state

# Install cask applications
CASKS_TO_INSTALL=()
CASKS_TO_UPGRADE=()

color_echo YELLOW "Checking cask applications..."
for cask in "${BREW_CASK_NAMES[@]}"; do
	if ! check_cask_installed "$cask"; then
		CASKS_TO_INSTALL+=("$cask")
		continue
	fi

	if is_cask_outdated "$cask"; then
		CASKS_TO_UPGRADE+=("$cask")
	fi
done

# Install casks if any are missing
if [ ${#CASKS_TO_INSTALL[@]} -gt 0 ]; then
	color_echo YELLOW "Installing ${#CASKS_TO_INSTALL[@]} cask applications..."
	brew install --cask "${CASKS_TO_INSTALL[@]}"
else
	color_echo GREEN "All cask applications already installed!"
fi

if [ ${#CASKS_TO_UPGRADE[@]} -gt 0 ]; then
	color_echo YELLOW "Upgrading ${#CASKS_TO_UPGRADE[@]} cask applications..."
	brew upgrade --cask "${CASKS_TO_UPGRADE[@]}"
else
	color_echo GREEN "No cask upgrades needed!"
fi

color_echo GREEN "All done!"
