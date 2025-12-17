# Centralized package list for both apt and brew installations
# NOTE: This file must be bash 3.2 compatible (macOS default)

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
	atuin
	bat
	bash
	btop
	coreutils
	curl
	duf
	eza
	fail2ban
	fastfetch
	fd
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
	hyperfine
	imagemagick
	ipv6calc
	jq
	lazygit
	less
	most
	moreutils
	msmtp
	ncdu
	openssh
	pandoc
	pigz
	procs
	pv
	python3
	rename
	rg
	rsync
	ruby
	screen
	sd
	sipcalc
	smartmontools
	sshuttle
	starship
	tealdeer
	thefuck
	tldr
	tokei
	tree
	vim
	watch
	wget
	xh
	zoxide
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
	ipcalc-ng
	ipcalc
	locales
	msmtp-mta
	net-tools
	nodejs
	openssh-client
	openssh-server
	parted
	rbenv
	ripcalc
	rsyslog
	sasl-xoauth2
	sudo
	speedtest-cli
	ufw
	wireguard
)

# Brew-specific packages (different names or brew-only)
export BREW_SPECIFIC=(
	ast-grep
	bandwhich
	bottom
	curlie
	doggo
	dust
	gitui
	glow
	mdless
	navi
	node
	nvim
	pnpm
	ripgrep-all
	ssh-copy-id
	wireguard-go
	wireguard-tools
)

# Brew casks - using parallel arrays for bash 3.2 compatibility
# BREW_CASK_NAMES[i] corresponds to BREW_CASK_APPS[i]
# Empty app name means CLI-only or font (no .app to check)
export BREW_CASK_NAMES=(
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

export BREW_CASK_APPS=(
	"1Password"
	""
	"iTerm"
	"KeyCastr"
	"Visual Studio Code"
	"Google Chrome"
	""
	""
	"Cyberduck"
	"UTM"
	"VLC"
	"Stats"
	"Xcodes"
	"PingPlotter"
)

# Get app name for a cask (returns empty if unknown or no .app)
# Bash 3.2 compatible - uses parallel arrays
get_cask_app_name() {
	local cask="$1"
	local i
	for i in "${!BREW_CASK_NAMES[@]}"; do
		if [[ "${BREW_CASK_NAMES[$i]}" == "$cask" ]]; then
			local name="${BREW_CASK_APPS[$i]}"
			[[ -n "$name" ]] && echo "${name}.app"
			return 0
		fi
	done
	return 1
}

# Get mapped package name for a specific package manager
# Usage: get_package_name "ack" "apt" -> "ack-grep"
get_package_name() {
	local pkg="$1"
	local type="$2"
	
	case "${pkg}:${type}" in
		ack:apt) echo "ack-grep" ;;
		fd:apt) echo "fd-find" ;;
		openssh:apt) echo "openssh-client openssh-server" ;;
		neovim:snap) echo "nvim" ;;
		*) echo "$pkg" ;;
	esac
}

# Check if a package is in an array
is_in_array() {
	local search="$1"
	shift
	local item
	for item in "$@"; do
		if [[ "$item" == "$search" ]]; then
			return 0
		fi
	done
	return 1
}
