#!/usr/bin/env bash
# Dynamically execute all scripts in the dotfiles motd directory

DOTFILES_DIR="${DOTFILES_DIR:-$HOME/.dotfiles}"
MOTD_DIR="$DOTFILES_DIR/lib/motd"

# Check if directory exists
if [ ! -d "$MOTD_DIR" ]; then
    exit 0
fi

# Execute all files in the motd directory in sorted order
for script in "$MOTD_DIR"/*; do
    if [ -f "$script" ] && [ -x "$script" ]; then
        "$script"
    elif [ -f "$script" ]; then
        # If not executable, try to execute with bash
        bash "$script"
    fi
done
