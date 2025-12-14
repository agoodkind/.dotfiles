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
	jq
	lazygit
	less
	most
	moreutils
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
	net-tools
	nodejs
	openssh-client
	openssh-server
    parted
	postfix
	rbenv
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
	paper
	pnpm
	ripgrep-all
	ssh-copy-id
	wireguard-go
    wireguard-tools
    discord
)

# Brew casks: [cask-name]="App Name" (empty = CLI/font, no .app)
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

# Centralized package name mappings
# Format: PACKAGE_MAP[package:type] = "mapped-name"
# Types: apt, snap, cmd
declare -A PACKAGE_MAP

# Initialize package mappings
# apt mappings
PACKAGE_MAP[ack:apt]="ack-grep"
PACKAGE_MAP[fd:apt]="fd-find"
PACKAGE_MAP[openssh:apt]="openssh-client openssh-server"

# snap mappings
PACKAGE_MAP[neovim:snap]="nvim"

# Get app name for a cask (returns empty if unknown or no .app)
get_cask_app_name() {
    local cask="$1"
    if [[ -v "BREW_CASKS[$cask]" ]]; then
        local name="${BREW_CASKS[$cask]}"
        [[ -n "$name" ]] && echo "${name}.app"
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
