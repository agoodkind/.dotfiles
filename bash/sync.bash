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
source "${DOTDOTFILES}/bash/core/tools.bash"

dotfiles_log_init "sync"

_on_error() {
    local exit_code=$? cmd="$BASH_COMMAND" line="$1"
    local src="${BASH_SOURCE[1]:-sync.bash}"
    dotfiles_log "FATAL: $src:$line exited $exit_code: $cmd"
    dotfiles_notify "error" "sync.sh failed at $src:$line (exit $exit_code): $cmd"
}
trap '_on_error $LINENO' ERR

###############################################################################
# Sync-specific helpers
###############################################################################


###############################################################################
# Parse Command Line Flags
###############################################################################

parse_flags() {
    repair_mode=false
    quick_mode=false
    skip_git=false
    skip_network=false

    for arg in "$@"; do
        case $arg in
            --repair)           repair_mode=true ;;
            --quick)            quick_mode=true ;;
            --skip-git)         skip_git=true ;;
            --skip-network)     skip_network=true; skip_git=true ;;
        esac
    done

    export repair_mode quick_mode skip_git skip_network
}

###############################################################################
# Dotfile Linking
###############################################################################

link_dotfiles() {
    section "Linking dotfiles"

    local BACKUPS_PATH="$DOTDOTFILES/backups/$timestamp"
    local files
    files=$(find "$DOTDOTFILES/home" -type f)
    local linked_count=0
    local skipped_count=0
    local backed_up_count=0

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
                backed_up_count=$((backed_up_count + 1))
            fi
        fi

        mkdir -p "$(dirname "$home_file")"
        ln -sf "$source_file" "$home_file"
        linked_count=$((linked_count + 1))
    done

    # Summary
}

###############################################################################
# SSH Configuration
###############################################################################

sync_ssh_config() {
    section "Syncing SSH config"

    if is_work_laptop; then
        return 0
    fi

    mkdir -p "$HOME/.ssh"
    chmod 700 "$HOME/.ssh"

    local src="$DOTDOTFILES/lib/ssh/config"
    local dst="$HOME/.ssh/config"

    if [[ -f "$src" ]]; then
        ln -sf "$src" "$dst" && chmod 600 "$src"
    fi
}

###############################################################################
# Authorized Keys
###############################################################################

update_authorized_keys() {
    section "Updating authorized keys"

    if is_work_laptop; then
        return 0
    fi

    if [[ "$skip_network" == true ]]; then
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
        if ! sudo git config --system --get-all safe.directory 2>/dev/null | \
            grep -Fxq "$SCRIPTS_DIR"; then
            sudo git config --system --add safe.directory "$SCRIPTS_DIR" \
                2>/dev/null || true
        fi
        # Trigger an update
        section "Triggering update"
        sudo launchctl start "${DAEMON_NAME}"
        return 0
    fi

    # Run the macOS installer script
    section "Installing launchd updater"
    "$DOTDOTFILES/lib/scripts/install-updater" --platform macos
}

# Simple symlink method for work laptops (no sudo required)
sync_scripts_to_local_symlinks() {
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

    # Run the installer script
    section "Installing systemd timer"
    sudo "$DOTDOTFILES/lib/scripts/install-updater" --platform linux
}

sync_all_scripts() {
    section "Syncing scripts"

    if [[ "$skip_network" == true ]]; then
        if is_work_laptop; then
            sync_scripts_to_local_symlinks
        fi
        return 0
    fi

    sync_scripts_to_local
    sync_scripts_to_opt
}

###############################################################################
# Cursor Configuration
###############################################################################

