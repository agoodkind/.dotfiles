#!/usr/bin/env bash

###############################################################################
# Configuration & Setup
###############################################################################

timestamp=$(date +"%Y%m%d_%H%M%S")
export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Source utilities
source "${DOTDOTFILES}/lib/bash/colors.sh"
source "${DOTDOTFILES}/lib/bash/defaults.sh"
source "${DOTDOTFILES}/lib/bash/packages.sh"

# Source local config if it exists (machine-specific settings)
[[ -f "$HOME/.overrides.local" ]] && source "$HOME/.overrides.local"

###############################################################################
# Utility Functions
###############################################################################

is_macos() {
    [[ "$OSTYPE" == "darwin"* ]]
}

is_ubuntu() {
    [[ -f /etc/os-release ]] && grep -qiE 'ubuntu|debian' /etc/os-release
}

is_work_laptop() {
    [[ -n "$WORK_DIR_PATH" ]]
}

skip_on_work_laptop() {
    is_work_laptop && color_echo YELLOW "⏭️  Skipping $1 on work laptop"
}

realpath_cmd() {
    if is_macos && command -v grealpath >/dev/null; then
        grealpath "$@"
    else
        realpath "$@"
    fi
}

get_checksum_cmd() {
    if command -v shasum >/dev/null 2>&1; then
        echo "shasum -a 256"
    else
        echo "sha256sum"
    fi
}

calculate_checksum() {
    local file="$1"
    local cmd
    cmd=$(get_checksum_cmd)
    $cmd "$file" | cut -d' ' -f1
}

has_sudo_access() {
    command -v sudo >/dev/null 2>&1 && (sudo -n true 2>/dev/null || sudo -v 2>/dev/null)
}

###############################################################################
# Parse Command Line Flags
###############################################################################

parse_flags() {
    repair_mode=false
    non_interactive=false
    quick_mode=false
    
    for arg in "$@"; do
        case $arg in
            --repair)
                repair_mode=true
                ;;
            --non-interactive)
                non_interactive=true
                ;;
            --quick)
                quick_mode=true
                ;;
        esac
    done
    
    # Export for use in other functions and subscripts
    export repair_mode non_interactive quick_mode
}

###############################################################################
# Git Operations
###############################################################################

handle_git_lock() {
    local lock_file="$DOTDOTFILES/.git/objects/info/commit-graphs/commit-graph-chain.lock"
    
    if [[ ! -f "$lock_file" ]]; then
        return 0
    fi
    
    if [[ "$non_interactive" == "true" ]]; then
        color_echo YELLOW "🔒  Git is locked, force unlocking..."
        rm -f "$lock_file" 2>/dev/null || \
            sudo rm -f "$lock_file" 2>/dev/null
        color_echo GREEN "🔓  Git unlocked"
    else
        color_echo RED "🔒  Git is locked, do you want to force unlock it?"
        read_with_default "Unlock? (y/n) " "n"
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            sudo rm -f "$lock_file"
            color_echo GREEN "🔓  Git unlocked"
        else
            color_echo RED "🔒  Git is locked, skipping update..."
            exit 1
        fi
    fi
}

update_git_repo() {
    color_echo BLUE "🔄  Updating plugins and submodules..."
    
    handle_git_lock
    
    # can't use config here since we don't know if its been defined yet
    (cd "$DOTDOTFILES" && git pull)
    (cd "$DOTDOTFILES" && git submodule update --init --recursive)
    
    # Update timestamp after git operations (matches original behavior)
    timestamp=$(date +"%Y%m%d_%H%M%S")
}

###############################################################################
# Dotfile Linking
###############################################################################

link_dotfiles() {
    printf "\nLinking dotfiles to home directory\n"
    
    local BACKUPS_PATH="$DOTDOTFILES/backups/$timestamp"
    mkdir -p "$BACKUPS_PATH"
    
    local files
    files=$(find "$DOTDOTFILES/home" -type f)
    color_echo YELLOW "🔗 Linking dotfiles to home directory..."
    
    for source_file in $files; do
        local relative_path
        relative_path=$(realpath_cmd --no-symlinks --relative-to="$DOTDOTFILES/home" "$source_file")
        local backup_file="$BACKUPS_PATH/$relative_path.bak"
        local home_file="$HOME/$relative_path"

        if [[ -e "$home_file" ]]; then
            mkdir -p "$(dirname "$backup_file")"
            cp -Hr "$home_file" "$backup_file"
            color_echo YELLOW "  💾  Backed up: $relative_path"
        fi
        
        mkdir -p "$(dirname "$home_file")"
        ln -sf "$source_file" "$home_file"
        color_echo GREEN "  🔗  Linked: $relative_path"
    done
}

###############################################################################
# SSH Configuration
###############################################################################

