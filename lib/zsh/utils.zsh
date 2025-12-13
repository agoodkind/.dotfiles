# shellcheck shell=bash
###############################################################################
# Aliases and functions #######################################################
###############################################################################

# Cache OS detection
if [[ ! -f ~/.cache/os-type.cache ]]; then
    echo "Detecting & caching OS type"
    mkdir -p ~/.cache
    if [[ $(uname) == "Darwin" ]]; then
        echo "mac" > ~/.cache/os-type.cache
    elif command -v apt >/dev/null; then
        echo "ubuntu" > ~/.cache/os-type.cache
    else
        echo "unknown" > ~/.cache/os-type.cache
    fi
fi

read -r OS_TYPE < ~/.cache/os-type.cache
export OS_TYPE

is_macos() {
    [[ "$OS_TYPE" == "mac" ]]
}

is_ubuntu() {
    [[ "$OS_TYPE" == "ubuntu" ]]
}

case "$OS_TYPE" in
    mac)
        source "$DOTDOTFILES/lib/zsh/mac.zsh"
        ;;
    ubuntu)
        source "$DOTDOTFILES/lib/zsh/ubuntu.zsh"
        ;;
    *)
        echo 'Unknown OS!'
        ;;
esac

# the fuck - lazy load this function
fuck () {
    # Undefine this function and source the real one on first use
    unfunction fuck
    eval "$(thefuck --alias)"
    fuck "$(fc -ln -1)"
}

# Run this function to enable profiling for the next shell session
zsh_profile() {
    mkdir -p ~/.cache
    touch ~/.cache/zsh_profile_next
    echo "Performance profiling enabled for next shell session"
}


reload() {
    color_echo YELLOW "🔄  Reloading shell..."
    exec zsh || color_echo RED "❌  Failed to reload shell" && exit 1
}

backup_local_changes() {
    local timestamp=$(date +"%Y%m%d_%H%M%S")
    local has_changes=false
    local has_untracked=false
    local untracked_files
    
    # Check for modified tracked files
    if ! git --git-dir="$DOTDOTFILES/.git" \
             --work-tree="$DOTDOTFILES" \
             diff --quiet HEAD 2>/dev/null; then
        has_changes=true
    fi
    
    # Check for untracked files
    untracked_files=$(git --git-dir="$DOTDOTFILES/.git" \
                           --work-tree="$DOTDOTFILES" \
                           ls-files --others --exclude-standard 2>/dev/null)
    if [[ -n "$untracked_files" ]]; then
        has_untracked=true
    fi
    
    if [[ "$has_changes" == "true" ]] || [[ "$has_untracked" == "true" ]]; then
        echo "📦 Backing up local changes..."
        
        # Create backup directory for untracked files
        local backup_dir="$DOTDOTFILES/backups/git/$timestamp"
        mkdir -p "$backup_dir"
        
        # Backup untracked files
        if [[ "$has_untracked" == "true" ]]; then
            echo "$untracked_files" | while IFS= read -r file; do
                [[ -z "$file" ]] && continue
                local target_dir="$backup_dir/$(dirname "$file")"
                mkdir -p "$target_dir" 2>/dev/null || true
                if [[ -f "$DOTDOTFILES/$file" ]]; then
                    cp "$DOTDOTFILES/$file" "$backup_dir/$file" 2>/dev/null || true
                fi
            done
            echo "  ✅ Backed up untracked files to $backup_dir/"
        fi
        
        # Stash tracked changes if any
        if [[ "$has_changes" == "true" ]]; then
            if git --git-dir="$DOTDOTFILES/.git" \
                   --work-tree="$DOTDOTFILES" \
                   stash push -m "backup-before-update-$timestamp" 2>/dev/null; then
                echo "  ✅ Stashed tracked changes: stash@{0}"
                echo "  💡 To restore: git stash pop stash@{0}"
            fi
        fi
        
        echo "📦 Backup complete. Changes saved to:"
        if [[ "$has_changes" == "true" ]]; then
            echo "   - Stashed changes: git stash list"
            echo "     (look for backup-before-update-$timestamp)"
        fi
        [[ "$has_untracked" == "true" ]] && echo "   - Untracked files: $backup_dir/"
    fi
}