sync_cursor_config() {
    section "Syncing Cursor configuration"

    local cursor_dir="$HOME/.cursor"
    local src_commands="$DOTDOTFILES/.cursor/commands"
    local src_skills="$DOTDOTFILES/.cursor/skills"
    local src_rules="$DOTDOTFILES/.cursor/rules"

    # Sync commands
    if [[ -d "$src_commands" ]]; then
        mkdir -p "$cursor_dir/commands"
        for cmd in "$src_commands"/*; do
            [[ -f "$cmd" ]] || continue
            local cmd_name
            cmd_name=$(basename "$cmd")
            ln -sf "$cmd" "$cursor_dir/commands/$cmd_name"
        done
    fi

    # Sync skills (each skill is a directory)
    if [[ -d "$src_skills" ]]; then
        mkdir -p "$cursor_dir/skills"
        for skill in "$src_skills"/*/; do
            [[ -d "$skill" ]] || continue
            local skill_name
            local skill_source skill_target current_link
            skill_source=${skill%/}
            skill_name=$(basename "$skill_source")
            skill_target="$cursor_dir/skills/$skill_name"

            if [[ "$skill_source" == "$skill_target" ]]; then
                continue
            fi

            if [[ -L "$skill_target" ]]; then
                current_link=$(readlink "$skill_target" 2>/dev/null || echo "")
                if [[ "$current_link" == "$skill_source" ]]; then
                    continue
                fi
                rm -f "$skill_target"
            fi

            ln -sfn "$skill_source" "$skill_target"
        done
    fi

    # Sync rules
    if [[ -d "$src_rules" ]]; then
        mkdir -p "$cursor_dir/rules"
        for rule in "$src_rules"/*.mdc; do
            [[ -f "$rule" ]] || continue
            local rule_name
            rule_name=$(basename "$rule")
            ln -sf "$rule" "$cursor_dir/rules/$rule_name"
        done
    fi
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

    section "Syncing Cursor User Rules"
    "$sync_script"
}

sync_git_hooks() {
    section "Syncing git hooks"

    if [[ ! -d "$DOTDOTFILES/.githooks" ]]; then
        return 0
    fi

    local hooks_dir="$DOTDOTFILES/.git/hooks"
    local src_hooks="$DOTDOTFILES/.githooks"
    mkdir -p "$hooks_dir"

    local hook hook_name linked=0
    for hook in "$src_hooks"/*; do
        [[ -f "$hook" ]] || continue
        hook_name=$(basename "$hook")
        ln -sfn "../../.githooks/$hook_name" "$hooks_dir/$hook_name"
        linked=$((linked + 1))
    done
}

sync_global_git_hooks() {
    section "Configuring global git hooks"
    local hooks_dir="$DOTDOTFILES/git-global-hooks"
    if [[ -d "$hooks_dir" ]]; then
        git config --global core.hooksPath "$hooks_dir"
    fi
}

check_git_hooks_path() {
    section "Checking git hooks path"

    # This repo does not auto-run git config changes.
    # If you want git to use .githooks directly, set core.hooksPath manually:
    #   git -C "$DOTDOTFILES" config --local core.hooksPath ".githooks"
    local configured
    configured=$(git -C "$DOTDOTFILES" config --local --get core.hooksPath 2>/dev/null || true)

    if [[ -z "$configured" ]] || [[ "$configured" == ".githooks" ]]; then
        return 0
    fi
}

###############################################################################
# Neovim Operations
###############################################################################

cleanup_homebrew_repair() {
    section "Repair: cleaning up Homebrew"

    if [[ "$repair_mode" != "true" ]]; then
        return 0
    fi

    if ! is_macos || ! command -v brew >/dev/null 2>&1; then
        return 0
    fi

    local cache_dir="$HOME/Library/Caches/Homebrew/downloads"
    local incomplete_files
    incomplete_files=$(find "$cache_dir" -name "*.incomplete" 2>/dev/null | wc -l | tr -d " ")
    if [[ "$incomplete_files" -gt 0 ]]; then
        rm -rf "$cache_dir"/*.incomplete 2>/dev/null || true
    fi

    brew cleanup --prune=all 2>/dev/null || true
}

cleanup_neovim_repair() {
    section "Repair: cleaning up Neovim"

    if [[ "$repair_mode" != "true" ]]; then
        return 0
    fi

    local nvim_data="${XDG_DATA_HOME:-$HOME/.local/share}/nvim"
    local lazy_dir="$nvim_data/lazy"

    if [[ -d "$lazy_dir" ]]; then
        find "$lazy_dir" -maxdepth 1 -name "*.cloning" -delete 2>/dev/null
        local dir
        for dir in "$lazy_dir"/*/; do
            [[ -d "$dir" ]] || continue
            if [[ ! -d "$dir/.git" ]] || [[ -z "$(ls -A "$dir" 2>/dev/null)" ]]; then
                rm -rf "$dir"
            fi
        done
    fi

    if [[ -d "$nvim_data" ]]; then
        find "$nvim_data" -maxdepth 1 -name "tree-sitter-*-tmp" -type d \
            -exec rm -rf {} + 2>/dev/null
        find "$nvim_data" -maxdepth 1 -name "tree-sitter-*" -type d ! -name "*.so" \
            -exec rm -rf {} + 2>/dev/null
    fi

    local nvim_swap="${XDG_STATE_HOME:-$HOME/.local/state}/nvim/swap"
    local swap_count
    if [[ -d "$nvim_swap" ]]; then
        swap_count=$(find "$nvim_swap" -name "*.swp" -type f 2>/dev/null | wc -l | tr -d " ")
        if [[ "$swap_count" -gt 0 ]]; then
            find "$nvim_swap" -name "*.swp" -type f -delete 2>/dev/null
        fi
    fi

    local nvim_swap_legacy="$nvim_data/swap"
    if [[ -d "$nvim_swap_legacy" ]]; then
        swap_count=$(find "$nvim_swap_legacy" -name "*.swp" -type f 2>/dev/null | wc -l | tr -d " ")
        if [[ "$swap_count" -gt 0 ]]; then
            find "$nvim_swap_legacy" -name "*.swp" -type f -delete 2>/dev/null
        fi
    fi
}