sync_ssh_config() {
    color_echo BLUE "🔧  Syncing SSH config..."

    skip_on_work_laptop "SSH config" && return 0
    
    mkdir -p "$HOME/.ssh"
    chmod 700 "$HOME/.ssh"
    
    local src="$DOTDOTFILES/lib/ssh/config"
    local dst="$HOME/.ssh/config"
    
    if [[ -f "$src" ]]; then
        cp -f "$src" "$dst"
        chmod 600 "$dst"
        color_echo GREEN "  ✅  SSH config synced"
    fi
}

###############################################################################
# Authorized Keys
###############################################################################

update_authorized_keys() {
    skip_on_work_laptop "authorized keys" && return 0
    
    color_echo BLUE "🔧  Updating authorized keys..."
    
    if ! wget https://github.com/agoodkind.keys -O "$HOME/.ssh/authorized_keys.tmp"; then
        color_echo RED "❌  Failed to download authorized keys" && exit 1
    fi
    
    # Append missing keys to ~/.ssh/authorized_keys
    mkdir -p "$HOME/.ssh"
    touch "$HOME/.ssh/authorized_keys"
    while IFS= read -r key || [[ -n "$key" ]]; do
        if ! grep -q "$key" "$HOME/.ssh/authorized_keys"; then
            echo "$key" >> "$HOME/.ssh/authorized_keys"
        fi
    done < "$HOME/.ssh/authorized_keys.tmp"
    
    rm -f "$HOME/.ssh/authorized_keys.tmp"
    color_echo GREEN "  ✅  Authorized keys updated"
}

###############################################################################
# Script Synchronization
###############################################################################

sync_script_with_checksum() {
    local src="$1"
    local dst="$2"
    local mode="$3"  # "link" or "copy"
    local script_name
    script_name=$(basename "$src" .sh)
    local target="$dst/$script_name"
    
    local src_sum
    src_sum=$(calculate_checksum "$src")
    
    # Check if target exists and compare checksums
    if [[ -e "$target" ]] || [[ -L "$target" ]]; then
        local dst_file="$target"
        
        # Resolve symlink if it is one
        if [[ -L "$target" ]]; then
            if [[ "$mode" == "link" ]]; then
                # For symlinks, check if it points to the right file
                local current_link
                current_link=$(readlink "$target" 2>/dev/null || echo "")
                if [[ "$current_link" == "$src" ]] && [[ -e "$src" ]]; then
                    return 0  # Symlink is already correct
                fi
            else
                # For copy mode, resolve to actual file
                dst_file=$(readlink -f "$target" 2>/dev/null || echo "$target")
            fi
        fi
        
        # Compare checksums if target file exists
        if [[ -f "$dst_file" ]]; then
            local dst_sum
            dst_sum=$(calculate_checksum "$dst_file")
            if [[ "$src_sum" == "$dst_sum" ]]; then
                return 0  # Already up to date
            fi
        fi
    fi
    
    # Perform sync based on mode
    if [[ "$mode" == "link" ]]; then
        ln -sf "$src" "$target"
        color_echo GREEN "  🔗  Linked: $script_name"
    else
        cp -f "$src" "$target"
        chmod +x "$target"
        color_echo GREEN "  📋  Copied: $script_name"
    fi
}

sync_scripts_to_local() {
    color_echo YELLOW "🔗 Syncing scripts to ~/.local/bin/scripts..."
    
    mkdir -p "$HOME/.local/bin/scripts"
    local scripts
    scripts=$(find "$DOTDOTFILES/lib/scripts" -maxdepth 1 -type f -name "*.sh")
    
    for script in $scripts; do
        sync_script_with_checksum "$script" "$HOME/.local/bin/scripts" "link"
    done
}

sync_scripts_to_opt() {
    color_echo YELLOW "📋 Syncing scripts to /opt/scripts..."

    skip_on_work_laptop "/opt/scripts" && return 0
    
    if ! has_sudo_access; then
        color_echo RED "  ⚠️  Skipping /opt/scripts (no sudo access)"
        return
    fi
    
    sudo mkdir -p /opt/scripts
    
    local scripts
    scripts=$(find "$DOTDOTFILES/lib/scripts" -maxdepth 1 -type f -name "*.sh")
    
    for script in $scripts; do
        local script_name
        script_name=$(basename "$script" .sh)
        local target="/opt/scripts/$script_name"
        local src_sum
        src_sum=$(calculate_checksum "$script")
        
        # Check if copy needs updating
        local needs_copy=true
        if [[ -f "$target" ]]; then
            local dst_sum
            if command -v shasum >/dev/null 2>&1; then
                dst_sum=$(sudo shasum -a 256 "$target" 2>/dev/null | cut -d' ' -f1)
            else
                dst_sum=$(sudo sha256sum "$target" 2>/dev/null | cut -d' ' -f1)
            fi
            if [[ "$src_sum" == "$dst_sum" ]]; then
                needs_copy=false
            fi
        fi
        
        if [[ "$needs_copy" == "true" ]]; then
            sudo cp -f "$script" "$target"
            sudo chmod +x "$target"
            color_echo GREEN "  📋  Copied to /opt: $script_name"
        fi
    done
}

