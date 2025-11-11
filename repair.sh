#!/usr/bin/env bash
set -euo pipefail

printf "\nUpdating plugins and submodules\n\n"
# can't use config here since we don't know if its been defined yet
git --git-dir="$DOTDOTFILES"/.git --work-tree="$DOTDOTFILES" submodule update --init --recursive

timestamp=$(date +"%Y%m%d_%H%M%S")

DOTDOTFILES="$(dirname "$(readlink -f "$0")")"
export DOTDOTFILES

printf "DOTDOTFILES: %s\n" "$DOTDOTFILES"

BACKUPS_PATH="$DOTDOTFILES/backups/$timestamp"
mkdir -p "$BACKUPS_PATH"

is_macos() {
    [[ "$OSTYPE" == "darwin"* ]]
}

realpath_cmd() {
    if is_macos && command -v grealpath >/dev/null; then
        grealpath "$@"
    else
        realpath "$@"
    fi
}

# for macOS clean up brew
if is_macos; then
    brew cleanup
fi

# go through all files in $DOTDOTFILES/home and create symlinks in $HOME
# make a backup of each file if it exists
files=$(find "$DOTDOTFILES/home" -type f)
for source_file in $files; do
    relative_path=$(realpath_cmd --no-symlinks --relative-to="$DOTDOTFILES/home" "$source_file")
    backup_file="$BACKUPS_PATH/$relative_path.bak"
    home_file=$HOME/$relative_path

    if [ -e "$home_file" ]; then
        mkdir -p "$(dirname "$backup_file")"
        cp -Hr "$home_file" "$backup_file"
        printf "\tBackup created: %s\n" "$relative_path"
    fi
    
    ln -sfv "$source_file" "$home_file"
    printf "\tLinked: %s\n" "$relative_path"
done

# Symlink all .sh scripts to ~/.local/bin without .sh extension
printf "\nLinking scripts to ~/.local/bin\n"
mkdir -p "$HOME/.local/bin"
scripts=$(find "$DOTDOTFILES/lib/scripts" -maxdepth 1 -type f -name "*.sh")
for script in $scripts; do
    script_name=$(basename "$script" .sh)
    target="$HOME/.local/bin/$script_name"
    ln -sfv "$script" "$target"
    chmod +x "$script"
    printf "\tLinked script: %s\n" "$script_name"
done

# remove zcompdump files
rm -f "$ZSH_COMPDUMP"

printf "\n.zshrc has been repaired and relinked\n"
printf "\nRun 'source \"%s/.zshrc\"' to apply changes or restart your terminal\n" "$HOME"
