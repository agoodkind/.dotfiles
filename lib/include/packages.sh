# Centralized package list for both apt and brew installations

# Common packages across both package managers
export COMMON_PACKAGES=(
	ack
	ansible
	ansible-lint
	aria2
	bat
	bash
	coreutils
	curl
	eza
	fail2ban
	fastfetch
	ffmpeg
	figlet
	fping
	gh
	git
	git-delta
	git-lfs
	gping
	grc
	grep
	htop
	imagemagick
	jq
	less
	most
	moreutils
	neovim
	openssh
	pandoc
	python3
	rename
	rsync
	ruby
	screen
	smartmontools
	sshuttle
	thefuck
	tree
	vim
	watch
	wget
	zsh
	yq
	tree-sitter-cli
)

# APT-specific packages (different names or apt-only)
export APT_SPECIFIC=(
	ack-grep
	fzf
	golang-go
	gpg
	locales
	net-tools
	nodejs
	openssh-client
	openssh-server
	postfix
	rbenv
	rsyslog
	sasl-xoauth2
	speedtest-cli
	wireguard
)

# Brew-specific packages (different names or brew-only)
export BREW_SPECIFIC=(
	ast-grep
	glow
	mdless
	node
	nvim
	paper
	pnpm
	ssh-copy-id
	wireguard-tools
	zoxide
)

# Brew cask applications
export BREW_CASKS=(
	1password
	1password-cli
	iterm2
	keycastr
	visual-studio-code
	google-chrome
	font-jetbrains-mono-nerd-font
	font-jetbrains-mono
	cyberduck
	utm
	vlc
	stats
	xcodes-app
	pingplotter
)

# Add all APT_SPECIFIC packages (no need to check for duplicates since we excluded them above)
ALL_APT_PACKAGES+=("${APT_SPECIFIC[@]}")

# Map common package names to APT package names
# Packages that map to something already in APT_SPECIFIC will be skipped later
map_to_apt_name() {
	local package="$1"
	case "$package" in
		ack) echo "ack-grep" ;;
		openssh) echo "openssh-client openssh-server" ;;
		*) echo "$package" ;;
	esac
}

# Map package names to their snap equivalents (if different)
map_to_snap_name() {
    local package="$1"
    case "$package" in
        neovim) echo "nvim" ;;
        *) echo "$package" ;;
    esac
}

# Map package names to their command names (if different)
map_to_cmd_name() {
    local package="$1"
    case "$package" in
        neovim) echo "nvim" ;;
        bat) echo "batcat" ;;
        *) echo "$package" ;;
    esac
}

# Check if a package is in an array
is_in_array() {
	local search="$1"
	shift
	local array=("$@")
	for item in "${array[@]}"; do
		if [[ "$item" == "$search" ]]; then
			return 0
		fi
	done
	return 1
}

# Build ALL_APT_PACKAGES array with mapped names
ALL_APT_PACKAGES=()

# Add mapped common packages (skip duplicates that are in APT_SPECIFIC)
for package in "${COMMON_PACKAGES[@]}"; do
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

is_available_via_apt() {
    apt-cache show "$1" &>/dev/null
    return $?
}

