#!/usr/bin/env bash

timestamp=$(date +"%Y%m%d_%H%M%S")

DOTDOTFILES="$(dirname "$(readlink -f "$0")")"
export DOTDOTFILES

# Color and emoji setup
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

color_echo() {
    color="$1"; shift
    echo -e "${!color}$*${NC}"
}

color_echo BLUE "🔄  Updating plugins and submodules..."
# can't use config here since we don't know if its been defined yet
(cd "$DOTDOTFILES" && git pull)
(cd "$DOTDOTFILES" && git submodule update --init --recursive)

timestamp=$(date +"%Y%m%d_%H%M%S")

BACKUPS_PATH="$DOTDOTFILES/backups/$timestamp"
mkdir -p "$BACKUPS_PATH"

is_macos() {
    [[ "$OSTYPE" == "darwin"* ]]
}

is_ubuntu() {
    [[ -f /etc/os-release ]] && grep -qiE 'ubuntu|debian' /etc/os-release
}

realpath_cmd() {
    if is_macos && command -v grealpath >/dev/null; then
        grealpath "$@"
    else
        realpath "$@"
    fi
}

printf "\nLinking dotfiles to home directory\n"

# go through all files in $DOTDOTFILES/home and create symlinks in $HOME
# make a backup of each file if it exists
files=$(find "$DOTDOTFILES/home" -type f)
color_echo YELLOW "🔗 Linking dotfiles to home directory..."
for source_file in $files; do
    relative_path=$(realpath_cmd --no-symlinks --relative-to="$DOTDOTFILES/home" "$source_file")
    backup_file="$BACKUPS_PATH/$relative_path.bak"
    home_file=$HOME/$relative_path

    if [ -e "$home_file" ]; then
        mkdir -p "$(dirname "$backup_file")"
        cp -Hr "$home_file" "$backup_file"
        color_echo YELLOW "  💾  Backed up: $relative_path"
    fi
    
    mkdir -p "$(dirname "$home_file")"
    ln -sf "$source_file" "$home_file"
    color_echo GREEN "  🔗  Linked: $relative_path"
done


# Symlink all .sh scripts to ~/.local/bin without .sh extension
color_echo YELLOW "🔗 Linking scripts to ~/.local/bin..."
rm -rf "$HOME/.local/bin/scripts" 2>/dev/null || true
mkdir -p "$HOME/.local/bin/scripts"
scripts=$(find "$DOTDOTFILES/lib/scripts" -maxdepth 1 -type f -name "*.sh")
for script in $scripts; do
    script_name=$(basename "$script" .sh)
    target="$HOME/.local/bin/scripts/$script_name"

    ln -sf "$script" "$target"

    color_echo GREEN "  🔗  Linked script: $script_name"
done


# remove zcompdump files only if ZSH_COMPDUMP is set
if [ -n "${ZSH_COMPDUMP:-}" ]; then
    color_echo YELLOW "🧹  Removing zcompdump file: $ZSH_COMPDUMP"
    rm -f "$ZSH_COMPDUMP"
fi

# for macOS clean up brew
if is_macos; then
    color_echo YELLOW "🧹  Cleaning up Homebrew..."
    brew cleanup

    color_echo BLUE "💡  Running macOS setup script..."
    "$DOTDOTFILES/lib/install/mac.sh" --skip-install
fi

if is_ubuntu; then
    color_echo BLUE "💡  Running Ubuntu setup script..."
    "$DOTDOTFILES/lib/install/ubuntu.sh" --skip-install
fi

color_echo GREEN "✅  .zshrc has been repaired and relinked"