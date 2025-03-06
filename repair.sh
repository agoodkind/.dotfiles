#!/usr/bin/env bash

# exit on error
set -e

timestamp=$(date +"%Y%m%d_%H%M%S")

export DOTDOTFILES="$(dirname "$(readlink -f "$0")")"

printf "DOTDOTFILES: %s\n" "$DOTDOTFILES"

BACKUPS_PATH="$DOTDOTFILES/backups"
mkdir -p "$BACKUPS_PATH"

# go through all files in $DOTDOTFILES/home and create symlinks in $HOME
# make a backup of each file if it exists
files=$(find "$DOTDOTFILES/home" -type f)
for source_file in $files; do
    file_name=$(basename "$source_file")
    home_file=$HOME/$file_name
    
    printf "\n\n"

    if [ -a "$home_file" ]; then
        backup_file="$BACKUPS_PATH/$file_name.bak-$timestamp"
        printf "Backing up %s to %s\n" "$home_file" "$backup_file"
        cp -L "$home_file" "$backup_file"
        rm "$home_file"
    fi

    printf "Creating symlink from %s to %s\n" "$source_file" "$home_file"
    ln -sfv "$source_file" "$home_file"
done

printf "\nUpdating plugins and submodules\n"
git submodule update --init --recursive


printf "\nRun 'source \"%s/.zshrc\"' to apply changes or restart your terminal\n" "$HOME"