sync_all_scripts() {
    sync_scripts_to_local
    sync_scripts_to_opt
}

###############################################################################
# Neovim Operations
###############################################################################

cleanup_neovim_repair() {
    if [[ "$repair_mode" != "true" ]]; then
        return 0
    fi
    
    color_echo YELLOW "🔧  Repair mode: aggressive cleanup..."
    
    local NVIM_DATA="${XDG_DATA_HOME:-$HOME/.local/share}/nvim"
    local LAZY_DIR="$NVIM_DATA/lazy"
    
    if [[ -d "$LAZY_DIR" ]]; then
        find "$LAZY_DIR" -maxdepth 1 -name "*.cloning" -delete 2>/dev/null
        
        # Remove failed/partial plugin directories (empty or missing .git)
        local dir
        for dir in "$LAZY_DIR"/*/; do
            [[ -d "$dir" ]] || continue
            if [[ ! -d "$dir/.git" ]] || [[ -z "$(ls -A "$dir" 2>/dev/null)" ]]; then
                color_echo YELLOW "  🗑️  Removing incomplete plugin: $(basename "$dir")"
                rm -rf "$dir"
            fi
        done
    fi
    
    if [[ -d "$NVIM_DATA" ]]; then
        find "$NVIM_DATA" -maxdepth 1 -name "tree-sitter-*-tmp" -type d -exec rm -rf {} + 2>/dev/null
        find "$NVIM_DATA" -maxdepth 1 -name "tree-sitter-*" -type d ! -name "*.so" -exec rm -rf {} + 2>/dev/null
    fi
}

update_neovim_plugins() {
    if ! command -v nvim >/dev/null 2>&1; then
        return 0
    fi
    
    color_echo YELLOW "📦  Installing/updating Neovim plugins..."
    
    local NVIM_DATA="${XDG_DATA_HOME:-$HOME/.local/share}/nvim"
    local LAZY_DIR="$NVIM_DATA/lazy"
    
    # Clean up stale lazy.nvim lock files that can cause SIGKILL failures
    if [[ -d "$LAZY_DIR" ]]; then
        find "$LAZY_DIR" -maxdepth 1 -name "*.cloning" -delete 2>/dev/null
    fi
    
    # Clean up stale treesitter temp directories
    if [[ -d "$NVIM_DATA" ]]; then
        find "$NVIM_DATA" -maxdepth 1 -name "tree-sitter-*-tmp" -type d -exec rm -rf {} + 2>/dev/null
    fi
    
    nvim --headless -c "lua require('lazy').sync()" -c "qa" 2>/dev/null || true
    color_echo GREEN "  ✅  Neovim plugins updated"
}

###############################################################################
# Cleanup Operations
###############################################################################

cleanup_zcompdump() {
    if [[ -n "${ZSH_COMPDUMP:-}" ]]; then
        color_echo YELLOW "🧹  Removing zcompdump file: $ZSH_COMPDUMP"
        rm -f "$ZSH_COMPDUMP"
    fi
}

create_hushlogin() {
    if [[ ! -f "$HOME/.hushlogin" ]]; then
        color_echo BLUE "🔇  Suppressing default last login message..."
        touch "$HOME/.hushlogin"
    fi
}

###############################################################################
# OS-Specific Installation
###############################################################################

run_os_install() {
    local install_script=""
    local os_type=""
    
    if is_macos; then
        os_type="macOS"
        install_script="$DOTDOTFILES/lib/install/mac.sh"
        color_echo YELLOW "🧹  Cleaning up Homebrew..."
        brew cleanup
    elif is_ubuntu; then
        os_type="Debian/Ubuntu/Proxmox"
        install_script="$DOTDOTFILES/lib/install/debian.sh"
    else
        return 0
    fi
    
    color_echo BLUE "💡  Running $os_type setup script..."
    
    if [[ "${USE_DEFAULTS:-false}" == "true" ]]; then
        "$install_script" --use-defaults "$@"
    else
        "$install_script" "$@"
    fi
}

###############################################################################
# Main Execution
###############################################################################

main() {
    update_git_repo
    link_dotfiles
    sync_ssh_config
    update_authorized_keys
    sync_all_scripts
    cleanup_neovim_repair
    update_neovim_plugins
    cleanup_zcompdump
    run_os_install "$@"
    create_hushlogin
    color_echo GREEN "✅  Dotfiles synced"
}

parse_flags "$@"
main "$@"
