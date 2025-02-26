#!/usr/bin/env bash

# exit on error
set -e

export DOTDOTFILES="$(dirname "$(readlink -f "$0")")"

# go through all files in $DOTDOTFILES/home and create symlinks in $HOME
# make a backup of each file if it exists
for file in "$DOTDOTFILES/home"/*; do
    echo "Creating symlink from $file to $HOME/$(basename "$file")"
    if [ -f "$HOME/$(basename "$file")" ]; then
        echo "Backing up $HOME/$(basename "$file") to $BACKUPS_PATH/.$(basename "$file").bak-$timestamp"
        mv "$HOME/$(basename "$file")" "$BACKUPS_PATH/.$(basename "$file").bak-$timestamp"
    fi
    ln -sF "$file" "$HOME/$(basename "$file")"
done

echo "Updating plugins and submodules"
git submodule update --init --recursive

timestamp=$(date +"%Y%m%d_%H%M%S")

BACKUPS_PATH="$DOTDOTFILES/backups"
mkdir -p "$BACKUPS_PATH"

echo "Run 'source \"$ZSHRC_HOME\"' to apply changes or restart your terminal"