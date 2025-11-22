#!/usr/bin/env bash

# Source centralized package list
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/packages.sh"
source "${SCRIPT_DIR}/../include/colors.sh"

is_installed_via_snap() {
    snap list "$1" &>/dev/null
    return $?
}

is_available_via_snap() {
    # Check if package is available on stable channel
    local package="$1"
    local info_output=$(snap info "$package" 2>&1)
    if [ $? -ne 0 ]; then
        return 1  # Not available
    fi
    # Check if stable channel has a version (not just "–" or empty)
    local stable_line=$(echo "$info_output" | grep "latest/stable:" | head -1)
    if [ -z "$stable_line" ]; then
        return 1  # No stable channel line found
    fi
    # If the stable line contains a version number pattern (e.g., "3.30 2020-12-09"), it's available
    # If it only contains "–" or "-" or is empty, it's not available
    if echo "$stable_line" | grep -qE "latest/stable:\s+[0-9]"; then
        return 0  # Available on stable (has version number)
    fi
    return 1  # Not available on stable (shows "–" or no version)
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
    # Also check channels section for classic (some snaps show it there, e.g., "latest/beta: 3.30 ... classic")
    if echo "$info_output" | grep -qE "latest/[^:]+:\s+[0-9].*\s+classic"; then
        return 0  # true - requires classic
    fi
    return 1  # false - doesn't require classic
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

# Install packages if any are missing
install_packages() {
    # Build list of packages to install
    local packages_to_install_via_apt=()
    local packages_to_remove_via_apt=()
    local packages_to_install_via_snap=()

    # Ask if user wants to use snap for available packages
    local use_snap=false
    local remove_from_apt=false

    read -p "Use snap for available packages? (Y/n) " -n 1 -r
    echo
    if [[ -z "$REPLY" || $REPLY =~ ^[Yy]$ ]]; then
        color_echo GREEN "OK will use snap for available packages"
        use_snap=true
        read -p "Remove packages from apt if available via snap? [Y/n] " -n 1 -r
        echo
        if [[ -z "$REPLY" || $REPLY =~ ^[Yy]$ ]]; then
            color_echo GREEN "OK will remove packages from apt if available via snap"
            remove_from_apt=true
        fi
    fi

    # DEBUG: Show what packages we're processing
    color_echo YELLOW "Processing $# packages..."
    
    for package in "$@"; do
        if [[ "$use_snap" == "true" ]]; then
            if is_available_via_snap "$package"; then
                if is_installed_via_snap "$package"; then
                    continue
                elif is_installed_via_apt "$package"; then
                    if [[ "$remove_from_apt" == "true" ]]; then
                        packages_to_remove_via_apt+=("$package")
                        packages_to_install_via_snap+=("$package")
                    fi
                else
                    packages_to_install_via_snap+=("$package")
                fi
            else
                if ! is_installed_via_apt "$package"; then
                    packages_to_install_via_apt+=("$package")
                fi
            fi
        elif ! is_installed_via_apt "$package"; then
            packages_to_install_via_apt+=("$package")
        fi
    done

    if [ ${#packages_to_install_via_apt[@]} -gt 0 ]; then
        color_echo YELLOW "Installing ${#packages_to_install_via_apt[@]} apt packages..."
        sudo apt-get install -y -qq "${packages_to_install_via_apt[@]}"
    fi

    if [ ${#packages_to_remove_via_apt[@]} -gt 0 ]; then
        color_echo YELLOW "Removing ${#packages_to_remove_via_apt[@]} apt packages..."
        sudo apt-get remove -y -qq "${packages_to_remove_via_apt[@]}"
    fi

    if [ ${#packages_to_install_via_snap[@]} -gt 0 ]; then
        color_echo YELLOW "Installing ${#packages_to_install_via_snap[@]} snap packages..."
        local failed_snap_packages=()
        for package in "${packages_to_install_via_snap[@]}"; do
            color_echo CYAN "Installing $package via snap..."
            local install_output
            local install_status
            
            # Try installing, checking if classic is needed
            if requires_classic_confinement "$package"; then
                install_output=$(sudo snap install --classic "$package" 2>&1)
                install_status=$?
            else
                install_output=$(sudo snap install "$package" 2>&1)
                install_status=$?
                
                # If it failed and error mentions classic confinement, retry with --classic
                if [ $install_status -ne 0 ] && echo "$install_output" | grep -qi "classic confinement"; then
                    color_echo YELLOW "  -> Retrying $package with --classic flag..."
                    install_output=$(sudo snap install --classic "$package" 2>&1)
                    install_status=$?
                fi
            fi
            
            if [ $install_status -eq 0 ]; then
                color_echo GREEN "  -> Successfully installed $package via snap"
            else
                # Show the error message
                echo "$install_output" | grep -v "^$" || true
                color_echo YELLOW "  -> Failed to install $package via snap, will try apt"
                failed_snap_packages+=("$package")
            fi
        done
        
        # Try to install failed snap packages via apt if available
        if [ ${#failed_snap_packages[@]} -gt 0 ]; then
            local packages_to_install_via_apt_fallback=()
            for package in "${failed_snap_packages[@]}"; do
                if ! is_installed_via_apt "$package" && ! is_installed_via_snap "$package"; then
                    packages_to_install_via_apt_fallback+=("$package")
                fi
            done
            if [ ${#packages_to_install_via_apt_fallback[@]} -gt 0 ]; then
                color_echo YELLOW "Installing ${#packages_to_install_via_apt_fallback[@]} packages via apt (snap fallback)..."
                sudo apt-get install -y -qq "${packages_to_install_via_apt_fallback[@]}" || true
            fi
        fi
    fi
}

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