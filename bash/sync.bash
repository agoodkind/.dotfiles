#!/usr/bin/env bash
set -e
set -o pipefail

###############################################################################
# Configuration & Setup
###############################################################################

timestamp=$(date +"%Y%m%d_%H%M%S")
export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

# Source local config if it exists (machine-specific settings)
[[ -f "$HOME/.overrides.local" ]] && source "$HOME/.overrides.local"

# Source utilities
source "${DOTDOTFILES}/bash/core/colors.bash"
source "${DOTDOTFILES}/bash/core/defaults.bash"
source "${DOTDOTFILES}/bash/core/packages.bash"
source "${DOTDOTFILES}/bash/core/progress.bash"
source "${DOTDOTFILES}/bash/core/tools.bash"


###############################################################################
# Sync-specific helpers
###############################################################################


###############################################################################
# Parse Command Line Flags
###############################################################################

parse_flags() {
    repair_mode=false
    non_interactive=false
    quick_mode=false
    skip_git=false

    for arg in "$@"; do
        case $arg in
            --repair)           repair_mode=true ;;
            --non-interactive)  non_interactive=true ;;
            --quick)            quick_mode=true ;;
            --skip-git)         skip_git=true ;;
        esac
    done

    export repair_mode non_interactive quick_mode skip_git
}

###############################################################################
# Dotfile Linking
###############################################################################

link_dotfiles() {
    local vid
    vid=$(progress_vertex_start "Linking dotfiles")

    local BACKUPS_PATH="$DOTDOTFILES/backups/$timestamp"
    local files
    files=$(find "$DOTDOTFILES/home" -type f)
    local linked_count=0
    local skipped_count=0
    local backed_up_count=0

    progress_log "Linking dotfiles to home directory..."

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
                progress_log "  Backed up: $relative_path"
                backed_up_count=$((backed_up_count + 1))
            fi
        fi

        mkdir -p "$(dirname "$home_file")"
        ln -sf "$source_file" "$home_file"
        progress_log "  Linked: $relative_path"
        linked_count=$((linked_count + 1))
    done

    # Summary
    if [[ $skipped_count -gt 0 ]]; then
        progress_log "  $skipped_count file(s) already correctly linked"
    fi
    if [[ $linked_count -gt 0 ]]; then
        progress_log "  $linked_count file(s) linked"
    fi
    if [[ $backed_up_count -gt 0 ]]; then
        progress_log "  $backed_up_count file(s) backed up to: $BACKUPS_PATH"
    fi

    progress_vertex_complete "$vid" "Linking dotfiles (Linked $linked_count, skipped $skipped_count, backed up $backed_up_count)"
}

###############################################################################
# SSH Configuration
###############################################################################

sync_ssh_config() {
    local vid
    vid=$(progress_vertex_start "Syncing SSH config")

    if is_work_laptop; then
        progress_vertex_cached "$vid" "Syncing SSH config (work laptop)"
        return 0
    fi

    mkdir -p "$HOME/.ssh"
    chmod 700 "$HOME/.ssh"

    local src="$DOTDOTFILES/lib/ssh/config"
    local dst="$HOME/.ssh/config"

    if [[ -f "$src" ]]; then
        ln -sf "$src" "$dst" && chmod 600 "$src"
        progress_log "  SSH config synced"
    fi
    progress_vertex_complete "$vid" "Syncing SSH config"
}

###############################################################################
# Authorized Keys
###############################################################################

