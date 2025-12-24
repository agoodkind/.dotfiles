#!/usr/bin/env bash

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Source utilities
source "${DOTDOTFILES}/lib/bash/colors.sh"
source "${DOTDOTFILES}/lib/bash/defaults.sh"
source "${DOTDOTFILES}/lib/bash/packages.sh"

# =============================================================================
# APT/Snap Helper Functions
# =============================================================================

# Map common package names to APT package names
# Uses get_package_name() from packages.sh
map_to_apt_name() {
	local package="$1"
	get_package_name "$package" "apt"
}

# Map package names to their snap equivalents (if different)
# Uses get_package_name() from packages.sh
map_to_snap_name() {
	local package="$1"
	get_package_name "$package" "snap"
}

is_installed_via_snap() {
    snap list "$1" &>/dev/null
    return $?
}

is_installed_via_apt() {
    dpkg -s "$1" &>/dev/null
    return $?
}

requires_classic_confinement() {
    local package="$1"
    local info_output=$(snap info "$package" 2>/dev/null)
    if [ $? -ne 0 ]; then
        return 1  # Can't determine, assume not classic
    fi
    # Check top-level confinement field first
    local confinement=$(echo "$info_output" | grep "^confinement:" | awk '{print $2}')
    if [ "$confinement" = "classic" ]; then
        return 0  # true - requires classic
    fi
    # Also check channels section for classic
    if echo "$info_output" | grep -qE "latest/[^:]+:\s+[0-9].*\s+classic"; then
        return 0  # true - requires classic
    fi
    return 1  # false - doesn't require classic
}

# =============================================================================
# Build ALL_APT_PACKAGES array
# =============================================================================

ALL_APT_PACKAGES=()

# Add mapped common packages (skip duplicates that are in APT_SPECIFIC or SNAP_PACKAGES)
for package in "${COMMON_PACKAGES[@]}"; do
	# Skip packages that should be installed via snap
	if is_in_array "$package" "${SNAP_PACKAGES[@]}"; then
		continue
	fi
	
	mapped=$(map_to_apt_name "$package")
	# Handle multi-word output (like openssh -> openssh-client openssh-server)
	for pkg in $mapped; do
		# Skip if already in APT_SPECIFIC (they'll be added separately)
		if ! is_in_array "$pkg" "${APT_SPECIFIC[@]}"; then
			# Skip if already in ALL_APT_PACKAGES
			if ! is_in_array "$pkg" "${ALL_APT_PACKAGES[@]}"; then
				ALL_APT_PACKAGES+=("$pkg")
			fi
		fi
	done
done

# Add all APT_SPECIFIC packages
ALL_APT_PACKAGES+=("${APT_SPECIFIC[@]}")

# =============================================================================
# Install packages function
# =============================================================================

