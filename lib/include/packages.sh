# Centralized package list for both apt and brew installations

# Packages that should always be installed via snap (on Linux)
export SNAP_PACKAGES=(
	neovim
)

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
    fzf
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
	openssh
	pandoc
    pigz
    pv
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
	golang-go
    golang
	gpg
	locales
	net-tools
	nodejs
	openssh-client
	openssh-server
    parted
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
    discord
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

# Centralized package name mappings
# Format: PACKAGE_MAP[package:type] = "mapped-name"
# Types: apt, snap, cmd
declare -A PACKAGE_MAP

# Initialize package mappings
# apt mappings
PACKAGE_MAP[ack:apt]="ack-grep"
PACKAGE_MAP[openssh:apt]="openssh-client openssh-server"

# snap mappings
PACKAGE_MAP[neovim:snap]="nvim"

# Map common package names to APT package names
# Packages that map to something already in APT_SPECIFIC will be skipped later
map_to_apt_name() {
	local package="$1"
	if [[ -n "${PACKAGE_MAP[$package:apt]}" ]]; then
		echo "${PACKAGE_MAP[$package:apt]}"
	else
		echo "$package"
	fi
}

# Map package names to their snap equivalents (if different)
map_to_snap_name() {
	local package="$1"
	if [[ -n "${PACKAGE_MAP[$package:snap]}" ]]; then
		echo "${PACKAGE_MAP[$package:snap]}"
	else
		echo "$package"
	fi
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
    # Also check channels section for classic (some snaps show it there, e.g., "latest/beta: 3.30 ... classic")
    if echo "$info_output" | grep -qE "latest/[^:]+:\s+[0-9].*\s+classic"; then
        return 0  # true - requires classic
    fi
    return 1  # false - doesn't require classic
}


# Install packages if any are missing
install_packages() {
    debug_echo "install_packages() called with $# package(s)"
    
    # Separate packages into apt and snap lists
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
            
            # Check if already installed via snap
            if is_installed_via_snap "$snap_name"; then
                debug_echo "  SKIP: $snap_name already installed via snap"
            else
                debug_echo "  Added $snap_name to snap install list"
                packages_to_install_via_snap+=("$snap_name")
            fi
        else
            debug_echo "  Package $package will be installed via apt"
            # Check if already installed via apt
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
                # Handle multi-word output (like openssh -> openssh-client openssh-server)
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
            
            # Check if classic confinement is needed
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

export ALL_APT_PACKAGES