update_authorized_keys() {
    local vid
    vid=$(progress_vertex_start "Updating authorized keys")

    if is_work_laptop; then
        progress_vertex_cached "$vid" "Updating authorized keys (work laptop)"
        return 0
    fi

    # Use curl (available on fresh macOS) instead of wget
    curl -fsSL https://github.com/agoodkind.keys -o "$HOME/.ssh/authorized_keys.tmp"

    # Append missing keys to ~/.ssh/authorized_keys
    mkdir -p "$HOME/.ssh"
    touch "$HOME/.ssh/authorized_keys"

    local added_count=0
    while IFS= read -r key || [[ -n "$key" ]]; do
        if ! grep -q "$key" "$HOME/.ssh/authorized_keys"; then
            echo "$key" >> "$HOME/.ssh/authorized_keys"
            added_count=$((added_count + 1))
        fi
    done < "$HOME/.ssh/authorized_keys.tmp"

    rm -f "$HOME/.ssh/authorized_keys.tmp"
    progress_log "  Authorized keys updated (added $added_count keys)"
    progress_vertex_complete "$vid" "Updating authorized keys"
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
        color_echo GREEN "  Linked: $script_name"
    else
        cp -f "$src" "$target"
        chmod +x "$target"
        color_echo GREEN "  Copied: $script_name"
    fi
}

sync_scripts_to_local() {
    is_macos "sync_scripts_to_local is macOS-only" || return 0

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
        progress_log "  scripts-updater launchd daemon already installed"
        if ! sudo git config --system --get-all safe.directory 2>/dev/null | \
            grep -Fxq "$SCRIPTS_DIR"; then
            progress_log "  Fixing git safe.directory for scripts-updater..."
            sudo git config --system --add safe.directory "$SCRIPTS_DIR" \
                2>/dev/null || true
        fi
        # Trigger an update
        progress_vertex_exec "Triggering update" sudo launchctl start "${DAEMON_NAME}"
        return 0
    fi

    # Check if /usr/local/opt/scripts exists but is not a git repo
    if [[ -d "$SCRIPTS_DIR" ]] && [[ ! -d "$SCRIPTS_DIR/.git" ]]; then
        progress_log "  Migrating to git-based launchd updater..."
    else
        progress_log "  Installing scripts-updater launchd daemon..."
    fi

    # Run the macOS installer script
    progress_vertex_exec "Installing launchd updater" "$DOTDOTFILES/lib/scripts/install-updater" --platform macos
}

# Simple symlink method for work laptops (no sudo required)
sync_scripts_to_local_symlinks() {
    progress_log "Syncing scripts to ~/.local/bin/scripts (work laptop mode)..."

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
        color_echo GREEN "  $linked_count script(s) linked"
    else
        color_echo GREEN "  All scripts already linked"
    fi
}

sync_scripts_to_opt() {
    # Only run on Debian/Ubuntu (macOS uses local symlinks)
    if ! is_ubuntu; then
        return 0
    fi

    if is_work_laptop; then
        progress_log "Skipping /opt/scripts on work laptop"
        return 0
    fi

    if ! has_sudo_access; then
        color_echo RED "  Skipping /opt/scripts (no sudo access)"
        return
    fi

    # Check if systemd updater is already installed
    if systemctl is-enabled scripts-updater.timer &>/dev/null; then
        color_echo GREEN "scripts-updater systemd timer already installed"
        # Just trigger an update
        sudo systemctl start scripts-updater.service 2>/dev/null || true
        return 0
    fi

    # Check if /opt/scripts exists but is not a git repo (old copy-based install)
    if [[ -d "/opt/scripts" ]] && [[ ! -d "/opt/scripts/.git" ]]; then
        progress_log "  Migrating /opt/scripts to git-based systemd updater..."
    else
        progress_log "  Installing scripts-updater systemd timer..."
    fi

    # Run the installer script
    progress_vertex_exec "Installing systemd timer" sudo "$DOTDOTFILES/lib/scripts/install-updater" --platform linux
}

sync_all_scripts() {
    local vid
    vid=$(progress_vertex_start "Syncing scripts")
    sync_scripts_to_local
    sync_scripts_to_opt
    progress_vertex_complete "$vid" "Syncing scripts"
}

###############################################################################
# Cursor Configuration
###############################################################################