update_neovim_plugins() {
    section "Updating Neovim plugins"

    if [[ "$skip_network" == true ]]; then
        return 0
    fi

    if ! command -v nvim >/dev/null 2>&1; then
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

    nvim --headless -c "lua require('lazy').sync()" -c "qa"

    # Install treesitter parsers if tree-sitter CLI is available
    if command -v tree-sitter >/dev/null 2>&1; then
        local ts_version
        ts_version=$(tree-sitter --version 2>/dev/null | awk '{print $2}')
        # Require tree-sitter 0.21+ for build command
        if [[ "$(printf '%s\n' "0.21.0" "$ts_version" | sort -V | head -1)" == "0.21.0" ]]; then
            local parsers="bash,lua,vim,vimdoc,python,javascript,typescript,json,yaml"
            section "treesitter parsers"
            nvim --headless "+lua require('nvim-treesitter').install({'${parsers//,/\',\'}'})" "+sleep 10" "+qa"
        else
            color_echo YELLOW "  tree-sitter CLI too old ($ts_version), skipping parser install"
        fi
    fi
}

###############################################################################
# Cleanup Operations
###############################################################################

update_zinit_plugins() {
    section "Updating and compiling zinit plugins"

    if [[ "$skip_network" == true ]]; then
        return 0
    fi

    if ! command -v zsh &>/dev/null; then
        return 1
    fi

    zsh -c '
        source "${DOTDOTFILES:-$HOME/.dotfiles}/lib/zinit/zinit.zsh"
        zinit update --all --quiet 2>&1
        zinit compile --all 2>&1
    ' 2>&1 || {
        return 1
    }
}

###############################################################################

cleanup_zinit_completions() {
    section "Cleaning zinit completions"

    local completions_dir="$HOME/.local/share/zinit/completions"

    if [[ ! -d "$completions_dir" ]]; then
        return 0
    fi

    local removed=0
    while IFS= read -r -d '' link; do
        rm -f "$link"
        removed=$((removed + 1))
    done < <(find "$completions_dir" -maxdepth 1 -type l ! -exec test -e {} \; -print0 2>/dev/null)
}

rebuild_zcompdump() {
    section "Rebuilding zcompdump"

    rm -rf ~/.zcompdump* 2>/dev/null || true

    if ! command -v zsh &>/dev/null; then
        return 1
    fi

    zsh -c '
        fpath=("'"$DOTDOTFILES"'/zshrc/completions" $fpath)
        autoload -Uz compinit
        compinit -d ~/.zcompdump
        zcompile ~/.zcompdump
    ' 2>/dev/null || true
}

rebuild_prefer_cache() {
    section "Rebuilding prefer cache"

    bash "$DOTDOTFILES/bash/background/prefer-cache-rebuild.bash" --force
}

compile_zsh_files() {
    section "Compiling zsh files"

    local compiled=0
    local f
    local dirs=(
        "$DOTDOTFILES/zshrc"
        "$DOTDOTFILES/lib/zinit"
        "$DOTDOTFILES/lib/zsh-defer"
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

}

create_hushlogin() {
    section "Creating hushlogin"
    if [[ ! -f "$HOME/.hushlogin" ]]; then
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
        install_script="$DOTDOTFILES/bash/setup/platform/mac.bash"
        if command -v brew >/dev/null 2>&1; then
            section "Cleaning up Homebrew"
            brew cleanup
        fi
    elif is_ubuntu; then
        os_type="Debian/Ubuntu/Proxmox"
        install_script="$DOTDOTFILES/bash/setup/platform/debian.bash"
    else
        return 0
    fi

    if [[ "${USE_DEFAULTS:-false}" == "true" ]]; then
        section "Running $os_type setup"
        "$install_script" --use-defaults "$@"
    else
        section "Running $os_type setup"
        "$install_script" "$@"
    fi
}

###############################################################################
# Git Repository Operations
###############################################################################

update_git_repo_sync() {
    if [[ "$skip_git" == true ]]; then
        return 0
    fi

    section "Updating git repo"
    dotfiles_update_repo
}

###############################################################################
# Main Execution
###############################################################################

main() {
    # Phase 1: Git
    update_git_repo_sync

    # Phase 2: Cleanup stale symlinks
    cleanup_zinit_completions

    # Phase 3: Core dotfile linking
    link_dotfiles
    sync_ssh_config
    update_authorized_keys

    # Phase 4: Scripts and Cursor config
    sync_all_scripts
    sync_cursor_config
    sync_cursor_user_rules

    # Phase 5: Git hooks
    check_git_hooks_path
    sync_git_hooks
    sync_global_git_hooks

    # Phase 6: OS-specific setup and tools
    update_zinit_plugins
    run_os_install "$@"
    section "Installing custom tools"
    "$DOTDOTFILES/bash/setup/platform/tools.bash" "$@"

    # Phase 7: Neovim
    update_neovim_plugins

    # Phase 8: Repair cleanups
    cleanup_homebrew_repair
    cleanup_neovim_repair

    # Phase 9: Compile and warm caches (order matters: compile first, then caches)
    compile_zsh_files
    rebuild_zcompdump
    rebuild_prefer_cache
    create_hushlogin

    echo -e "${GREEN}Dotfiles synced${NC}"
}

parse_flags "$@"
main "$@"
