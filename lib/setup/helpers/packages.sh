# Centralized package list for both apt and brew installations
# NOTE: Requires bash 4.0+ (modern bash with associative arrays)

# Packages that should always be installed via snap (on Linux)
export SNAP_PACKAGES=(
	neovim
	fx
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
	fx
	gitui
	glow
	mdless
	mitmproxy
	navi
	node
	nvim
	pnpm
	ripgrep-all
	ssh-copy-id
	wireguard-go
	wireguard-tools
)

# Cargo packages (installed via cargo install)
export CARGO_PACKAGES=(
	async-cmd
	cloudflare-speed-cli
)

# Cargo packages requiring git installation - associative arrays
# Format: [package]="url|features"
declare -gA CARGO_GIT_PACKAGES=(
	[cloudflare-speed-cli]="https://github.com/kavehtehrani/cloudflare-speed-cli|tui"
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
declare -gA BREW_CASKS=(
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
