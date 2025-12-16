#!/usr/bin/env bash
#
# Uninstall dotfiles - removes symlinks and configurations
# Does NOT remove installed packages (brew/apt)
#
# Bash 3.2 compatible (macOS default)

set -euo pipefail

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { printf "%b[+]%b %s\n" "$GREEN" "$NC" "$1"; }
warn() { printf "%b[!]%b %s\n" "$YELLOW" "$NC" "$1"; }
error() { printf "%b[x]%b %s\n" "$RED" "$NC" "$1" >&2; }
info() { printf "%b[*]%b %s\n" "$BLUE" "$NC" "$1"; }

# Remove a symlink if it points to dotfiles
remove_dotfiles_symlink() {
    local target="$1"
    if [[ -L "$target" ]]; then
        local link_dest
        link_dest=$(readlink "$target" 2>/dev/null || echo "")
        if [[ "$link_dest" == "$DOTDOTFILES"* ]]; then
            rm -f "$target"
            log "Removed symlink: $target"
        else
            warn "Skipping $target (not a dotfiles symlink)"
        fi
    elif [[ -e "$target" ]]; then
        warn "Skipping $target (not a symlink)"
    fi
}

# Remove dotfile symlinks from home directory
remove_home_symlinks() {
    info "Removing home directory symlinks..."
    
    local dotfiles_home="$DOTDOTFILES/home"
    if [[ ! -d "$dotfiles_home" ]]; then
        warn "No home directory found in dotfiles"
        return 0
    fi
    
    # Find all files in dotfiles/home and remove corresponding symlinks
    while IFS= read -r source_file; do
        local relative_path="${source_file#"$dotfiles_home"/}"
        local home_file="$HOME/$relative_path"
        remove_dotfiles_symlink "$home_file"
    done < <(find "$dotfiles_home" -type f 2>/dev/null)
}

# Remove SSH config symlink
remove_ssh_config() {
    info "Removing SSH config..."
    remove_dotfiles_symlink "$HOME/.ssh/config"
}

# Remove Cursor config symlinks
remove_cursor_config() {
    info "Removing Cursor configuration..."
    
    local cursor_dir="$HOME/.cursor"
    
    # Remove rule symlinks
    if [[ -d "$cursor_dir/rules" ]]; then
        for rule in "$cursor_dir/rules"/*; do
            [[ -e "$rule" ]] || continue
            remove_dotfiles_symlink "$rule"
        done
    fi
    
    # Remove command symlinks
    if [[ -d "$cursor_dir/commands" ]]; then
        for cmd in "$cursor_dir/commands"/*; do
            [[ -e "$cmd" ]] || continue
            remove_dotfiles_symlink "$cmd"
        done
    fi
}

# Remove scripts symlinks
remove_scripts() {
    info "Removing script symlinks..."
    
    local scripts_dir="$HOME/.local/bin/scripts"
    if [[ -d "$scripts_dir" ]]; then
        for script in "$scripts_dir"/*; do
            [[ -e "$script" ]] || continue
            remove_dotfiles_symlink "$script"
        done
        # Remove directory if empty
        rmdir "$scripts_dir" 2>/dev/null && log "Removed empty: $scripts_dir" || true
    fi
}

# Remove git config include
remove_git_config() {
    info "Removing git config include..."
    
    local include_path="$DOTDOTFILES/lib/.gitconfig_incl"
    local current_include
    current_include=$(git config --global --get include.path 2>/dev/null || echo "")
    
    if [[ "$current_include" == "$include_path" ]]; then
        git config --global --unset include.path
        log "Removed git include.path"
    else
        warn "Git include.path not set to dotfiles (skipping)"
    fi
}

# Remove passwordless sudo (requires sudo)
remove_passwordless_sudo() {
    info "Checking for passwordless sudo config..."
    
    local sudoers_file
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sudoers_file="/private/etc/sudoers.d/$(whoami)"
    else
        sudoers_file="/etc/sudoers.d/$(whoami)"
    fi
    
    if [[ -f "$sudoers_file" ]]; then
        warn "Found passwordless sudo config: $sudoers_file"
        printf "Remove passwordless sudo? (y/n) "
        read -r response
        if [[ "$response" =~ ^[Yy]$ ]]; then
            if sudo rm -f "$sudoers_file"; then
                log "Removed passwordless sudo config"
            else
                error "Failed to remove passwordless sudo config"
            fi
        else
            warn "Keeping passwordless sudo config"
        fi
    fi
}

# Remove cache files
remove_cache_files() {
    info "Removing cache files..."
    
    rm -f "$HOME/.cache/dotfiles_update.lock" 2>/dev/null && log "Removed update lock" || true
    rm -f "$HOME/.cache/dotfiles_update.log" 2>/dev/null && log "Removed update log" || true
    rm -f "$HOME/.cache/dotfiles_update_error" 2>/dev/null && log "Removed update error file" || true
    rm -f "$HOME/.cache/dotfiles_update_success" 2>/dev/null && log "Removed update success file" || true
    rm -f "$HOME/.cache/dotfiles_debug_enabled" 2>/dev/null && log "Removed debug flag" || true
}

# Remove hushlogin
remove_hushlogin() {
    info "Checking hushlogin..."
    
    if [[ -f "$HOME/.hushlogin" ]]; then
        printf "Remove .hushlogin (show login message)? (y/n) "
        read -r response
        if [[ "$response" =~ ^[Yy]$ ]]; then
            rm -f "$HOME/.hushlogin"
            log "Removed .hushlogin"
        fi
    fi
}

# Remove backups directory
remove_backups() {
    info "Checking backups..."
    
    local backups_dir="$DOTDOTFILES/backups"
    if [[ -d "$backups_dir" ]]; then
        printf "Remove backups directory? (y/n) "
        read -r response
        if [[ "$response" =~ ^[Yy]$ ]]; then
            rm -rf "$backups_dir"
            log "Removed backups directory"
        fi
    fi
}

# Main
main() {
    printf "%b" "$BLUE"
    cat << 'EOF'
╔═══════════════════════════════════════════╗
║         Dotfiles Uninstaller              ║
║  This will remove symlinks & configs      ║
║  Packages will NOT be removed             ║
╚═══════════════════════════════════════════╝
EOF
    printf "%b\n" "$NC"
    
    printf "Continue with uninstall? (y/n) "
    read -r response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        info "Uninstall cancelled"
        exit 0
    fi
    
    echo
    remove_home_symlinks
    remove_ssh_config
    remove_cursor_config
    remove_scripts
    remove_git_config
    remove_cache_files
    remove_hushlogin
    remove_passwordless_sudo
    remove_backups
    
    echo
    log "Uninstall complete!"
    warn "The dotfiles directory ($DOTDOTFILES) was NOT removed"
    warn "Installed packages (brew/apt) were NOT removed"
    info "To fully remove, run: rm -rf $DOTDOTFILES"
}

main "$@"
