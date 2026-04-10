# shellcheck shell=bash
#
#
#
#

###############################################################################
# Core Utility Functions
###############################################################################

# Run command asynchronously (background job)
function async_run() {
    "$@" &!
}

# Resolve the dotfiles remote URL.
# Returns origin URL from git config if present, otherwise
# reads from .git/wsm-url (set by git-wsm strip).
function _dotfiles_remote() {
    local url
    url=$(git --git-dir="$DOTDOTFILES/.git" \
        config remote.origin.url 2>/dev/null)
    if [[ -n "$url" ]]; then
        echo "$url"
        return
    fi
    if [[ -f "$DOTDOTFILES/.git/wsm-url" ]]; then
        cat "$DOTDOTFILES/.git/wsm-url"
    fi
}

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
            if [[ -z "$key" ]]; then
                continue
            fi
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
function is_macos() {
    if [[ "$OS_TYPE" == "mac" ]]; then
        return 0
    fi
    return 1
}

function is_debian_based() {
    if [[ "$OS_TYPE" == "ubuntu" || "$OS_TYPE" == "debian" ]]; then
        return 0
    fi
    return 1
}

function is_ubuntu() {
    if [[ "$OS_TYPE" == "ubuntu" ]]; then
        return 0
    fi
    return 1
}

# Source OS-specific configuration
case "$OS_TYPE" in
    mac)
        source "$DOTDOTFILES/zshrc/platform/mac.zsh"
        ;;
    ubuntu|debian)
        source "$DOTDOTFILES/zshrc/platform/ubuntu.zsh"
        ;;
    *)
        echo 'Unknown OS!'
        ;;
esac

###############################################################################
# General Utility Functions
###############################################################################

# Check for internet connectivity
function has_internet() {
    if command -v ping6 >/dev/null; then
        if ping -c 1 -W 2 google.com >/dev/null 2>&1; then
            return 0
        fi
    fi
    
    return 1
}

# Portable command existence check (lazy PATH lookup, avoids 10ms full hash build)
function isinstalled() {
    command -v "$1" >/dev/null 2>&1
}

function dotfiles_changed_hash() {
    local head_hash=""
    if [[ -f "$DOTDOTFILES/.git/HEAD" ]]; then
        read -r head_hash < "$DOTDOTFILES/.git/HEAD"
        if [[ "$head_hash" == ref:* ]]; then
            local ref="${head_hash#ref: }"
            if [[ -f "$DOTDOTFILES/.git/$ref" ]]; then
                read -r head_hash < "$DOTDOTFILES/.git/$ref"
            fi
        fi
    fi

    # Include mtime of key files so local edits (pre-commit) also
    # invalidate the prefer cache without needing a git commit.
    zmodload -F zsh/stat b:zstat 2>/dev/null
    local max_mtime=0 f
    local -a file_stat
    for f in \
        "$DOTDOTFILES/zshrc/commands/prefer.zsh" \
        "$DOTDOTFILES/home/.zshrc" \
        "$DOTDOTFILES/.zshrc.local"; do
        if zstat -A file_stat +mtime "$f" 2>/dev/null; then
            if (( file_stat[1] > max_mtime )); then
                max_mtime=$file_stat[1]
            fi
        fi
    done

    echo "${head_hash}-${max_mtime}"
}

###############################################################################
# Shell Management
###############################################################################

# Reload the shell
function reload() {
    echo "🔄  Reloading shell..."
    exec zsh
}

###############################################################################
# Dotfiles Management
###############################################################################

# Backup local changes before destructive operations
function backup_local_changes() {
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
                if [[ -z "$file" ]]; then
            continue
        fi
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
        if [[ "$has_untracked" == "true" ]]; then
            echo "   - Untracked files: $backup_dir/"
        fi
    fi
}

# Repair dotfiles by resetting to remote state
function repair() {
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
        config fetch
        config reset --hard origin/main
        config clean -fd
    }
    (builtin cd "$DOTDOTFILES" && \
        git submodule update --init --recursive --remote)
    "$DOTDOTFILES/sync.sh" --repair --skip-git "$@"
    reload
}

# Quick sync dotfiles
function sync() {
    "$DOTDOTFILES/sync.sh" --quick "$@"
}

# Dotfile management wrapper for git operations
function config() {
    local subcommand="$1"
    shift  # Remove subcommand from arguments
    
    case "$subcommand" in
        update)
            # Backup local changes before discarding
            backup_local_changes
            
            # Fetch latest changes first
            config fetch
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
        fetch|pull|push)
            if git --git-dir="$DOTDOTFILES/.git" \
                remote | grep -q .; then
                git --git-dir="$DOTDOTFILES/.git" \
                    --work-tree="$DOTDOTFILES" \
                    "$subcommand" "$@"
            else
                local _url
                _url=$(_dotfiles_remote) || {
                    echo "wsm: no remote configured" >&2
                    return 1
                }
                case "$subcommand" in
                    fetch)
                        git --git-dir="$DOTDOTFILES/.git" \
                            --work-tree="$DOTDOTFILES" \
                            fetch "$_url" \
                            '+refs/heads/*:refs/remotes/origin/*' \
                            "$@"
                        ;;
                    pull)
                        git --git-dir="$DOTDOTFILES/.git" \
                            --work-tree="$DOTDOTFILES" \
                            fetch "$_url" \
                            '+refs/heads/*:refs/remotes/origin/*'
                        local _pull_cmd=merge
                        if [[ "$(git --git-dir="$DOTDOTFILES/.git" \
                            config --get pull.rebase \
                            2>/dev/null)" == "true" ]]; then
                            _pull_cmd=rebase
                        fi
                        git --git-dir="$DOTDOTFILES/.git" \
                            --work-tree="$DOTDOTFILES" \
                            "$_pull_cmd" origin/main "$@"
                        ;;
                    push)
                        git --git-dir="$DOTDOTFILES/.git" \
                            --work-tree="$DOTDOTFILES" \
                            push "$_url" main "$@"
                        ;;
                esac
            fi
            ;;
        *)
            git --git-dir="$DOTDOTFILES/.git" \
                --work-tree="$DOTDOTFILES" \
                "$subcommand" "$@"
            ;;
    esac
}
