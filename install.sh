#!/usr/bin/env bash

# exit on error
set -euo pipefail

export DOTDOTFILES="$HOME/.dotfiles"

chmod +x "$DOTDOTFILES/repair.sh"
chmod +x "$DOTDOTFILES/lib/install/git.sh"
chmod +x "$DOTDOTFILES/lib/install/apt.sh"
chmod +x "$DOTDOTFILES/lib/install/mac.sh"
chmod +x "$DOTDOTFILES/lib/install/brew.sh"

echo "Creating SSH sockets directory"
mkdir -p "$HOME/.ssh/sockets"

echo "Starting SSH agent"
eval $(ssh-agent -s)

echo "Adding SSH keys"
# Auto-add SSH key if not already in agent
ssh-add -l &>/dev/null || ssh-add ~/.ssh/id_ed25519 &>/dev/null

# set up git
echo "Setting up git configuration"
"$DOTDOTFILES/lib/install/git.sh"

# run repair.sh
echo "Running repair script"
"$DOTDOTFILES/repair.sh"

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
    echo "Installing mac packages"
    "$DOTDOTFILES/lib/install/mac.sh"
fi


echo "Installing submodule"
git submodule update --init --recursive lib/zinit
git submodule update --init --recursive lib/scripts
git submodule update --init --recursive home/.ssh