sync_cursor_config() {
    local vid
    vid=$(progress_vertex_start "Syncing Cursor configuration")

    local cursor_dir="$HOME/.cursor"
    local src_rules="$DOTDOTFILES/.cursor/rules"
    local src_commands="$DOTDOTFILES/.cursor/commands"

    # Sync rules locally
    if [[ -d "$src_rules" ]]; then
        mkdir -p "$cursor_dir/rules"
        for rule in "$src_rules"/*; do
            [[ -f "$rule" ]] || continue
            local rule_name
            rule_name=$(basename "$rule")
            ln -sf "$rule" "$cursor_dir/rules/$rule_name"
            progress_log "  Linked rule: $rule_name"
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
            progress_log "  Linked command: $cmd_name"
        done
    fi

    progress_vertex_complete "$vid" "Syncing Cursor configuration"
}

sync_cursor_user_rules() {
    local sync_script="$DOTDOTFILES/bin/sync-cursor-rules"
    local cursor_db="$HOME/Library/Application Support/Cursor/User/globalStorage/state.vscdb"

    is_macos "sync_cursor_user_rules is macOS-only" || return 0

    if [[ ! -x "$sync_script" ]]; then
        return 0
    fi

    if [[ ! -f "$cursor_db" ]]; then
        return 0
    fi

    progress_vertex_exec "Syncing Cursor User Rules" "$sync_script"
}

sync_git_hooks() {
    local vid
    vid=$(progress_vertex_start "Syncing git hooks")

    if [[ ! -d "$DOTDOTFILES/.githooks" ]]; then
        progress_vertex_cached "$vid" "Syncing git hooks (no directory)"
        return 0
    fi

    env DOTDOTFILES="$DOTDOTFILES" sh -c '
        hooks_dir="$DOTDOTFILES/.git/hooks"
        src_hooks="$DOTDOTFILES/.githooks"
        mkdir -p "$hooks_dir"
        for hook in "$src_hooks"/*; do
            [[ -f "$hook" ]] || continue
            hook_name=$(basename "$hook")
            ln -sf "../../.githooks/$hook_name" "$hooks_dir/$hook_name"
            echo "Linked hook: $hook_name"
        done
    '
    progress_vertex_complete "$vid" "Syncing git hooks"
}

sync_global_git_hooks() {
    local vid
    vid=$(progress_vertex_start "Configuring global git hooks")
    local hooks_dir="$DOTDOTFILES/git-global-hooks"
    if [[ -d "$hooks_dir" ]]; then
        git config --global core.hooksPath "$hooks_dir"
    fi
    progress_vertex_complete "$vid" "Configuring global git hooks"
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
        color_echo YELLOW "  core.hooksPath is set to: $configured"
        color_echo YELLOW "  Expected: .githooks"
        color_echo YELLOW "  Run:"
        color_echo YELLOW "    git -C \"$DOTDOTFILES\" config --local core.hooksPath \".githooks\""
    fi
}

###############################################################################
# Neovim Operations
###############################################################################

cleanup_homebrew_repair() {
    if [[ "$repair_mode" != "true" ]]; then
        color_echo GRAY "  Skipping Homebrew repair (not in repair mode)"
        return 0
    fi

    is_macos "Homebrew repair is macOS-only" || return 0

    if ! command -v brew >/dev/null 2>&1; then
        return 0
    fi

    progress_vertex_exec "Repair mode: cleaning up Homebrew" sh -c '
        incomplete_files=$(find "$HOME/Library/Caches/Homebrew/downloads" -name "*.incomplete" 2>/dev/null | wc -l | tr -d " ")
        if [[ "$incomplete_files" -gt 0 ]]; then
            echo "Removing $incomplete_files incomplete download(s)..."
            rm -rf "$HOME/Library/Caches/Homebrew/downloads"/*.incomplete 2>/dev/null
        fi
        brew cleanup --prune=all 2>/dev/null || true
    '
}

cleanup_neovim_repair() {
    if [[ "$repair_mode" != "true" ]]; then
        progress_log "  Skipping Neovim repair (not in repair mode)"
        return 0
    fi

    progress_vertex_exec "Repair mode: cleaning up Neovim" sh -c '
        NVIM_DATA="${XDG_DATA_HOME:-$HOME/.local/share}/nvim"
        LAZY_DIR="$NVIM_DATA/lazy"

        if [ -d "$LAZY_DIR" ]; then
            find "$LAZY_DIR" -maxdepth 1 -name "*.cloning" -delete 2>/dev/null
            for dir in "$LAZY_DIR"/*/; do
                [ -d "$dir" ] || continue
                if [ ! -d "$dir/.git" ] || [ -z "$(ls -A "$dir" 2>/dev/null)" ]; then
                    echo "Removing incomplete plugin: $(basename "$dir")"
                    rm -rf "$dir"
                fi
            done
        fi

        if [ -d "$NVIM_DATA" ]; then
            find "$NVIM_DATA" -maxdepth 1 -name "tree-sitter-*-tmp" -type d \
                -exec rm -rf {} + 2>/dev/null
            find "$NVIM_DATA" -maxdepth 1 -name "tree-sitter-*" -type d ! -name "*.so" \
                -exec rm -rf {} + 2>/dev/null
        fi

        NVIM_SWAP="${XDG_STATE_HOME:-$HOME/.local/state}/nvim/swap"
        if [ -d "$NVIM_SWAP" ]; then
            swap_count=$(find "$NVIM_SWAP" -name "*.swp" -type f 2>/dev/null | wc -l | tr -d " ")
            if [ "$swap_count" -gt 0 ]; then
                find "$NVIM_SWAP" -name "*.swp" -type f -delete 2>/dev/null
                echo "Removed $swap_count swap file(s)"
            fi
        fi

        NVIM_SWAP_LEGACY="$NVIM_DATA/swap"
        if [ -d "$NVIM_SWAP_LEGACY" ]; then
            swap_count=$(find "$NVIM_SWAP_LEGACY" -name "*.swp" -type f 2>/dev/null | wc -l | tr -d " ")
            if [ "$swap_count" -gt 0 ]; then
                find "$NVIM_SWAP_LEGACY" -name "*.swp" -type f -delete 2>/dev/null
                echo "Removed $swap_count swap file(s) from legacy location"
            fi
        fi

        echo "Neovim cleanup complete"
        '
}

