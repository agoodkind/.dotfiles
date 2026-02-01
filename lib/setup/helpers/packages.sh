# Centralized package list for both apt and brew installations
# NOTE: Requires bash 4.0+ (modern bash with associative arrays)

# Packages that should always be installed via snap (on Linux)
export SNAP_PACKAGES=(
	neovim
	fx
)

# Common packages across both package managers
# Note: Some packages (atuin, procs, starship, tokei, xh, tree-sitter-cli)
# are in CARGO_PACKAGES for Linux since they're not in apt
export COMMON_PACKAGES=(
	ack
	ansible
	ansible-lint
	aria2
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
	less
	most
	moreutils
	msmtp
	ncdu
	openssh
	pandoc
	pigz
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
	tealdeer
	thefuck
	tree
	tshark
	vim
	watch
	wget
	zoxide
	zsh
	yq
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
	rsyslog
	sudo
	speedtest-cli
	ufw
	wireguard
)

# Brew-specific packages (different names or brew-only)
# Includes packages that are cargo-only on Linux but available via brew
export BREW_SPECIFIC=(
	ast-grep
	bandwhich
	bottom
	curlie
	doggo
	dust
	fx
	gitui
	glow
	lazygit
	mdless
	mitmproxy
	navi
	node
	nvim
	pnpm
	procs
	ripgrep-all
	ssh-copy-id
	tokei
	tree-sitter
	wireguard-go
	wireguard-tools
)

# Cargo packages (installed via cargo install)
# These are packages not available via apt on Linux
export CARGO_PACKAGES=(
)

# Go packages (installed via go install)
# Format: package-name=install-path
declare -A GO_PACKAGES=(
	[lazygit]="github.com/jesseduffield/lazygit@latest"
)

# Cargo packages requiring git installation - associative arrays
# Format: [package]="url|features"
declare -A CARGO_GIT_PACKAGES=(
)

# Get git installation details for a cargo package
# Returns: "url|features" or empty if not a git package
function get_cargo_git_details() {
	local package="$1"
	if [[ -n "${CARGO_GIT_PACKAGES[$package]:-}" ]]; then
		echo "${CARGO_GIT_PACKAGES[$package]}"
		return 0
	fi
	return 1
}

# Brew casks - associative array mapping cask name to app name
# Empty value means CLI-only or font (no .app to check)
declare -A BREW_CASKS=(
	[1password]="1Password"
	[1password-cli]=""
	[iterm2]="iTerm"
	[keycastr]="KeyCastr"
	[visual-studio-code]="Visual Studio Code"
	[google-chrome]="Google Chrome"
	[font-jetbrains-mono-nerd-font]=""
	[font-jetbrains-mono]=""
	[cyberduck]="Cyberduck"
	[utm]="UTM"
	[vlc]="VLC"
	[stats]="Stats"
	[xcodes-app]="Xcodes"
	[pingplotter]="PingPlotter"
)

# Get app name for a cask (returns empty if unknown or no .app)
function get_cask_app_name() {
	local cask="$1"
	local app_name="${BREW_CASKS[$cask]:-}"
	if [[ -n "$app_name" ]]; then
		echo "${app_name}.app"
		return 0
	fi
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
		rg:apt) echo "ripgrep" ;;
		openssh:apt) echo "openssh-client openssh-server" ;;
		neovim:snap) echo "nvim" ;;
		tshark:brew) echo "wireshark" ;;
		*) echo "$pkg" ;;
	esac
}

# APT PPAs to add before installing packages
# Format: [package_name]="ppa:user/repo"
# The PPA will only be added if the package is in the install list
declare -A APT_PPAS=(
	[fastfetch]="ppa:zhangsongcui3371/fastfetch"
)

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
