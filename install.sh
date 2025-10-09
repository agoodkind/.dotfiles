#!/usr/bin/env bash

# exit on error
set -e

export DOTDOTFILES="$HOME/.dotfiles"

chmod +x "$DOTDOTFILES/repair.sh"
chmod +x "$DOTDOTFILES/lib/install/git.sh"
chmod +x "$DOTDOTFILES/lib/install/apt.sh"
chmod +x "$DOTDOTFILES/lib/install/mac.sh"
chmod +x "$DOTDOTFILES/lib/install/brew.sh"

mkdir -p "$HOME/.ssh/sockets"

# set up git
"$DOTDOTFILES/lib/install/git.sh"
ssh-add "$HOME/.ssh/id_ed25519"

# run repair.sh
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

# install zinit
bash -c "$(curl --fail --show-error --silent --location https://raw.githubusercontent.com/zdharma-continuum/zinit/HEAD/scripts/install.sh)"


