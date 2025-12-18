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

case "$OS_TYPE" in
    mac)
        source "$DOTDOTFILES/lib/zsh/mac.zsh"
        ;;
    ubuntu|debian)
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

# portable command existence check (zsh builtin, no fork)
isinstalled() { (( $+commands[$1] )); }

# Run a command asynchronously in the background
# - Disowns the job (no "[1] 12345" notifications)
# - Shell redirections still work: async_run echo "data" > file
# Usage: async_run <command> [args...]
async_run() { "$@" &! }

_needs_sudoedit_for_any_path() {
    emulate -L zsh
    setopt localoptions no_unset

    local p parent
    for p in "$@"; do
        [[ -n "$p" ]] || continue

        # If the file exists and isn't writable, it will fail without sudo.
        if [[ -e "$p" ]]; then
            [[ -w "$p" ]] || return 0
            continue
        fi

        # If creating a new file, parent dir must be writable.
        parent="${p:h}"
        [[ -n "$parent" ]] || parent="."
        if [[ -d "$parent" ]] && [[ ! -w "$parent" ]]; then
            return 0
        fi
    done

    return 1
}

_edit_maybe_sudoedit() {
    emulate -L zsh
    setopt localoptions noshwordsplit

    local editor_bin="$1"
    shift || true

    if (( $# == 0 )); then
        command "$editor_bin"
        return $?
    fi

    # Keep this conservative: if any editor flags are present, don't rewrite.
    # Users can still do `sudo vim ...` (sudo wrapper rewrites that to sudoedit).
    local a
    for a in "$@"; do
        if [[ "$a" == -* || "$a" == +* ]]; then
            command "$editor_bin" "$@"
            return $?
        fi
    done

    if _needs_sudoedit_for_any_path "$@"; then
        command sudo -e -- "$@"
        return $?
    fi

    command "$editor_bin" "$@"
}

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
