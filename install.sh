#!/usr/bin/env bash

# exit on error
set -e

DOTDOTFILES="$HOME/.dotfiles"

chmod +x "$DOTDOTFILES/repair.sh"
chmod +x "$DOTDOTFILES/lib/install/brew.sh"
chmod +x "$DOTDOTFILES/lib/install/apt.sh"

# set some common git configs
git config --global rerere.enabled true
git config --global push.autoSetupRemote true
git config --global pull.rebase true

# if mac, install brew
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "macOS detected"
    "$DOTDOTFILES/lib/install/mac.sh"
fi

# if linux, install apt
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    echo "Linux detected"
    echo "Installing apt packages"
    "$DOTDOTFILES/lib/install/apt.sh"
fi

# run repair.sh
"$DOTDOTFILES/repair.sh"