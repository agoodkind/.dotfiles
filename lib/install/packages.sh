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

