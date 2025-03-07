#!/usr/bin/env bash

set -e 

timestamp=$(date +"%Y%m%d_%H%M%S")

export DOTDOTFILES="$(dirname "$(readlink -f "$0")")"

printf "DOTDOTFILES: %s\n" "$DOTDOTFILES"

BACKUPS_PATH="$DOTDOTFILES/backups/$timestamp"
mkdir -p "$BACKUPS_PATH"

# go through all files in $DOTDOTFILES/home and create symlinks in $HOME
# make a backup of each file if it exists
files=$(find "$DOTDOTFILES/home" -type f)
for source_file in $files; do
    relative_path=$(realpath --relative-to="$DOTDOTFILES/home" --no-symlinks "$source_file")
    backup_file="$BACKUPS_PATH/$relative_path.bak"
    home_file=$HOME/$relative_path
    
    printf "\n\n"

    if [ -a "$home_file" ]; then
        
        printf "Backing up %s -> %s\n" "$home_file" "$backup_file"

        mkdir -p "$(dirname "$backup_file")"
        cp -Hr "$home_file" "$backup_file"
    fi

    printf "Symlink created: "
    ln -sfv "$source_file" "$home_file"
done

printf "\nUpdating plugins and submodules\n"
git submodule update --init --recursive


printf "\nRun 'source \"%s/.zshrc\"' to apply changes or restart your terminal\n" "$HOME"