update_neovim_plugins() {
    local vid
    vid=$(progress_vertex_start "Updating Neovim plugins")

    if ! command -v nvim >/dev/null 2>&1; then
        progress_log "  Skipping Neovim plugins (nvim not installed)"
        progress_vertex_cached "$vid" "Updating Neovim plugins (nvim not installed)"
        return 0
    fi

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

    progress_log "Running lazy.sync()..."
    progress_vertex_exec "lazy.sync()" nvim --headless -c "lua require('lazy').sync()" -c "qa"

    # Install treesitter parsers if tree-sitter CLI is available
    if command -v tree-sitter >/dev/null 2>&1; then
        local ts_version
        ts_version=$(tree-sitter --version 2>/dev/null | awk '{print $2}')
        # Require tree-sitter 0.21+ for build command
        if [[ "$(printf '%s\n' "0.21.0" "$ts_version" | sort -V | head -1)" == "0.21.0" ]]; then
            progress_log "Installing treesitter parsers..."
            local parsers="bash,lua,vim,vimdoc,python,javascript,typescript,json,yaml"
            progress_vertex_exec "treesitter parsers" nvim --headless "+lua require('nvim-treesitter').install({'${parsers//,/\',\'}'})" "+sleep 10" "+qa"
        else
            color_echo YELLOW "  tree-sitter CLI too old ($ts_version), skipping parser install"
        fi
    fi
    progress_vertex_complete "$vid" "Updating Neovim plugins"
}

###############################################################################
# Cleanup Operations
###############################################################################

cleanup_zinit_completions() {
    local completions_dir="$HOME/.local/share/zinit/completions"

    if [[ ! -d "$completions_dir" ]]; then
        return 0
    fi

    local removed=0
    while IFS= read -r -d '' link; do
        rm -f "$link"
        progress_log "  Removed stale completion symlink: $(basename "$link")"
        removed=$((removed + 1))
    done < <(find "$completions_dir" -maxdepth 1 -type l ! -exec test -e {} \; -print0 2>/dev/null)

    if [[ $removed -gt 0 ]]; then
        progress_log "  Removed $removed stale zinit completion symlink(s)"
    fi
}