# Check if a command/binary is already available in PATH
# This helps avoid installing via apt when already installed via snap with different name
command_available() {
    command -v "$1" &>/dev/null
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


# Install packages if any are missing
install_packages() {
    debug_echo "install_packages() called with $# package(s)"
    
    # Build list of packages to install
    local packages_to_install_via_apt=()
    local packages_to_remove_via_apt=()
    local packages_to_install_via_snap=()

    # Ask if user wants to use snap for available packages
    local use_snap=false
    local remove_from_apt=false

    debug_echo "Prompting for snap usage preference"
    read_with_default "Use snap for available packages? (Y/n) " "Y"
    debug_echo "User reply for snap usage: '$REPLY'"
    if [[ -z "$REPLY" || $REPLY =~ ^[Yy]$ ]]; then
        color_echo GREEN "OK will use snap for available packages"
        use_snap=true
        debug_echo "use_snap set to true"
        read_with_default "Remove packages from apt if available via snap? [Y/n] " "Y"
        debug_echo "User reply for remove_from_apt: '$REPLY'"
        if [[ -z "$REPLY" || $REPLY =~ ^[Yy]$ ]]; then
            color_echo GREEN "OK will remove packages from apt if available via snap"
            remove_from_apt=true
            debug_echo "remove_from_apt set to true"
        else
            debug_echo "remove_from_apt remains false"
        fi
    else
        debug_echo "use_snap remains false"
    fi

    debug_echo "Configuration: use_snap=$use_snap, remove_from_apt=$remove_from_apt"
    
    # DEBUG: Show what packages we're processing
    color_echo YELLOW "Processing $# packages..."
    debug_echo "Package list: $*"
    
    for package in "$@"; do
        debug_echo "--- Processing package: $package ---"
        
        # Determine command name and snap name (may differ from package name)
        local snap_name=$(map_to_snap_name "$package")
        local cmd_name=$(map_to_cmd_name "$package")
        debug_echo "Mapped names - snap: '$snap_name', cmd: '$cmd_name'"
        
        # Perform all checks upfront
        local installed_via_apt=false
        local available_via_apt=false
        local installed_via_snap_pkg=false
        local installed_via_snap_mapped=false
        local available_via_snap=false
        local cmd_available=false
        
        debug_echo "Running checks for $package..."
        is_installed_via_apt "$package" && installed_via_apt=true
        debug_echo "  installed_via_apt: $installed_via_apt"
        
        is_available_via_apt "$package" && available_via_apt=true
        debug_echo "  available_via_apt: $available_via_apt"
        
        is_installed_via_snap "$package" && installed_via_snap_pkg=true
        debug_echo "  installed_via_snap (package name): $installed_via_snap_pkg"
        
        is_installed_via_snap "$snap_name" && installed_via_snap_mapped=true
        debug_echo "  installed_via_snap (mapped name): $installed_via_snap_mapped"
        
        is_available_via_snap "$package" && available_via_snap=true
        debug_echo "  available_via_snap: $available_via_snap"
        
        command_available "$cmd_name" && cmd_available=true
        debug_echo "  cmd_available: $cmd_available"
        
        # Skip if already installed via snap (either name)
        if [[ "$installed_via_snap_pkg" == "true" || "$installed_via_snap_mapped" == "true" ]]; then
            debug_echo "SKIP: Package already installed via snap, continuing to next package"
            continue
        fi
        
        debug_echo "Entering decision logic: use_snap=$use_snap"
        
        # Truth table evaluation based on use_snap and remove_from_apt flags
        if [[ "$use_snap" == "true" ]]; then
            debug_echo "  Branch: use_snap is true"
            if [[ "$available_via_snap" == "true" ]]; then
                debug_echo "    Branch: available_via_snap is true"
                # Available via snap
                if [[ "$installed_via_apt" == "true" ]]; then
                    debug_echo "      Branch: installed_via_apt is true"
                    # Installed via apt, available via snap
                    if [[ "$remove_from_apt" == "true" ]]; then
                        debug_echo "        Action: Replace apt with snap (remove_from_apt=true)"
                        # Replace apt with snap
                        packages_to_remove_via_apt+=("$package")
                        packages_to_install_via_snap+=("$package")
                        debug_echo "        Added to remove_apt and install_snap arrays"
                    elif [[ "$cmd_available" == "false" ]]; then
                        debug_echo "        Action: Install via snap (cmd not available)"
                        # Command not available, install via snap
                        packages_to_install_via_snap+=("$package")
                        debug_echo "        Added to install_snap array"
                    else
                        debug_echo "        SKIP: cmd is available, no action needed"
                    fi
                elif [[ "$cmd_available" == "false" ]]; then
                    debug_echo "      Branch: Not installed, cmd not available"
                    # Not installed, command not available, install via snap
                    packages_to_install_via_snap+=("$package")
                    debug_echo "      Added to install_snap array"
                else
                    debug_echo "      SKIP: cmd is available, no action needed"
                fi
            elif [[ "$installed_via_apt" == "false" && "$available_via_apt" == "true" && "$cmd_available" == "false" ]]; then
                debug_echo "    Branch: Not available via snap, but available via apt and not installed"
                # Not available via snap, but available via apt and not installed
                packages_to_install_via_apt+=("$package")
                debug_echo "    Added to install_apt array"
            else
                debug_echo "    SKIP: Conditions not met for apt install (installed=$installed_via_apt, available=$available_via_apt, cmd=$cmd_available)"
            fi
        elif [[ "$installed_via_apt" == "false" && "$available_via_apt" == "true" && "$cmd_available" == "false" ]]; then
            debug_echo "  Branch: Not using snap, install via apt if available and needed"
            # Not using snap, install via apt if available and needed
            packages_to_install_via_apt+=("$package")
            debug_echo "  Added to install_apt array"
        else
            debug_echo "  SKIP: No action needed (installed=$installed_via_apt, available=$available_via_apt, cmd=$cmd_available)"
        fi
        debug_echo "Finished processing $package"
    done
    
    debug_echo "Loop complete. Summary:"
    debug_echo "  packages_to_install_via_apt: ${#packages_to_install_via_apt[@]} packages"
    debug_echo "  packages_to_remove_via_apt: ${#packages_to_remove_via_apt[@]} packages"
    debug_echo "  packages_to_install_via_snap: ${#packages_to_install_via_snap[@]} packages"

    if [ ${#packages_to_install_via_apt[@]} -gt 0 ]; then
        debug_echo "Installing ${#packages_to_install_via_apt[@]} apt packages: ${packages_to_install_via_apt[*]}"
        color_echo YELLOW "Installing ${#packages_to_install_via_apt[@]} apt packages..."
        sudo apt-get install -y -qq "${packages_to_install_via_apt[@]}"
        debug_echo "apt-get install completed with exit code: $?"
    else
        debug_echo "No apt packages to install"
    fi

    if [ ${#packages_to_remove_via_apt[@]} -gt 0 ]; then
        debug_echo "Removing ${#packages_to_remove_via_apt[@]} apt packages: ${packages_to_remove_via_apt[*]}"
        color_echo YELLOW "Removing ${#packages_to_remove_via_apt[@]} apt packages..."
        sudo apt-get remove -y -qq "${packages_to_remove_via_apt[@]}"
        debug_echo "apt-get remove completed with exit code: $?"
    else
        debug_echo "No apt packages to remove"
    fi

    if [ ${#packages_to_install_via_snap[@]} -gt 0 ]; then
        debug_echo "Installing ${#packages_to_install_via_snap[@]} snap packages: ${packages_to_install_via_snap[*]}"
        color_echo YELLOW "Installing ${#packages_to_install_via_snap[@]} snap packages..."
        local failed_snap_packages=()
        for package in "${packages_to_install_via_snap[@]}"; do
            debug_echo "--- Installing snap package: $package ---"
            color_echo CYAN "Installing $package via snap..."
            local install_output
            local install_status
            
            # Try installing, checking if classic is needed
            if requires_classic_confinement "$package"; then
                debug_echo "  Package requires classic confinement, installing with --classic"
                install_output=$(sudo snap install --classic "$package" 2>&1)
                install_status=$?
                debug_echo "  snap install --classic exit code: $install_status"
            else
                debug_echo "  Package does not require classic confinement, installing normally"
                install_output=$(sudo snap install "$package" 2>&1)
                install_status=$?
                debug_echo "  snap install exit code: $install_status"
                
                # If it failed and error mentions classic confinement, retry with --classic
                if [ $install_status -ne 0 ] && echo "$install_output" | grep -qi "classic confinement"; then
                    debug_echo "  Installation failed, error mentions classic confinement, retrying with --classic"
                    color_echo YELLOW "  -> Retrying $package with --classic flag..."
                    install_output=$(sudo snap install --classic "$package" 2>&1)
                    install_status=$?
                    debug_echo "  Retry with --classic exit code: $install_status"
                fi
            fi
            
            if [ $install_status -eq 0 ]; then
                debug_echo "  SUCCESS: $package installed via snap"
                color_echo GREEN "  -> Successfully installed $package via snap"
            else
                debug_echo "  FAILED: $package installation failed, exit code: $install_status"
                debug_echo "  Error output: $install_output"
                # Show the error message
                echo "$install_output" | grep -v "^$" || true
                color_echo YELLOW "  -> Failed to install $package via snap, will try apt"
                failed_snap_packages+=("$package")
                debug_echo "  Added $package to failed_snap_packages array"
            fi
        done
        
        debug_echo "Snap installation loop complete. Failed packages: ${#failed_snap_packages[@]}"
        if [ ${#failed_snap_packages[@]} -gt 0 ]; then
            debug_echo "Failed snap packages: ${failed_snap_packages[*]}"
        fi
        
        # Try to install failed snap packages via apt if available
        if [ ${#failed_snap_packages[@]} -gt 0 ]; then
            debug_echo "Processing ${#failed_snap_packages[@]} failed snap packages for apt fallback"
            local packages_to_install_via_apt_fallback=()
            for package in "${failed_snap_packages[@]}"; do
                debug_echo "  Checking $package for apt fallback..."
                if ! is_installed_via_apt "$package" && ! is_installed_via_snap "$package"; then
                    debug_echo "    Package not installed via apt or snap, adding to fallback list"
                    packages_to_install_via_apt_fallback+=("$package")
                else
                    debug_echo "    Package already installed, skipping fallback"
                fi
            done
            debug_echo "  Fallback packages to install via apt: ${#packages_to_install_via_apt_fallback[@]}"
            if [ ${#packages_to_install_via_apt_fallback[@]} -gt 0 ]; then
                debug_echo "  Installing fallback packages: ${packages_to_install_via_apt_fallback[*]}"
                color_echo YELLOW "Installing ${#packages_to_install_via_apt_fallback[@]} packages via apt (snap fallback)..."
                sudo apt-get install -y -qq "${packages_to_install_via_apt_fallback[@]}" || true
                debug_echo "  apt-get install fallback completed with exit code: $?"
            else
                debug_echo "  No packages need apt fallback installation"
            fi
        else
            debug_echo "No failed snap packages, skipping fallback"
        fi
    else
        debug_echo "No snap packages to install"
    fi
    
    debug_echo "install_packages() completed"
}

export ALL_APT_PACKAGES
