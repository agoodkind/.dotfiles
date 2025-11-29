#!/usr/bin/env bash

timestamp=$(date +"%Y%m%d_%H%M%S")

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Source utilities
source "${DOTDOTFILES}/lib/include/defaults.sh"
source "${DOTDOTFILES}/lib/include/colors.sh"
source "${DOTDOTFILES}/lib/include/packages.sh"

# Parse flags
run_background=false
repair_mode=false
non_interactive=false
for arg in "$@"; do
    case $arg in
        --background|--bg)
            run_background=true
            non_interactive=true
            ;;
        --repair)
            repair_mode=true
            ;;
        --non-interactive)
            non_interactive=true
            ;;
    esac
done

color_echo BLUE "🔄  Updating plugins and submodules..."
# Check if git is locked
if [ -f "$DOTDOTFILES/.git/objects/info/commit-graphs/commit-graph-chain.lock" ]; then
    if [[ "$non_interactive" == "true" ]]; then
        # In non-interactive mode, force unlock
        color_echo YELLOW "🔒  Git is locked, force unlocking..."
        rm -f "$DOTDOTFILES/.git/objects/info/commit-graphs/commit-graph-chain.lock" 2>/dev/null || \
            sudo rm -f "$DOTDOTFILES/.git/objects/info/commit-graphs/commit-graph-chain.lock" 2>/dev/null
        color_echo GREEN "🔓  Git unlocked"
    else
        color_echo RED "🔒  Git is locked, do you want to force unlock it?"
        read_with_default "Unlock? (y/n) " "n"
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            sudo rm -f "$DOTDOTFILES/.git/objects/info/commit-graphs/commit-graph-chain.lock"
            color_echo GREEN "🔓  Git unlocked"
        else
            color_echo RED "🔒  Git is locked, skipping update..."
            exit 1
        fi
    fi
fi

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

# Use github authorized keys to add to ~/.ssh/authorized_keys
wget https://github.com/agoodkind.keys -O "$HOME"/.ssh/authorized_keys.tmp
# append missing keys to ~/.ssh/authorized_keys
touch "$HOME"/.ssh/authorized_keys  # Ensure file exists
while IFS= read -r key || [ -n "$key" ]; do
    if ! grep -q "$key" "$HOME"/.ssh/authorized_keys; then
        echo "$key" >> "$HOME"/.ssh/authorized_keys
    fi
done < "$HOME"/.ssh/authorized_keys.tmp
rm -f "$HOME"/.ssh/authorized_keys.tmp

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

NVIM_DATA="${XDG_DATA_HOME:-$HOME/.local/share}/nvim"
LAZY_DIR="$NVIM_DATA/lazy"

# Aggressive cleanup in repair mode
if [[ "$repair_mode" == "true" ]]; then
    color_echo YELLOW "🔧  Repair mode: aggressive cleanup..."
    
    if [ -d "$LAZY_DIR" ]; then
        find "$LAZY_DIR" -maxdepth 1 -name "*.cloning" -delete 2>/dev/null
        
        # Remove failed/partial plugin directories (empty or missing .git)
        for dir in "$LAZY_DIR"/*/; do
            [ -d "$dir" ] || continue
            if [ ! -d "$dir/.git" ] || [ -z "$(ls -A "$dir" 2>/dev/null)" ]; then
                color_echo YELLOW "  🗑️  Removing incomplete plugin: $(basename "$dir")"
                rm -rf "$dir"
            fi
        done
    fi
    
    if [ -d "$NVIM_DATA" ]; then
        find "$NVIM_DATA" -maxdepth 1 -name "tree-sitter-*-tmp" -type d -exec rm -rf {} + 2>/dev/null
        find "$NVIM_DATA" -maxdepth 1 -name "tree-sitter-*" -type d ! -name "*.so" -exec rm -rf {} + 2>/dev/null
    fi
fi

# Initialize and update neovim plugins
if command -v nvim >/dev/null 2>&1; then
    color_echo YELLOW "📦  Installing/updating Neovim plugins..."
    
    # Clean up stale lazy.nvim lock files that can cause SIGKILL failures
    if [ -d "$LAZY_DIR" ]; then
        find "$LAZY_DIR" -maxdepth 1 -name "*.cloning" -delete 2>/dev/null
    fi
    
    # Clean up stale treesitter temp directories
    if [ -d "$NVIM_DATA" ]; then
        find "$NVIM_DATA" -maxdepth 1 -name "tree-sitter-*-tmp" -type d -exec rm -rf {} + 2>/dev/null
    fi
    
    nvim --headless -c "lua require('lazy').sync()" -c "qa" 2>/dev/null || true
    color_echo GREEN "  ✅  Neovim plugins updated"
fi

# remove zcompdump files only if ZSH_COMPDUMP is set
if [ -n "${ZSH_COMPDUMP:-}" ]; then
    color_echo YELLOW "🧹  Removing zcompdump file: $ZSH_COMPDUMP"
    rm -f "$ZSH_COMPDUMP"
fi

if is_macos; then
    color_echo YELLOW "🧹  Cleaning up Homebrew..."
    brew cleanup

    color_echo BLUE "💡  Running macOS setup script..."
    if [[ "$run_background" == "true" ]]; then
        mkdir -p "$HOME/.cache"
        log_file="$HOME/.cache/dotfiles_install_${timestamp}.log"
        color_echo YELLOW "🚀  Running package installation in background..."
        color_echo CYAN "   Log file: $log_file"
        if [[ "$USE_DEFAULTS" == "true" ]]; then
            nohup "$DOTDOTFILES/lib/install/mac.sh" --use-defaults "$@" > "$log_file" 2>&1 &
        else
            nohup "$DOTDOTFILES/lib/install/mac.sh" "$@" > "$log_file" 2>&1 &
        fi
        color_echo GREEN "   Background process started (PID: $!)"
        color_echo CYAN "   Monitor with: tail -f $log_file"
    else
        if [[ "$USE_DEFAULTS" == "true" ]]; then
            "$DOTDOTFILES/lib/install/mac.sh" --use-defaults "$@"
        else
            "$DOTDOTFILES/lib/install/mac.sh" "$@"
        fi
    fi
fi

if is_ubuntu; then
    color_echo BLUE "💡  Running Ubuntu setup script..."
    if [[ "$run_background" == "true" ]]; then
        mkdir -p "$HOME/.cache"
        log_file="$HOME/.cache/dotfiles_install_${timestamp}.log"
        color_echo YELLOW "🚀  Running package installation in background..."
        color_echo CYAN "   Log file: $log_file"
        if [[ "$USE_DEFAULTS" == "true" ]]; then
            nohup "$DOTDOTFILES/lib/install/ubuntu.sh" --use-defaults "$@" > "$log_file" 2>&1 &
        else
            nohup "$DOTDOTFILES/lib/install/ubuntu.sh" "$@" > "$log_file" 2>&1 &
        fi
        color_echo GREEN "   Background process started (PID: $!)"
        color_echo CYAN "   Monitor with: tail -f $log_file"
    else
        if [[ "$USE_DEFAULTS" == "true" ]]; then
            "$DOTDOTFILES/lib/install/ubuntu.sh" --use-defaults "$@"
        else
            "$DOTDOTFILES/lib/install/ubuntu.sh" "$@"
        fi
    fi
fi

if [[ ! -f "$HOME/.hushlogin" ]]; then
    color_echo BLUE "🔇  Suppressing default last login message..."
    touch "$HOME/.hushlogin"
fi

color_echo GREEN "✅  Dotfiles synced"

