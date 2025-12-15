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

###############################################################################
# git worktree shortcut: `git wtk|wkt|wk|wt <branch>`
#
# - If `GIT_WTK_REPOS=(/repo/a /repo/b ...)` and/or `GIT_WTK_PARENT_DIR=/parent`
#   is set, we try to guess the intended repo by checking which candidate repos
#   already have `<branch>` (either `refs/heads/<branch>` or `origin/<branch>`).
#   `GIT_WTK_PARENT_DIR` is searched 1 level deep (`$parent/*` only).
# - If exactly one repo matches, we run the worktree command against that repo
#   (equivalent to `git -C <repo> worktree ...`).
# - If multiple repos match, we print candidates and fall back.
# - If no repo matches (or no candidates configured), we fall back to the current
#   directory and require it to be a git repo.
# - On success (existing or created worktree), we `cd` into the worktree path.
###############################################################################
function _git_wtk() {
    local repo_cwd="$PWD"
    local -a git_prefix
    git_prefix=()

    if [[ "$1" == "--git-c" ]]; then
        # Internal helper to emulate `git -C <repo>` while still `cd`'ing into
        # the resulting worktree at the end.
        repo_cwd="$2"
        git_prefix=(-C "$repo_cwd")
        shift 2
    fi

    local branch_name="$1"
    if [[ -z "$branch_name" ]]; then
        echo "Usage: git wtk <branch-name>" >&2
        return 1
    fi

    if ! command git "${git_prefix[@]}" rev-parse \
        --is-inside-work-tree >/dev/null 2>&1; then
        echo "git wtk: not in a git repo" >&2
        echo "try: git -C /path/to/repo wtk <branch>" >&2
        return 1
    fi

    local common_dir
    # `--git-common-dir` is stable across worktrees; use it to find the main
    # worktree root, even when invoked inside a worktree.
    common_dir="$(command git "${git_prefix[@]}" rev-parse \
        --git-common-dir 2>/dev/null)" || return 1

    if [[ "$common_dir" != /* ]]; then
        # `--git-common-dir` can be relative to the current working directory.
        common_dir="$repo_cwd/$common_dir"
    fi
    common_dir="$(cd "$common_dir" 2>/dev/null && pwd -P)" || return 1

    local main_root
    if [[ "$(basename "$common_dir")" == ".git" ]]; then
        # Normal repo: common dir is `<root>/.git`.
        main_root="$(cd "$(dirname "$common_dir")" && pwd -P)" || return 1
    else
        # Worktree: common dir is usually `<root>/.git/worktrees/<name>`.
        main_root="$(command git "${git_prefix[@]}" rev-parse \
            --show-toplevel 2>/dev/null)" || return 1
    fi

    local base_dir
    # Worktrees are created as siblings of the main worktree root.
    base_dir="$(cd "$(dirname "$main_root")" && pwd -P)" || return 1

    # Find an existing worktree path for the branch regardless of directory
    # name, using porcelain output (stable for parsing).
    local existing_wt_path
    existing_wt_path="$(command git "${git_prefix[@]}" worktree list --porcelain \
        | awk -v branch="$branch_name" '
            /^worktree/ { path=$2 }
            $1 == "branch" && $2 == "refs/heads/" branch { print path; exit }
        ')"

    if [[ -n "$existing_wt_path" ]]; then
        echo "Worktree for '$branch_name' found at $existing_wt_path."
        builtin cd "$existing_wt_path" || return 1
        return 0
    fi

    local dir_name="${branch_name//\//-}"
    local worktree_path="$base_dir/$dir_name"

    command git "${git_prefix[@]}" fetch origin >/dev/null 2>&1

    if command git "${git_prefix[@]}" rev-parse \
        --verify "origin/$branch_name" >/dev/null 2>&1; then
        echo "Branch '$branch_name' found on origin. Creating worktree."
        if command git "${git_prefix[@]}" show-ref --verify --quiet \
            "refs/heads/$branch_name"; then
            command git "${git_prefix[@]}" worktree add \
                "$worktree_path" "$branch_name" || return $?
        else
            command git "${git_prefix[@]}" worktree add --track -b "$branch_name" \
                "$worktree_path" "origin/$branch_name" || return $?
        fi
    else
        echo "Branch '$branch_name' not found. Creating branch and worktree."
        if command git "${git_prefix[@]}" show-ref --verify --quiet \
            "refs/heads/$branch_name"; then
            command git "${git_prefix[@]}" worktree add \
                "$worktree_path" "$branch_name" || return $?
        else
            command git "${git_prefix[@]}" worktree add -b "$branch_name" \
                "$worktree_path" || return $?
        fi
        (
            builtin cd "$worktree_path" \
                && command git push -u origin "$branch_name"
        ) || return $?
    fi

    builtin cd "$worktree_path" || return 1
}

function _git_wtk_candidates() {
    emulate -L zsh
    setopt localoptions no_unset null_glob

    local -A seen
    local -a repos unique
    repos=()
    unique=()

    if (( ${+GIT_WTK_REPOS} )); then
        # Explicit list of repo roots; no traversal.
        repos+=("${(@)GIT_WTK_REPOS}")
    fi

    local parent_dir="${GIT_WTK_PARENT_DIR:-}"
    if [[ -n "$parent_dir" && -d "$parent_dir" ]]; then
        # Search 1 level deep (`$parent_dir/*` only). A repo can have `.git` as
        # a directory or a file (e.g. some worktree and submodule setups).
        #
        # Heuristics:
        # - Scan most-recently-modified directories first.
        # - Optionally cap work by count and/or time.
        local scan_limit="${GIT_WTK_PARENT_SCAN_LIMIT:-500}"
        local scan_seconds="${GIT_WTK_PARENT_SCAN_SECONDS:-0}"
        local scanned=0

        zmodload zsh/datetime >/dev/null 2>&1 || true
        local start_seconds="${SECONDS}"
        local start_realtime="${EPOCHREALTIME:-}"

        local child
        # Newest-first ordering by directory mtime.
        for child in "$parent_dir"/*(/Nom); do
            (( scanned++ ))
            if (( scanned > scan_limit )); then
                break
            fi

            if (( scan_seconds > 0 )); then
                if [[ -n "$start_realtime" && -n "${EPOCHREALTIME:-}" ]]; then
                    if (( EPOCHREALTIME - start_realtime > scan_seconds )); then
                        break
                    fi
                else
                    if (( SECONDS - start_seconds > scan_seconds )); then
                        break
                    fi
                fi
            fi

            [[ -e "$child/.git" ]] || continue
            repos+=("$child")
        done
    fi

    local repo
    for repo in "${repos[@]}"; do
        [[ -n "$repo" ]] || continue
        [[ -d "$repo" ]] || continue
        if [[ -z "${seen[$repo]:-}" ]]; then
            unique+=("$repo")
            seen[$repo]=1
        fi
    done

    printf '%s\n' "${unique[@]}"
}

function _git_wtk_guess_repo() {
    emulate -L zsh
    setopt localoptions no_unset

    local branch_name="$1"
    [[ -n "$branch_name" ]] || return 1

    local -a matches
    matches=()

    local repo
    for repo in "${(@f)$(_git_wtk_candidates)}"; do
        command git -C "$repo" rev-parse --is-inside-work-tree \
            >/dev/null 2>&1 || continue

        # A repo "matches" if the branch exists locally or on origin.
        command git -C "$repo" show-ref --verify --quiet \
            "refs/heads/$branch_name" && matches+=("$repo") && continue

        command git -C "$repo" show-ref --verify --quiet \
            "refs/remotes/origin/$branch_name" && matches+=("$repo")
    done

    if (( ${#matches[@]} == 1 )); then
        echo "${matches[1]}"
        return 0
    fi

    if (( ${#matches[@]} > 1 )); then
        echo "git wtk: multiple repos match '$branch_name'; use -C" >&2
        printf '%s\n' "${matches[@]}" >&2
        return 2
    fi

    return 1
}

function git() {
    if [[ $# -eq 0 ]]; then
        command git
        return $?
    fi

    local -a git_prefix
    git_prefix=()

    if [[ "$1" == "-C" && -n "${2:-}" ]]; then
        # Preserve `git -C <path> ...` behavior for all subcommands.
        git_prefix=(-C "$2")
        shift 2
    fi

    local subcmd="$1"
    shift

    case "$subcmd" in
        wtk|wkt|wk|wt)
            if (( ${#git_prefix[@]} > 0 )); then
                _git_wtk --git-c "${git_prefix[2]}" "$@"
                return $?
            fi

            # Without `-C`, try to guess the intended repo from configured
            # candidates; if guessing doesn't produce exactly one match, fall
            # back to operating on the current directory.
            local guessed_repo=""
            guessed_repo="$(_git_wtk_guess_repo "${1:-}")"
            if [[ -n "$guessed_repo" ]]; then
                _git_wtk --git-c "$guessed_repo" "$@"
            else
                _git_wtk "$@"
            fi
            ;;
        *)
            if (( ${#git_prefix[@]} > 0 )); then
                command git "${git_prefix[@]}" "$subcmd" "$@"
            else
                command git "$subcmd" "$@"
            fi
            ;;
    esac
}