cleanup_zcompdump() {
    if [[ -n "${ZSH_COMPDUMP:-}" ]]; then
        progress_log "  Removing zcompdump file: $ZSH_COMPDUMP"
        rm -f "$ZSH_COMPDUMP"
    else
        progress_log "  Skipping zcompdump cleanup (ZSH_COMPDUMP not set)"
    fi
}

compile_zsh_files() {
    local compiled=0
    local f
    local dirs=(
        "$DOTDOTFILES/zshrc"
        "$DOTDOTFILES/lib/zinit"
        "$DOTDOTFILES/home"
        "$DOTDOTFILES/bin"
    )
    while IFS= read -r -d '' f; do
        if [[ "$f" -nt "${f}.zwc" ]] \
            || [[ ! -f "${f}.zwc" ]]; then
            zsh -c "zcompile '$f'" 2>/dev/null \
                && compiled=$((compiled + 1))
        fi
    done < <(
        find "${dirs[@]}" \
            \( -name '*.zsh' -o -name '.zshrc' \) \
            -print0 2>/dev/null
    )

    if [[ $compiled -gt 0 ]]; then
        progress_log \
            "  Compiled $compiled zsh file(s)"
    fi
}

create_hushlogin() {
    local vid
    vid=$(progress_vertex_start "Creating hushlogin")
    if [[ ! -f "$HOME/.hushlogin" ]]; then
        progress_log "  Suppressing default last login message..."
        touch "$HOME/.hushlogin"
        progress_vertex_complete "$vid" "Creating hushlogin"
    else
        progress_log "  Skipping hushlogin (already exists)"
        progress_vertex_cached "$vid" "Creating hushlogin"
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
        install_script="$DOTDOTFILES/bash/setup/platform/mac.bash"
        if command -v brew >/dev/null 2>&1; then
            progress_vertex_exec "Cleaning up Homebrew" brew cleanup
        fi
    elif is_ubuntu; then
        os_type="Debian/Ubuntu/Proxmox"
        install_script="$DOTDOTFILES/bash/setup/platform/debian.bash"
    else
        return 0
    fi

    if [[ "${USE_DEFAULTS:-false}" == "true" ]]; then
        progress_vertex_exec "Running $os_type setup" "$install_script" --use-defaults "$@"
    else
        progress_vertex_exec "Running $os_type setup" "$install_script" "$@"
    fi
}

###############################################################################
# Main Execution
###############################################################################

main() {
    progress_begin "${HOME}/.cache/dotfiles/sync-${timestamp}.log"
    if [[ "$skip_git" != true ]]; then
        local vid
        vid=$(progress_vertex_start "Updating git repo")
        local update_msg
        if update_msg=$(dotfiles_update_repo 2>&1); then
            progress_log "  git update succeeded"
            progress_vertex_complete "$vid" "Updating git repo"
        else
            progress_log "  git update: $update_msg"
            progress_vertex_error "$vid" "Updating git repo failed"
        fi
    fi

    link_dotfiles
    sync_ssh_config
    update_authorized_keys
    sync_all_scripts
    sync_cursor_config
    sync_cursor_user_rules
    check_git_hooks_path
    sync_git_hooks
    sync_global_git_hooks

    # Run OS-specific install
    run_os_install "$@"

    # Install custom tools
    progress_vertex_exec "Installing custom tools" "$DOTDOTFILES/bash/setup/platform/tools.bash" "$@"

    update_neovim_plugins
    cleanup_homebrew_repair
    cleanup_neovim_repair
    cleanup_zinit_completions
    cleanup_zcompdump
    compile_zsh_files
    create_hushlogin

    progress_end
    echo -e "${COLOR_GREEN}Dotfiles synced${TEXT_RESET}"
}

parse_flags "$@"
main "$@"