repair() {
    echo "Repairing dotfiles..."
    # Backup local changes before discarding
    backup_local_changes
    
    # Ensure we're on main branch
    config checkout main
    # Discard all local changes to tracked files
    config reset --hard HEAD
    # Remove untracked files and directories that might conflict
    config clean -fd
    # Pull latest changes (will fail if there are conflicts, but we've reset above)
    config pull || {
        # If pull fails, force reset to remote
        git --git-dir="$DOTDOTFILES/.git" --work-tree="$DOTDOTFILES" fetch origin
        config reset --hard origin/main
        config clean -fd
    }
    (cd "$DOTDOTFILES" && git submodule update --init --recursive --remote)
    "$DOTDOTFILES/sync.sh" --repair "$@"
    reload
}

sync() {
    "$DOTDOTFILES/sync.sh" --quick "$@"
}

# dotfile management
config() {
    local subcommand="$1"
    shift  # Remove subcommand from arguments
    
    case "$subcommand" in
        update)
            # Backup local changes before discarding
            backup_local_changes
            
            # Fetch latest changes first
            git --git-dir="$DOTDOTFILES/.git" --work-tree="$DOTDOTFILES" fetch --all
            # Discard all local changes to tracked files
            config reset --hard origin/main 2>/dev/null || true
            # Remove untracked files and directories that might conflict
            config clean -fd
            # Now run repair which will sync everything
            config repair "$@"
            # repair already calls reload, no need to call it again
            ;;
        reload)
            reload "$@"
            ;;
        repair)
            repair "$@"
            ;;
        sync)
            sync "$@"
            ;;
        *)
            # Restore original arguments for git command
            # (subcommand + remaining args)
            git --git-dir="$DOTDOTFILES/.git" \
                --work-tree="$DOTDOTFILES" \
                "$subcommand" "$@"
            ;;
    esac
}


flush_dns() {
    echo "Flushing DNS..."
    if is_macos; then
        sudo dscacheutil -flushcache
        sudo killall -HUP mDNSResponder
    elif is_ubuntu; then
        sudo systemd-resolve --flush-caches
        sudo systemctl restart systemd-resolved
    else
        echo "Unsupported OS"
        return 1
    fi
}

# portable command existence check (zsh builtin, no fork)
isinstalled() { (( $+commands[$1] )); }

# Prefer running an alternate binary for a command when available
prefer() {
    local name="$1"
    local binary="$2"
    shift 2 || true
    local args=("$@")

    if ! isinstalled "$binary"; then
        return
    fi

    local qargs=""
    local arg
    for arg in "${args[@]}"; do
        [[ -z "$arg" ]] && continue
        qargs+=" $(printf '%q' "$arg")"
    done

    eval "$name() { command $binary$qargs \"\$@\"; }"
}

# Prefer an alternate binary only when writing to a terminal (fallback otherwise)
prefer_tty() {
    local name="$1"
    local binary="$2"
    shift 2 || true
    local args=("$@")

    if ! isinstalled "$binary"; then
        return
    fi

    local qargs=""
    local arg
    for arg in "${args[@]}"; do
        [[ -z "$arg" ]] && continue
        qargs+=" $(printf '%q' "$arg")"
    done

    eval "$name() { if [[ -t 1 ]]; then command $binary$qargs \"\$@\"; else command $name \"\$@\"; fi; }"
}

# pbcopy wrapper: on macOS use native, otherwise ssh to source host
pbcopy() {
    if is_macos; then
        if [[ $# -gt 0 ]]; then
            echo -n "$*" | /usr/bin/pbcopy
        else
            /usr/bin/pbcopy
        fi
    else
        if [[ -z "$SSH_SOURCE_HOST" ]]; then
            echo "pbcopy: SSH_SOURCE_HOST not set" >&2
            return 1
        fi
        if [[ $# -gt 0 ]]; then
            echo -n "$*" | ssh "$SSH_SOURCE_HOST" /usr/bin/pbcopy 2>/dev/null
        else
            ssh "$SSH_SOURCE_HOST" /usr/bin/pbcopy 2>/dev/null
        fi
    fi
}