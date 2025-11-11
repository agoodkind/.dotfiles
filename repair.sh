#!/usr/bin/env bash
set -euo pipefail

printf "\nUpdating plugins and submodules\n\n"
# can't use config here since we don't know if its been defined yet
(cd "$DOTDOTFILES" && git pull)
(cd "$DOTDOTFILES" && git submodule update --init --recursive)

timestamp=$(date +"%Y%m%d_%H%M%S")

DOTDOTFILES="$(dirname "$(readlink -f "$0")")"
export DOTDOTFILES

printf "DOTDOTFILES: %s\n\n" "$DOTDOTFILES"

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

# go through all files in $DOTDOTFILES/home and create symlinks in $HOME
# make a backup of each file if it exists
files=$(find "$DOTDOTFILES/home" -type f)
printf "\nLinking dotfiles to home directory\n"
for source_file in $files; do
    relative_path=$(realpath_cmd --no-symlinks --relative-to="$DOTDOTFILES/home" "$source_file")
    backup_file="$BACKUPS_PATH/$relative_path.bak"
    home_file=$HOME/$relative_path

    if [ -e "$home_file" ]; then
        mkdir -p "$(dirname "$backup_file")"
        cp -Hr "$home_file" "$backup_file"
    fi
    
    mkdir -p "$(dirname "$home_file")"
    ln -sf "$source_file" "$home_file"
    printf "\tLinked: %s\n" "$relative_path"
done

# Symlink all .sh scripts to ~/.local/bin without .sh extension
printf "\nLinking scripts to ~/.local/bin\n"
rm -rf "$HOME/.local/bin/scripts" 2>/dev/null || true
mkdir -p "$HOME/.local/bin/scripts"
scripts=$(find "$DOTDOTFILES/lib/scripts" -maxdepth 1 -type f -name "*.sh")
for script in $scripts; do
    script_name=$(basename "$script" .sh)
    target="$HOME/.local/bin/scripts/$script_name"

    ln -sf "$script" "$target"
    chmod +x "$script"

    printf "\tLinked script: %s\n" "$script_name"
done

# remove zcompdump files only if ZSH_COMPDUMP is set
if [ -n "${ZSH_COMPDUMP:-}" ]; then
    printf "\nRemoving zcompdump file: %s\n" "$ZSH_COMPDUMP"
    rm -f "$ZSH_COMPDUMP"
fi

# for macOS clean up brew
if is_macos; then
    printf "\nCleaning up Homebrew\n"
    brew cleanup
fi

printf "\n.zshrc has been repaired and relinked\n"
printf "\nRun 'source \"%s/.zshrc\"' to apply changes or restart your terminal\n" "$HOME"
