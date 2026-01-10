#!/usr/bin/env bash

###############################################################################
# Configuration & Setup
###############################################################################

timestamp=$(date +"%Y%m%d_%H%M%S")
export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Source local config if it exists (machine-specific settings)
[[ -f "$HOME/.overrides.local" ]] && source "$HOME/.overrides.local"

# Source utilities
source "${DOTDOTFILES}/lib/setup/helpers/colors.sh"
source "${DOTDOTFILES}/lib/setup/helpers/defaults.sh"
source "${DOTDOTFILES}/lib/setup/helpers/packages.sh"

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
    if command -v grealpath >/dev/null; then
        grealpath "$@"
    elif [[ "$OSTYPE" != "darwin"* ]]; then
        realpath "$@"
    else
        # macOS realpath doesn't support GNU options; provide fallback
        echo "realpath_cmd: macOS fallback not available for: $*" >&2
        return 1
    fi
}

# Get path relative to a base directory (works without GNU coreutils)
relative_path_from() {
    local base="$1"
    local target="$2"
    # Simple case: target is under base - just strip the prefix
    if [[ "$target" == "$base"/* ]]; then
        echo "${target#"$base"/}"
    else
        # Fall back to realpath if available
        realpath_cmd --relative-to="$base" "$target"
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
    local files
    files=$(find "$DOTDOTFILES/home" -type f)
    local linked_count=0
    local skipped_count=0
    local backed_up_count=0
    
    color_echo YELLOW "🔗 Linking dotfiles to home directory..."
    
    for source_file in $files; do
        local relative_path
        relative_path=$(relative_path_from "$DOTDOTFILES/home" "$source_file")
        local home_file="$HOME/$relative_path"

        # Check if already correctly linked
        if [[ -L "$home_file" ]]; then
            local link_target
            link_target=$(readlink "$home_file" 2>/dev/null || echo "")
            if [[ "$link_target" == "$source_file" ]]; then
                # Already correctly linked, skip
                skipped_count=$((skipped_count + 1))
                continue
            fi
        fi

        # Need to create or update the link
        if [[ -e "$home_file" ]] || [[ -L "$home_file" ]]; then
            # File exists (or is a broken/wrong symlink), back it up
            mkdir -p "$BACKUPS_PATH"
            local backup_file="$BACKUPS_PATH/$relative_path.bak"
            mkdir -p "$(dirname "$backup_file")"
            if cp -Hr "$home_file" "$backup_file" 2>/dev/null; then
                color_echo YELLOW "  💾  Backed up: $relative_path"
                backed_up_count=$((backed_up_count + 1))
            fi
        fi
        
        mkdir -p "$(dirname "$home_file")"
        ln -sf "$source_file" "$home_file"
        color_echo GREEN "  🔗  Linked: $relative_path"
        linked_count=$((linked_count + 1))
    done
    
    # Summary
    if [[ $skipped_count -gt 0 ]]; then
        color_echo GREEN "  ✅  $skipped_count file(s) already correctly linked"
    fi
    if [[ $linked_count -gt 0 ]]; then
        color_echo GREEN "  ✅  $linked_count file(s) linked"
    fi
    if [[ $backed_up_count -gt 0 ]]; then
        color_echo YELLOW "  📁  $backed_up_count file(s) backed up to: $BACKUPS_PATH"
    fi
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
        ln -sf "$src" "$dst"
        chmod 600 "$src"
        color_echo GREEN "  ✅  SSH config synced"
    fi
}

###############################################################################
# Authorized Keys
###############################################################################

update_authorized_keys() {
    skip_on_work_laptop "authorized keys" && return 0
    
    color_echo BLUE "🔧  Updating authorized keys..."
    
    # Use curl (available on fresh macOS) instead of wget
    if ! curl -fsSL https://github.com/agoodkind.keys -o "$HOME/.ssh/authorized_keys.tmp"; then
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
    # macOS only
    if ! is_macos; then
        return 0
    fi
    
    # Work laptop: use simple ~/.local/bin/scripts symlinks (no sudo, no launchd)
    if is_work_laptop; then
        sync_scripts_to_local_symlinks
        return 0
    fi
    
    # Personal Mac: install launchd daemon to auto-update /usr/local/opt/scripts
    local DAEMON_NAME="com.agoodkind.scripts-updater"
    local DAEMON_PLIST="/Library/LaunchDaemons/${DAEMON_NAME}.plist"
    local SCRIPTS_DIR="/usr/local/opt/scripts"
    
    # Check if launchd daemon is already installed
    if [[ -f "$DAEMON_PLIST" ]] && [[ -d "$SCRIPTS_DIR/.git" ]]; then
        color_echo GREEN "✅  scripts-updater launchd daemon already installed"
        if ! sudo git config --system --get-all safe.directory 2>/dev/null | \
            grep -Fxq "$SCRIPTS_DIR"; then
            color_echo YELLOW "🔧  Fixing git safe.directory for scripts-updater..."
            sudo git config --system --add safe.directory "$SCRIPTS_DIR" \
                2>/dev/null || true
        fi
        # Trigger an update
        sudo launchctl start "${DAEMON_NAME}" 2>/dev/null || true
        return 0
    fi
    
    # Check if /usr/local/opt/scripts exists but is not a git repo
    if [[ -d "$SCRIPTS_DIR" ]] && [[ ! -d "$SCRIPTS_DIR/.git" ]]; then
        color_echo YELLOW "🔄  Migrating to git-based launchd updater..."
    else
        color_echo YELLOW "📦  Installing scripts-updater launchd daemon..."
    fi
    
    # Run the macOS installer script
    "$DOTDOTFILES/lib/scripts/install-updater-mac"
}

# Simple symlink method for work laptops (no sudo required)
sync_scripts_to_local_symlinks() {
    color_echo YELLOW "🔗 Syncing scripts to ~/.local/bin/scripts (work laptop mode)..."
    
    mkdir -p "$HOME/.local/bin/scripts"
    local scripts
    # Find executable files (excluding hidden files and specific non-script files)
    scripts=$(find "$DOTDOTFILES/lib/scripts" -maxdepth 1 -type f \
        \( -perm -u=x -o -perm -g=x -o -perm -o=x \) \
        ! -name ".*" ! -name "LICENSE" ! -name "*.md" ! -name "*.txt")
    
    local linked_count=0
    for script in $scripts; do
        local script_name
        script_name=$(basename "$script")
        local target="$HOME/.local/bin/scripts/$script_name"
        
        # Check if symlink already points to correct location
        if [[ -L "$target" ]]; then
            local current_link
            current_link=$(readlink "$target" 2>/dev/null || echo "")
            if [[ "$current_link" == "$script" ]]; then
                continue  # Already correct
            fi
        fi
        
        ln -sf "$script" "$target"
        linked_count=$((linked_count + 1))
    done
    
    if [[ $linked_count -gt 0 ]]; then
        color_echo GREEN "  ✅  $linked_count script(s) linked"
    else
        color_echo GREEN "  ✅  All scripts already linked"
    fi
}

sync_scripts_to_opt() {
    # Only run on Debian/Ubuntu (macOS uses local symlinks)
    if ! is_ubuntu; then
        return 0
    fi
    
    skip_on_work_laptop "/opt/scripts" && return 0
    
    if ! has_sudo_access; then
        color_echo RED "  ⚠️  Skipping /opt/scripts (no sudo access)"
        return
    fi
    
    # Check if systemd updater is already installed
    if systemctl is-enabled scripts-updater.timer &>/dev/null; then
        color_echo GREEN "✅  scripts-updater systemd timer already installed"
        # Just trigger an update
        sudo systemctl start scripts-updater.service 2>/dev/null || true
        return 0
    fi
    
    # Check if /opt/scripts exists but is not a git repo (old copy-based install)
    if [[ -d "/opt/scripts" ]] && [[ ! -d "/opt/scripts/.git" ]]; then
        color_echo YELLOW "🔄  Migrating /opt/scripts to git-based systemd updater..."
    else
        color_echo YELLOW "📦  Installing scripts-updater systemd timer..."
    fi
    
    # Run the installer script
    sudo "$DOTDOTFILES/lib/scripts/install-updater"
}

sync_all_scripts() {
    sync_scripts_to_local
    sync_scripts_to_opt
}

###############################################################################
# Cursor Configuration
###############################################################################

sync_cursor_config() {
    color_echo BLUE "🔧  Syncing Cursor configuration..."
    
    local cursor_dir="$HOME/.cursor"
    local src_rules="$DOTDOTFILES/lib/cursor/rules"
    local src_commands="$DOTDOTFILES/lib/cursor/commands"
    
    # Sync rules locally
    if [[ -d "$src_rules" ]]; then
        mkdir -p "$cursor_dir/rules"
        for rule in "$src_rules"/*; do
            [[ -f "$rule" ]] || continue
            local rule_name
            rule_name=$(basename "$rule")
            ln -sf "$rule" "$cursor_dir/rules/$rule_name"
            color_echo GREEN "  🔗  Linked rule: $rule_name"
        done
    fi
    
    # Sync commands
    if [[ -d "$src_commands" ]]; then
        mkdir -p "$cursor_dir/commands"
        for cmd in "$src_commands"/*; do
            [[ -f "$cmd" ]] || continue
            local cmd_name
            cmd_name=$(basename "$cmd")
            ln -sf "$cmd" "$cursor_dir/commands/$cmd_name"
            color_echo GREEN "  🔗  Linked command: $cmd_name"
        done
    fi
}

sync_cursor_user_rules() {
    local sync_script="$DOTDOTFILES/bin/sync-cursor-rules"
    local cursor_db="$HOME/Library/Application Support/Cursor/User/globalStorage/state.vscdb"

    if ! is_macos; then
        return 0
    fi

    if [[ ! -x "$sync_script" ]]; then
        return 0
    fi

    if [[ ! -f "$cursor_db" ]]; then
        return 0
    fi

    color_echo BLUE "☁️  Syncing Cursor User Rules..."
    "$sync_script"
}

sync_git_hooks() {
    color_echo BLUE "🪝  Syncing git hooks..."

    local hooks_dir="$DOTDOTFILES/.git/hooks"
    local src_hooks="$DOTDOTFILES/.githooks"

    if [[ -d "$src_hooks" ]]; then
        mkdir -p "$hooks_dir"
        for hook in "$src_hooks"/*; do
            [[ -f "$hook" ]] || continue
            local hook_name
            hook_name=$(basename "$hook")
            ln -sf "../../.githooks/$hook_name" "$hooks_dir/$hook_name"
            color_echo GREEN "  🔗  Linked hook: $hook_name"
        done
    fi
}

check_git_hooks_path() {
    # This repo does not auto-run git config changes.
    # If you want git to use .githooks directly, set core.hooksPath manually:
    #   git -C "$DOTDOTFILES" config --local core.hooksPath ".githooks"
    local configured
    configured=$(git -C "$DOTDOTFILES" config --local --get core.hooksPath 2>/dev/null || true)

    if [[ -z "$configured" ]]; then
        return 0
    fi

    if [[ "$configured" != ".githooks" ]]; then
        color_echo YELLOW "  ⚠️  core.hooksPath is set to: $configured"
        color_echo YELLOW "  ⚠️  Expected: .githooks"
        color_echo YELLOW "  Run:"
        color_echo YELLOW "    git -C \"$DOTDOTFILES\" config --local core.hooksPath \".githooks\""
    fi
}

###############################################################################
# Neovim Operations
###############################################################################

cleanup_homebrew_repair() {
    if [[ "$repair_mode" != "true" ]]; then
        return 0
    fi
    
    if ! is_macos; then
        return 0
    fi
    
    if ! command -v brew >/dev/null 2>&1; then
        return 0
    fi
    
    color_echo YELLOW "🔧  Repair mode: cleaning up Homebrew..."
    
    # Remove incomplete downloads that can cause lock issues
    local incomplete_files
    incomplete_files=$(find "$HOME/Library/Caches/Homebrew/downloads" -name "*.incomplete" 2>/dev/null | wc -l | tr -d ' ')
    if [[ "$incomplete_files" -gt 0 ]]; then
        color_echo YELLOW "  🗑️  Removing $incomplete_files incomplete download(s)..."
        rm -rf "$HOME/Library/Caches/Homebrew/downloads"/*.incomplete 2>/dev/null
    fi
    
    # Run brew cleanup
    color_echo YELLOW "  🧹  Running brew cleanup..."
    brew cleanup --prune=all 2>/dev/null || true
    
    color_echo GREEN "  ✅  Homebrew cleanup complete"
}

cleanup_neovim_repair() {
    if [[ "$repair_mode" != "true" ]]; then
        return 0
    fi
    
    color_echo YELLOW "🔧  Repair mode: cleaning up Neovim..."
    
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
    
    color_echo GREEN "  ✅  Neovim cleanup complete"
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
        install_script="$DOTDOTFILES/lib/setup/platform/mac.sh"
        # Only cleanup if brew is already installed
        if command -v brew >/dev/null 2>&1; then
            color_echo YELLOW "🧹  Cleaning up Homebrew..."
            brew cleanup
        fi
    elif is_ubuntu; then
        os_type="Debian/Ubuntu/Proxmox"
        install_script="$DOTDOTFILES/lib/setup/platform/debian.sh"
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
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    color_echo BLUE "[1/11] Updating git repo..."
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    update_git_repo

    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    color_echo BLUE "[2/11] Linking dotfiles..."
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    link_dotfiles

    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    color_echo BLUE "[3/11] Syncing SSH config..."
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    sync_ssh_config

    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    color_echo BLUE "[4/11] Updating authorized keys..."
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    update_authorized_keys

    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    color_echo BLUE "[5/11] Syncing scripts..."
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    sync_all_scripts

    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    color_echo BLUE "[6/11] Syncing Cursor config..."
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    sync_cursor_config
    sync_cursor_user_rules
    check_git_hooks_path
    sync_git_hooks

    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    color_echo BLUE "[7/11] Repair cleanup (if --repair)..."
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    cleanup_homebrew_repair
    cleanup_neovim_repair

    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    color_echo BLUE "[8/11] Updating Neovim plugins..."
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    update_neovim_plugins

    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    color_echo BLUE "[9/11] Cleaning up zcompdump..."
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    cleanup_zcompdump

    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    color_echo BLUE "[10/11] Running OS-specific install..."
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    run_os_install "$@"

    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    color_echo BLUE "[11/11] Creating hushlogin..."
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    create_hushlogin

    color_echo GREEN "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    color_echo GREEN "✅  Dotfiles synced"
    color_echo GREEN "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

parse_flags "$@"
main "$@"
