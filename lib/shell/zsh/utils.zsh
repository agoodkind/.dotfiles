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
    elif [[ -f /etc/os-release ]]; then
        os_id=""
        os_like=""
        while IFS='=' read -r key value; do
            [[ -n "$key" ]] || continue
            value="${value%\"}"
            value="${value#\"}"
            case "$key" in
                ID) os_id="$value" ;;
                ID_LIKE) os_like="$value" ;;
            esac
        done < /etc/os-release

        case "$os_id" in
            ubuntu) echo "ubuntu" > ~/.cache/os-type.cache ;;
            debian) echo "debian" > ~/.cache/os-type.cache ;;
            *)
                if [[ " $os_like " == *" debian "* ]]; then
                    echo "debian" > ~/.cache/os-type.cache
                else
                    echo "unknown" > ~/.cache/os-type.cache
                fi
                ;;
        esac
    elif command -v apt >/dev/null; then
        echo "debian" > ~/.cache/os-type.cache
    else
        echo "unknown" > ~/.cache/os-type.cache
    fi
fi

read -r OS_TYPE < ~/.cache/os-type.cache
export OS_TYPE

is_macos() {
    [[ "$OS_TYPE" == "mac" ]]
}

is_debian_based() {
    [[ "$OS_TYPE" == "ubuntu" || "$OS_TYPE" == "debian" ]]
}

is_ubuntu() {
    is_debian_based
}


async_run() { "$@" &! }

case "$OS_TYPE" in
    mac)
        source "$DOTDOTFILES/lib/shell/zsh/mac.zsh"
        ;;
    ubuntu|debian)
        source "$DOTDOTFILES/lib/shell/zsh/ubuntu.zsh"
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
    echo "🔄  Reloading shell..."
    exec zsh
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
    (builtin cd "$DOTDOTFILES" && git submodule update --init --recursive --remote)
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
    elif is_debian_based; then
        local did_any=false

        if command -v resolvectl >/dev/null 2>&1; then
            sudo resolvectl flush-caches >/dev/null 2>&1 && did_any=true
        fi

        if command -v systemd-resolve >/dev/null 2>&1; then
            sudo systemd-resolve --flush-caches >/dev/null 2>&1 && did_any=true
        fi

        if command -v systemctl >/dev/null 2>&1; then
            local svc
            for svc in systemd-resolved NetworkManager network-manager nscd \
                dnsmasq unbound bind9 named; do
                if systemctl cat "$svc" >/dev/null 2>&1; then
                    sudo systemctl restart "$svc" >/dev/null 2>&1 && did_any=true
                fi
            done
        elif command -v service >/dev/null 2>&1; then
            local svc
            for svc in network-manager nscd dnsmasq unbound bind9 named; do
                sudo service "$svc" restart >/dev/null 2>&1 && did_any=true
            done
        fi

        if [[ "$did_any" != "true" ]]; then
            echo "No DNS cache service detected" >&2
            return 0
        fi
    else
        echo "Unsupported OS"
        return 1
    fi
}
