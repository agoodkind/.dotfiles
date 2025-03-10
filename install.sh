#! /usr/bin/env zsh
# shellcheck shell=bash

# exit on error
set -e

DOTDOTFILES="$HOME/.dotfiles"

chmod +x "$DOTDOTFILES/repair.sh"
chmod +x "$DOTDOTFILES/lib/install/apt.sh"
chmod +x "$DOTDOTFILES/lib/install/mac.sh"
chmod +x "$DOTDOTFILES/lib/install/brew.sh"

# include the global gitconfig
git --git-dir="$DOTDOTFILES"/.git \
    --work-tree="$DOTDOTFILES" \
    config --global include.path "$DOTDOTFILES/lib/.gitconfig_incl"

SSH_SIGNING_KEY="$HOME/.ssh/id_ed25519"
# get ssh signing key (path or paste)
vared -p "Enter your ssh signing key: " SSH_SIGNING_KEY
# if path, check if file exists
if [[ "$SSH_SIGNING_KEY" == *"/"* ]]; then
    if [ -f "$SSH_SIGNING_KEY" ]; then
        git --git-dir="$DOTDOTFILES"/.git \
            --work-tree="$DOTDOTFILES" \
            config --global user.signingkey "$SSH_SIGNING_KEY"
    else
        echo "File does not exist"
        exit 1
    fi
fi

# if linux, install apt
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    echo "Linux detected"
    echo "Installing apt packages"
    "$DOTDOTFILES/lib/install/apt.sh"
fi

# run mac last because it calls brew which takes forever
# if mac, install brew
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "macOS detected"
    "$DOTDOTFILES/lib/install/mac.sh"
fi

# run repair.sh
"$DOTDOTFILES/repair.sh"