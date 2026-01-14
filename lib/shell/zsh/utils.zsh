# shellcheck shell=bash
#
#
#
#

###############################################################################
# Core Utility Functions
###############################################################################

# Run command asynchronously (background job)
async_run() { "$@" &! }

###############################################################################
# OS Detection & Caching
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

# OS detection helpers
is_macos() {
    [[ "$OS_TYPE" == "mac" ]]
}

is_debian_based() {
    [[ "$OS_TYPE" == "ubuntu" || "$OS_TYPE" == "debian" ]]
}

is_ubuntu() {
    [[ "$OS_TYPE" == "ubuntu" ]]
}

# Source OS-specific configuration
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

###############################################################################
# General Utility Functions
###############################################################################

# Check for internet connectivity
has_internet() {
    # Try ping v6 first (as requested)
    if command -v ping6 >/dev/null; then
        if ping6 -c 1 -W 2 google.com >/dev/null 2>&1; then
            return 0
        fi
    fi

    # Try ping first (fastest)
    if command -v ping >/dev/null; then
        if ping -c 1 -W 2 8.8.8.8 >/dev/null 2>&1; then
            return 0
        fi
    fi
    
    # Fallback to curl
    if command -v curl >/dev/null; then
        if curl -s --head --request GET --connect-timeout 2 http://google.com >/dev/null 2>&1; then
            return 0
        fi
    fi
    
    return 1
}

# Portable command existence check (zsh builtin, no fork)
isinstalled() { (( $+commands[$1] )); }

# Check if dotfiles have changed since last check (git HEAD + local modifications)
dotfiles_changed_hash() {
    # Fast path: check if .git/HEAD changed (very cheap)
    local head_hash
    if [[ -f "$DOTDOTFILES/.git/HEAD" ]]; then
        read -r head_hash < "$DOTDOTFILES/.git/HEAD"
        # If it's a ref, follow it
        if [[ "$head_hash" == ref:* ]]; then
            local ref="${head_hash#ref: }"
            if [[ -f "$DOTDOTFILES/.git/$ref" ]]; then
                read -r head_hash < "$DOTDOTFILES/.git/$ref"
            fi
        fi
    fi

    # Fast path: check mtime of specific key files instead of scanning everything
    # We only care if:
    # 1. lib/shell/zsh/commands.zsh changed (prefer logic)
    # 2. home/.zshrc changed (aliases defined)
    # 3. .zshrc.local changed
    
    local last_mod
    # Use zsh/stat for fast, consistent, cross-platform timestamp checking
    zmodload -F zsh/stat b:zstat 2>/dev/null
    
    local -a files=("$DOTDOTFILES/lib/shell/zsh/commands.zsh" "$DOTDOTFILES/home/.zshrc" "$DOTDOTFILES/.zshrc.local")
    local max_mtime=0
    local f
    local -a file_stat
    
    for f in "${files[@]}"; do
        if zstat -A file_stat +mtime "$f" 2>/dev/null; then
            (( file_stat[1] > max_mtime )) && max_mtime=$file_stat[1]
        fi
    done
    last_mod=$max_mtime
    
    echo "${head_hash}-${last_mod}"
}

###############################################################################
# Shell Management
###############################################################################

# Enable performance profiling for the next shell session
zsh_profile() {
    mkdir -p ~/.cache
    touch ~/.cache/zsh_profile_next
    echo "Performance profiling enabled for next shell session"
}

# Reload the shell
reload() {
    echo "🔄  Reloading shell..."
    exec zsh
}

###############################################################################
# Dotfiles Management
###############################################################################

# Backup local changes before destructive operations
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
    
    if [[ "$has_changes" == "true" ]] || \
       [[ "$has_untracked" == "true" ]]; then
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
                    cp "$DOTDOTFILES/$file" "$backup_dir/$file" \
                        2>/dev/null || true
                fi
            done
            echo "  ✅ Backed up untracked files to $backup_dir/"
        fi
        
        # Stash tracked changes if any
        if [[ "$has_changes" == "true" ]]; then
            if git --git-dir="$DOTDOTFILES/.git" \
                   --work-tree="$DOTDOTFILES" \
                   stash push -m "backup-before-update-$timestamp" \
                   2>/dev/null; then
                echo "  ✅ Stashed tracked changes: stash@{0}"
                echo "  💡 To restore: git stash pop stash@{0}"
            fi
        fi
        
        echo "📦 Backup complete. Changes saved to:"
        if [[ "$has_changes" == "true" ]]; then
            echo "   - Stashed changes: git stash list"
            echo "     (look for backup-before-update-$timestamp)"
        fi
        [[ "$has_untracked" == "true" ]] && \
            echo "   - Untracked files: $backup_dir/"
    fi
}

# Repair dotfiles by resetting to remote state
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
    # Pull latest changes
    config pull || {
        # If pull fails, force reset to remote
        git --git-dir="$DOTDOTFILES/.git" \
            --work-tree="$DOTDOTFILES" fetch origin
        config reset --hard origin/main
        config clean -fd
    }
    (builtin cd "$DOTDOTFILES" && \
        git submodule update --init --recursive --remote)
    "$DOTDOTFILES/sync.sh" --repair "$@"
    reload
}

# Quick sync dotfiles
sync() {
    "$DOTDOTFILES/sync.sh" --quick "$@"
}

# Dotfile management wrapper for git operations
config() {
    local subcommand="$1"
    shift  # Remove subcommand from arguments
    
    case "$subcommand" in
        update)
            # Backup local changes before discarding
            backup_local_changes
            
            # Fetch latest changes first
            git --git-dir="$DOTDOTFILES/.git" \
                --work-tree="$DOTDOTFILES" fetch --all
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
