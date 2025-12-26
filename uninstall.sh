#!/usr/bin/env bash
#
# Uninstall dotfiles - removes symlinks and configurations
# Use --purge-packages to also remove installed packages
#
# Bash 3.2 compatible (macOS default)

set -euo pipefail

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Parse flags
PURGE_PACKAGES=false
for arg in "$@"; do
    case "$arg" in
        --purge-packages) PURGE_PACKAGES=true ;;
    esac
done

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

is_macos() { [[ "$OSTYPE" == "darwin"* ]]; }
is_linux() { [[ "$OSTYPE" == "linux-gnu"* ]]; }

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

# Remove scripts and updater
remove_scripts() {
    info "Removing scripts updater..."
    
    if is_macos; then
        # macOS: remove launchd daemon and symlinks
        local DAEMON_NAME="com.agoodkind.scripts-updater"
        local DAEMON_PLIST="/Library/LaunchDaemons/${DAEMON_NAME}.plist"
        local OLD_AGENT_PLIST="$HOME/Library/LaunchAgents/${DAEMON_NAME}.plist"
        local SCRIPTS_DIR="/usr/local/opt/scripts"
        
        # Unload launchd daemon
        sudo launchctl unload "$DAEMON_PLIST" 2>/dev/null || true
        [[ -f "$DAEMON_PLIST" ]] && sudo rm "$DAEMON_PLIST" && log "Removed: $DAEMON_PLIST"
        
        # Also remove old LaunchAgent if present
        if [[ -f "$OLD_AGENT_PLIST" ]]; then
            launchctl unload "$OLD_AGENT_PLIST" 2>/dev/null || true
            rm "$OLD_AGENT_PLIST" && log "Removed old agent: $OLD_AGENT_PLIST"
        fi
        
        # Remove /etc/paths.d entry
        [[ -f "/etc/paths.d/scripts" ]] && sudo rm "/etc/paths.d/scripts" 2>/dev/null && \
            log "Removed: /etc/paths.d/scripts"
        
        # Remove symlinks from /usr/local/bin
        if [[ -d "$SCRIPTS_DIR" ]]; then
            for script in "$SCRIPTS_DIR"/*; do
                [[ -f "$script" ]] || continue
                local name
                name=$(basename "$script")
                local target="/usr/local/bin/$name"
                if [[ -L "$target" ]] && [[ "$(readlink "$target")" == "$script" ]]; then
                    sudo rm "$target" && log "Removed symlink: $target"
                fi
            done
        fi
        
        # Remove scripts repo
        [[ -d "$SCRIPTS_DIR" ]] && rm -rf "$SCRIPTS_DIR" && log "Removed: $SCRIPTS_DIR"
        
        # Also clean up old ~/.local/bin/scripts if it exists
        local old_scripts_dir="$HOME/.local/bin/scripts"
        if [[ -d "$old_scripts_dir" ]]; then
            for script in "$old_scripts_dir"/*; do
                [[ -e "$script" ]] || continue
                remove_dotfiles_symlink "$script"
            done
            rmdir "$old_scripts_dir" 2>/dev/null && log "Removed empty: $old_scripts_dir" || true
        fi
    else
        # Linux: handled by remove_systemd_updater
        local scripts_dir="$HOME/.local/bin/scripts"
        if [[ -d "$scripts_dir" ]]; then
            for script in "$scripts_dir"/*; do
                [[ -e "$script" ]] || continue
                remove_dotfiles_symlink "$script"
            done
            rmdir "$scripts_dir" 2>/dev/null && log "Removed empty: $scripts_dir" || true
        fi
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
    if is_macos; then
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

# Remove systemd updater (Linux only)
remove_systemd_updater() {
    if is_macos; then
        return 0
    fi
    
    info "Checking for systemd scripts-updater..."
    
    if systemctl is-enabled scripts-updater.timer &>/dev/null 2>&1; then
        warn "Found scripts-updater systemd timer"
        printf "Remove scripts-updater systemd timer? (y/n) "
        read -r response
        if [[ "$response" =~ ^[Yy]$ ]]; then
            sudo systemctl stop scripts-updater.timer 2>/dev/null || true
            sudo systemctl disable scripts-updater.timer 2>/dev/null || true
            sudo rm -f /etc/systemd/system/scripts-updater.service
            sudo rm -f /etc/systemd/system/scripts-updater.timer
            sudo systemctl daemon-reload
            log "Removed scripts-updater systemd timer"
        fi
    fi
}

# =============================================================================
# Package Removal Functions
# =============================================================================

# Source package lists if available
source_packages() {
    local packages_file="$DOTDOTFILES/lib/setup/helpers/packages.sh"
    if [[ -f "$packages_file" ]]; then
        # Only source the arrays, not the full file (which may have functions)
        # Extract just the package arrays
        eval "$(grep -E '^(export )?(COMMON_PACKAGES|BREW_SPECIFIC|BREW_CASK_NAMES|APT_SPECIFIC|SNAP_PACKAGES)=\(' "$packages_file" | head -100)"
        # Read until closing paren for each array
        COMMON_PACKAGES=()
        BREW_SPECIFIC=()
        BREW_CASK_NAMES=()
        APT_SPECIFIC=()
        SNAP_PACKAGES=()
        source "$packages_file" 2>/dev/null || true
        return 0
    fi
    return 1
}

# Remove Homebrew packages (macOS)
remove_brew_packages() {
    if ! is_macos; then
        return 0
    fi
    
    if ! command -v brew &>/dev/null; then
        warn "Homebrew not installed, skipping package removal"
        return 0
    fi
    
    info "Removing Homebrew packages..."
    
    if ! source_packages; then
        warn "Could not load package list"
        return 1
    fi
    
    # Combine package lists
    local all_packages=()
    if [[ ${#COMMON_PACKAGES[@]} -gt 0 ]]; then
        all_packages+=("${COMMON_PACKAGES[@]}")
    fi
    if [[ ${#BREW_SPECIFIC[@]} -gt 0 ]]; then
        all_packages+=("${BREW_SPECIFIC[@]}")
    fi
    
    # Remove formulae
    if [[ ${#all_packages[@]} -gt 0 ]]; then
        local installed_formulae
        installed_formulae=$(brew list --formula 2>/dev/null | tr '\n' ' ')
        
        local to_remove=()
        for pkg in "${all_packages[@]}"; do
            if [[ " $installed_formulae " == *" $pkg "* ]]; then
                to_remove+=("$pkg")
            fi
        done
        
        if [[ ${#to_remove[@]} -gt 0 ]]; then
            warn "Will remove ${#to_remove[@]} formulae: ${to_remove[*]}"
            printf "Continue? (y/n) "
            read -r response
            if [[ "$response" =~ ^[Yy]$ ]]; then
                brew uninstall --force "${to_remove[@]}" 2>/dev/null || true
                log "Removed ${#to_remove[@]} formulae"
            fi
        else
            log "No matching formulae installed"
        fi
    fi
    
    # Remove casks
    if [[ ${#BREW_CASK_NAMES[@]} -gt 0 ]]; then
        local installed_casks
        installed_casks=$(brew list --cask 2>/dev/null | tr '\n' ' ')
        
        local casks_to_remove=()
        for cask in "${BREW_CASK_NAMES[@]}"; do
            if [[ " $installed_casks " == *" $cask "* ]]; then
                casks_to_remove+=("$cask")
            fi
        done
        
        if [[ ${#casks_to_remove[@]} -gt 0 ]]; then
            warn "Will remove ${#casks_to_remove[@]} casks: ${casks_to_remove[*]}"
            printf "Continue? (y/n) "
            read -r response
            if [[ "$response" =~ ^[Yy]$ ]]; then
                brew uninstall --cask --force "${casks_to_remove[@]}" 2>/dev/null || true
                log "Removed ${#casks_to_remove[@]} casks"
            fi
        else
            log "No matching casks installed"
        fi
    fi
}

# Remove APT/Snap packages (Linux)
remove_apt_packages() {
    if ! is_linux; then
        return 0
    fi
    
    if ! command -v apt-get &>/dev/null; then
        warn "apt-get not found, skipping package removal"
        return 0
    fi
    
    info "Removing APT/Snap packages..."
    
    if ! source_packages; then
        warn "Could not load package list"
        return 1
    fi
    
    # Combine package lists
    local all_packages=()
    if [[ ${#COMMON_PACKAGES[@]} -gt 0 ]]; then
        all_packages+=("${COMMON_PACKAGES[@]}")
    fi
    if [[ ${#APT_SPECIFIC[@]} -gt 0 ]]; then
        all_packages+=("${APT_SPECIFIC[@]}")
    fi
    
    # Remove APT packages
    if [[ ${#all_packages[@]} -gt 0 ]]; then
        local to_remove=()
        for pkg in "${all_packages[@]}"; do
            if dpkg -s "$pkg" &>/dev/null 2>&1; then
                to_remove+=("$pkg")
            fi
        done
        
        if [[ ${#to_remove[@]} -gt 0 ]]; then
            warn "Will remove ${#to_remove[@]} APT packages"
            printf "Continue? (y/n) "
            read -r response
            if [[ "$response" =~ ^[Yy]$ ]]; then
                sudo apt-get remove -y "${to_remove[@]}" 2>/dev/null || true
                sudo apt-get autoremove -y 2>/dev/null || true
                log "Removed ${#to_remove[@]} APT packages"
            fi
        else
            log "No matching APT packages installed"
        fi
    fi
    
    # Remove Snap packages
    if command -v snap &>/dev/null && [[ ${#SNAP_PACKAGES[@]} -gt 0 ]]; then
        local snaps_to_remove=()
        for pkg in "${SNAP_PACKAGES[@]}"; do
            if snap list "$pkg" &>/dev/null 2>&1; then
                snaps_to_remove+=("$pkg")
            fi
        done
        
        if [[ ${#snaps_to_remove[@]} -gt 0 ]]; then
            warn "Will remove ${#snaps_to_remove[@]} Snap packages: ${snaps_to_remove[*]}"
            printf "Continue? (y/n) "
            read -r response
            if [[ "$response" =~ ^[Yy]$ ]]; then
                for snap_pkg in "${snaps_to_remove[@]}"; do
                    sudo snap remove "$snap_pkg" 2>/dev/null || true
                done
                log "Removed ${#snaps_to_remove[@]} Snap packages"
            fi
        else
            log "No matching Snap packages installed"
        fi
    fi
}

# Remove all packages (dispatcher)
remove_packages() {
    if [[ "$PURGE_PACKAGES" != "true" ]]; then
        return 0
    fi
    
    info "Package removal requested..."
    
    if is_macos; then
        remove_brew_packages
    elif is_linux; then
        remove_apt_packages
    fi
}

# Main
main() {
    printf "%b" "$BLUE"
    if [[ "$PURGE_PACKAGES" == "true" ]]; then
        cat << 'EOF'
╔═══════════════════════════════════════════╗
║         Dotfiles Uninstaller              ║
║  This will remove symlinks & configs      ║
║  ⚠️  PACKAGES WILL ALSO BE REMOVED ⚠️      ║
╚═══════════════════════════════════════════╝
EOF
    else
        cat << 'EOF'
╔═══════════════════════════════════════════╗
║         Dotfiles Uninstaller              ║
║  This will remove symlinks & configs      ║
║  Packages will NOT be removed             ║
║  Use --purge-packages to remove them      ║
╚═══════════════════════════════════════════╝
EOF
    fi
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
    remove_systemd_updater
    remove_backups
    remove_packages
    
    echo
    log "Uninstall complete!"
    warn "The dotfiles directory ($DOTDOTFILES) was NOT removed"
    if [[ "$PURGE_PACKAGES" != "true" ]]; then
        warn "Installed packages were NOT removed (use --purge-packages)"
    fi
    info "To fully remove, run: rm -rf $DOTDOTFILES"
}

main "$@"
