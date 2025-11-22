#!/usr/bin/env bash

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

# Map common package names to APT package names
# Packages that map to something already in APT_SPECIFIC will be skipped later
map_to_apt_name() {
	local package="$1"
	case "$package" in
		ack) echo "ack-grep" ;;
		neovim) echo "neovim" ;;
		openssh) echo "openssh-client openssh-server" ;;
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

# Add all APT_SPECIFIC packages (no need to check for duplicates since we excluded them above)
ALL_APT_PACKAGES+=("${APT_SPECIFIC[@]}")

export ALL_APT_PACKAGES