install_packages() {
    debug_echo "install_packages() called with $# package(s)"
    
    local packages_to_install_via_apt=()
    local packages_to_install_via_snap=()
    
    debug_echo "Processing $# packages..."
    debug_echo "Package list: $*"
    
    for package in "$@"; do
        debug_echo "--- Processing package: $package ---"
        
        # Check if this package should be installed via snap
        if is_in_array "$package" "${SNAP_PACKAGES[@]}"; then
            debug_echo "  Package $package is in SNAP_PACKAGES, will install via snap"
            local snap_name=$(map_to_snap_name "$package")
            
            if is_installed_via_snap "$snap_name"; then
                debug_echo "  SKIP: $snap_name already installed via snap"
            else
                debug_echo "  Added $snap_name to snap install list"
                packages_to_install_via_snap+=("$snap_name")
            fi
        else
            debug_echo "  Package $package will be installed via apt"
            if is_installed_via_apt "$package"; then
                debug_echo "  SKIP: $package already installed via apt"
            else
                debug_echo "  Added $package to apt install list"
                packages_to_install_via_apt+=("$package")
            fi
        fi
    done
    
    debug_echo "Summary:"
    debug_echo "  packages_to_install_via_apt: ${#packages_to_install_via_apt[@]} packages"
    debug_echo "  packages_to_install_via_snap: ${#packages_to_install_via_snap[@]} packages"

    # Install apt packages
    if [ ${#packages_to_install_via_apt[@]} -gt 0 ]; then
        debug_echo "Installing ${#packages_to_install_via_apt[@]} apt packages: ${packages_to_install_via_apt[*]}"
        color_echo YELLOW "Installing ${#packages_to_install_via_apt[@]} apt packages..."
        sudo apt-get install -y -qq "${packages_to_install_via_apt[@]}"
        debug_echo "apt-get install completed with exit code: $?"
    else
        debug_echo "No apt packages to install"
    fi

    # Install snap packages
    if [ ${#packages_to_install_via_snap[@]} -gt 0 ]; then
        debug_echo "Installing ${#packages_to_install_via_snap[@]} snap packages: ${packages_to_install_via_snap[*]}"
        color_echo YELLOW "Installing ${#packages_to_install_via_snap[@]} snap packages..."
        for snap_package in "${packages_to_install_via_snap[@]}"; do
            debug_echo "--- Installing snap package: $snap_package ---"
            color_echo CYAN "Installing $snap_package via snap..."
            
            # Find the original package name from SNAP_PACKAGES that maps to this snap name
            local original_package=""
            for pkg in "${SNAP_PACKAGES[@]}"; do
                if [ "$(map_to_snap_name "$pkg")" = "$snap_package" ]; then
                    original_package="$pkg"
                    break
                fi
            done
            
            # Remove apt equivalent if installed
            if [ -n "$original_package" ]; then
                local apt_name=$(map_to_apt_name "$original_package")
                for apt_pkg in $apt_name; do
                    if is_installed_via_apt "$apt_pkg"; then
                        debug_echo "  Removing apt package $apt_pkg (replacing with snap $snap_package)..."
                        color_echo YELLOW "  -> Removing $apt_pkg from apt..."
                        sudo apt-get remove -y -qq "$apt_pkg" 2>&1 | grep -v "^$" || true
                        debug_echo "  Removed $apt_pkg via apt"
                    fi
                done
            fi
            
            local install_output
            local install_status
            
            if requires_classic_confinement "$snap_package"; then
                debug_echo "  Package requires classic confinement, installing with --classic"
                install_output=$(sudo snap install --classic "$snap_package" 2>&1)
                install_status=$?
            else
                debug_echo "  Package does not require classic confinement, installing normally"
                install_output=$(sudo snap install "$snap_package" 2>&1)
                install_status=$?
                
                # If it failed and error mentions classic confinement, retry with --classic
                if [ $install_status -ne 0 ] && echo "$install_output" | grep -qi "classic confinement"; then
                    debug_echo "  Installation failed, error mentions classic confinement, retrying with --classic"
                    color_echo YELLOW "  -> Retrying $snap_package with --classic flag..."
                    install_output=$(sudo snap install --classic "$snap_package" 2>&1)
                    install_status=$?
                fi
            fi
            
            if [ $install_status -eq 0 ]; then
                debug_echo "  SUCCESS: $snap_package installed via snap"
                color_echo GREEN "  -> Successfully installed $snap_package via snap"
            else
                debug_echo "  FAILED: $snap_package installation failed, exit code: $install_status"
                debug_echo "  Error output: $install_output"
                echo "$install_output" | grep -v "^$" || true
                color_echo RED "  -> Failed to install $snap_package via snap"
            fi
        done
    else
        debug_echo "No snap packages to install"
    fi
    
    debug_echo "install_packages() completed"
}

# =============================================================================
# Repository Setup
# =============================================================================

# Install zoxide if not installed
if ! command -v zoxide &>/dev/null; then
    color_echo BLUE "Installing zoxide..."
    curl -sSfL \
        https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh \
        | sh
else
    color_echo GREEN "zoxide already installed, skipping..."
fi

# Setup GitHub CLI repository if not already configured
GITHUB_CLI_LIST="/etc/apt/sources.list.d/github-cli.list"
GITHUB_CLI_KEY="/usr/share/keyrings/githubcli-archive-keyring.gpg"
if [ ! -f "$GITHUB_CLI_LIST" ] || [ ! -f "$GITHUB_CLI_KEY" ]; then
    color_echo BLUE "Setting up GitHub CLI repository..."
    curl -fsSL \
        https://cli.github.com/packages/githubcli-archive-keyring.gpg \
        | sudo dd of="$GITHUB_CLI_KEY"
    ARCH=$(dpkg --print-architecture)
    echo "deb [arch=$ARCH signed-by=$GITHUB_CLI_KEY] \
https://cli.github.com/packages stable main" \
        | sudo tee "$GITHUB_CLI_LIST" >/dev/null
    color_echo GREEN "GitHub CLI repository configured"
else
    color_echo GREEN \
        "GitHub CLI repository already configured, skipping..."
fi

# Detect distribution (Ubuntu vs Debian vs other)
DISTRO_ID=""
DISTRO_CODENAME=""
if [ -f /etc/os-release ]; then
    DISTRO_ID=$(grep -E "^ID=" /etc/os-release 2>/dev/null \
        | cut -d= -f2 | tr -d '"' | tr '[:upper:]' '[:lower:]' || echo "")
    DISTRO_CODENAME=$(grep -E "^VERSION_CODENAME=" /etc/os-release \
        2>/dev/null | cut -d= -f2 | tr -d '"' || \
        grep -E "^UBUNTU_CODENAME=" /etc/os-release 2>/dev/null \
        | cut -d= -f2 | tr -d '"' || echo "")
fi

# Check if add-apt-repository is available
# (Ubuntu-specific, but Debian may have it too)
if ! command -v add-apt-repository &>/dev/null; then
    color_echo YELLOW \
        "add-apt-repository not found, attempting to install \
software-properties-common..."
    sudo apt-get update -qq
    if sudo apt-get install -y -qq software-properties-common \
        2>/dev/null; then
        if ! command -v add-apt-repository &>/dev/null; then
            color_echo YELLOW \
                "add-apt-repository still not available after \
installation (may not be supported on this distribution)"
        fi
    else
        color_echo YELLOW \
            "Failed to install software-properties-common \
(may not be available on this distribution)"
    fi
fi

# Add sasl-xoauth2 PPA if not already added
# (Ubuntu-only, PPAs don't work on Debian)
if [ "$DISTRO_ID" = "ubuntu" ]; then
    if command -v add-apt-repository &>/dev/null; then
        PPA_PATTERN="/etc/apt/sources.list.d/*sasl-xoauth2*stable*.list"
        PPA_GREP="sasl-xoauth2.*stable|ppa\.launchpadcontent\.net/sasl-xoauth2/stable"
        if ! compgen -G "$PPA_PATTERN" >/dev/null 2>&1 && \
           ! grep -qE "$PPA_GREP" \
              /etc/apt/sources.list /etc/apt/sources.list.d/* \
              2>/dev/null; then
            color_echo BLUE "Adding sasl-xoauth2 PPA..."
            if sudo add-apt-repository ppa:sasl-xoauth2/stable -y \
                2>/dev/null; then
                color_echo GREEN \
                    "sasl-xoauth2 PPA added successfully"
            else
                color_echo RED \
                    "Failed to add sasl-xoauth2 PPA, skipping..."
            fi
        else
            color_echo GREEN \
                "sasl-xoauth2 PPA already added, skipping..."
        fi
    else
        color_echo YELLOW \
            "add-apt-repository not available, \
skipping sasl-xoauth2 PPA..."
    fi
else
    color_echo YELLOW \
        "PPAs are Ubuntu-specific, skipping sasl-xoauth2 PPA on \
$DISTRO_ID..."
fi

# Get release codename for backports repository
# (use previously detected codename or detect)
RELEASE_CODENAME="$DISTRO_CODENAME"
if [ -z "$RELEASE_CODENAME" ] && \
    command -v lsb_release &>/dev/null; then
    RELEASE_CODENAME=$(lsb_release -sc 2>/dev/null)
fi

# Fallback: try to get codename from /etc/os-release
# (if not already done)
if [ -z "$RELEASE_CODENAME" ] && [ -f /etc/os-release ]; then
    RELEASE_CODENAME=$(grep -E "^VERSION_CODENAME=" /etc/os-release \
        2>/dev/null | cut -d= -f2 | tr -d '"' || \
        grep -E "^UBUNTU_CODENAME=" /etc/os-release 2>/dev/null \
        | cut -d= -f2 | tr -d '"')
fi

# If still no codename found, try installing lsb-release
if [ -z "$RELEASE_CODENAME" ]; then
    color_echo YELLOW \
        "Release codename not found, attempting to install \
lsb-release..."
    sudo apt-get update -qq
    if sudo apt-get install -y -qq lsb-release 2>/dev/null && \
        command -v lsb_release &>/dev/null; then
        RELEASE_CODENAME=$(lsb_release -sc 2>/dev/null)
    fi
fi

# Add backports repository if codename is available and
# repository not already added
if [ -n "$RELEASE_CODENAME" ]; then
    BACKPORTS_PATTERN="$RELEASE_CODENAME-backports"
    if ! grep -qE "$BACKPORTS_PATTERN" \
        /etc/apt/sources.list /etc/apt/sources.list.d/* \
        2>/dev/null; then
        color_echo BLUE \
            "Adding backports repository for $RELEASE_CODENAME..."
        
        # Determine repository URL and components based on distribution
        if [ "$DISTRO_ID" = "debian" ]; then
            # Debian backports
            REPO_URL="http://deb.debian.org/debian"
            REPO_COMPONENTS="main contrib non-free"
            BACKPORTS_REPO="deb $REPO_URL $RELEASE_CODENAME-backports \
$REPO_COMPONENTS"
        elif [ "$DISTRO_ID" = "ubuntu" ]; then
            # Ubuntu backports
            REPO_URL="http://archive.ubuntu.com/ubuntu"
            REPO_COMPONENTS="main restricted universe multiverse"
            BACKPORTS_REPO="deb $REPO_URL $RELEASE_CODENAME-backports \
$REPO_COMPONENTS"
        else
            # Unknown distribution - try to infer from existing sources
            color_echo YELLOW \
                "Unknown distribution '$DISTRO_ID', attempting to \
detect repository structure..."
            UBUNTU_PATTERN="archive\.ubuntu\.com|ppa\.launchpadcontent\.net"
            DEBIAN_PATTERN="deb\.debian\.org"
            if grep -qE "$UBUNTU_PATTERN" \
                /etc/apt/sources.list /etc/apt/sources.list.d/* \
                2>/dev/null; then
                REPO_URL="http://archive.ubuntu.com/ubuntu"
                REPO_COMPONENTS="main restricted universe multiverse"
                BACKPORTS_REPO="deb $REPO_URL \
$RELEASE_CODENAME-backports $REPO_COMPONENTS"
            elif grep -qE "$DEBIAN_PATTERN" \
                /etc/apt/sources.list /etc/apt/sources.list.d/* \
                2>/dev/null; then
                REPO_URL="http://deb.debian.org/debian"
                REPO_COMPONENTS="main contrib non-free"
                BACKPORTS_REPO="deb $REPO_URL $RELEASE_CODENAME-backports \
$REPO_COMPONENTS"
            else
                color_echo RED \
                    "Could not determine repository structure for \
$DISTRO_ID, skipping backports..."
                REPO_URL=""
            fi
        fi
        
        # Add backports repository if we determined the structure
        if [ -n "$REPO_URL" ]; then
            BACKPORTS_LIST="/etc/apt/sources.list.d/backports.list"
            if command -v add-apt-repository &>/dev/null; then
                if sudo add-apt-repository "$BACKPORTS_REPO" -y \
                    2>/dev/null; then
                    color_echo GREEN "backports repository added"
                else
                    color_echo YELLOW \
                        "add-apt-repository failed, trying manual \
method..."
                    if echo "$BACKPORTS_REPO" | \
                        sudo tee -a "$BACKPORTS_LIST" >/dev/null 2>&1; \
                    then
                        color_echo GREEN \
                            "backports repository added manually"
                    else
                        color_echo RED \
                            "Failed to manually add backports \
repository, skipping..."
                    fi
                fi
            else
                color_echo YELLOW \
                    "add-apt-repository not available, manually \
adding backports repository..."
                if echo "$BACKPORTS_REPO" | \
                    sudo tee -a "$BACKPORTS_LIST" >/dev/null 2>&1; \
                then
                    color_echo GREEN \
                        "backports repository added manually"
                else
                    color_echo RED \
                        "Failed to manually add backports repository, \
skipping..."
                fi
            fi
        fi
    else
        color_echo GREEN \
            "backports repository already added, skipping..."
    fi
else
    color_echo RED \
        "Could not determine release codename, skipping backports \
repository..."
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
    FASTFETCH_URL="https://api.github.com/repos/fastfetch-cli/fastfetch/releases/latest"
    FASTFETCH_DEB_URL=$(curl -s "$FASTFETCH_URL" \
        | grep "browser_download_url.*linux-amd64.deb" \
        | cut -d : -f 2,3 \
        | tr -d \" \
        | tr -d ' ')
    if [[ -n "$FASTFETCH_DEB_URL" ]]; then
        curl -fsSL "$FASTFETCH_DEB_URL" -o /tmp/fastfetch-linux-amd64.deb
        sudo dpkg -i /tmp/fastfetch-linux-amd64.deb
        rm -f /tmp/fastfetch-linux-amd64.deb
    else
        color_echo RED "Failed to get fastfetch download URL"
    fi
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

    if command -v nvim &>/dev/null; then
        color_echo YELLOW "Linking nvim as nano..."
        sudo ln -sf "$(command -v nvim)" /usr/bin/nano
    else
        color_echo YELLOW "nvim not found, skipping nano symlink..."
    fi
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