#!/usr/bin/env bash
# Dynamically execute all scripts in the motd directory

MOTD_DIR="$(dirname "${BASH_SOURCE[0]}")/motd"

# Check if directory exists
if [ ! -d "$MOTD_DIR" ]; then
    echo "Error: MOTD directory not found at $MOTD_DIR"
    exit 1
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
