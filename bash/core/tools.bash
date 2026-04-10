#!/usr/bin/env bash
# Helper functions for tool installation scripts

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "${DOTDOTFILES}/bash/core/colors.bash"

# Exit with skip message if not Linux. Call before Linux-only install logic.
# Usage: linux_only "reason for skipping"
linux_only() {
    local reason="$1"
    if [[ "$(uname -s)" != "Linux" ]]; then
        color_echo YELLOW "  ⏭  $reason"
        exit 0
    fi
}

# Exit with skip message if not macOS. Call before macOS-only install logic.
# Usage: mac_only "reason for skipping"
mac_only() {
    local reason="$1"
    if [[ "$(uname -s)" != "Darwin" ]]; then
        color_echo YELLOW "  ⏭  $reason"
        exit 0
    fi
}

# Returns 0 on Linux, 1 otherwise. Optional msg: echo skip reason when not Linux.
# Usage: is_linux "reason" || return
is_linux() {
    if [[ "$(uname -s)" == "Linux" ]]; then
        return 0
    fi
    if [[ -n "${1:-}" ]]; then
        color_echo YELLOW "  ⏭  $1"
    fi
    return 1
}

# Returns 0 on macOS, 1 otherwise. Optional msg: echo skip reason when not macOS.
# Usage: is_macos "reason" || return
is_macos() {
    if [[ "$(uname -s)" == "Darwin" ]]; then
        return 0
    fi
    if [[ -n "${1:-}" ]]; then
        color_echo YELLOW "  ⏭  $1"
    fi
    return 1
}

# Returns 0 on Debian/Ubuntu, 1 otherwise.
is_ubuntu() {
    if [[ ! -f /etc/os-release ]]; then
        return 1
    fi
    grep -qiE 'ubuntu|debian' /etc/os-release
}

# Returns 0 if running on work laptop (WORK_DIR_PATH set).
is_work_laptop() {
    if [[ -n "${WORK_DIR_PATH:-}" ]]; then
        return 0
    fi
    return 1
}

# GNU realpath. On macOS without coreutils, returns 1.
realpath_cmd() {
    if command -v grealpath >/dev/null; then
        grealpath "$@"
    elif [[ "${OSTYPE:-}" != "darwin"* ]]; then
        realpath "$@"
    else
        echo "realpath_cmd: macOS fallback not available for: $*" >&2
        return 1
    fi
}

# Path relative to base. Simple prefix strip when target is under base.
relative_path_from() {
    local base="$1"
    local target="$2"
    if [[ "$target" == "$base"/* ]]; then
        echo "${target#"$base"/}"
    else
        realpath_cmd --relative-to="$base" "$target"
    fi
}

# Print a section header (separator line + title). Used by sync.bash for task output.
# Usage: section "Task name"
section() {
    printf '\n'
    printf '%.0s━' {1..40}
    printf '\n%s\n' "$1"
}

###############################################################################
# Portable Date Helpers
###############################################################################

# Convert a unix epoch to a formatted date string.
# Uses awk strftime, which is POSIX-standard and works on BSD awk (macOS),
# mawk, and gawk (Linux) without any platform detection.
# Usage: epoch_to_date <epoch> [format]
# Default format: %Y-%m-%d
epoch_to_date() {
    local epoch="$1"
    local fmt="${2:-%Y-%m-%d}"
    awk -v ts="$epoch" -v fmt="$fmt" 'BEGIN { print strftime(fmt, ts) }'
}

# Current unix epoch.
# Usage: epoch_now
epoch_now() {
    date +%s
}

###############################################################################

get_checksum_cmd() {
    if command -v shasum >/dev/null 2>&1; then
        echo "shasum -a 256"
    else
        echo "sha256sum"
    fi
}

calculate_checksum() {
    local file="$1"
    local cmd
    cmd=$(get_checksum_cmd)
    $cmd "$file" | cut -d' ' -f1
}

has_sudo_access() {
    if ! command -v sudo >/dev/null 2>&1; then
        return 1
    fi
    if sudo -n true 2>/dev/null; then
        return 0
    fi
    sudo -v 2>/dev/null
}

# Get system architecture and OS type
# Sets: ARCH, OS_TYPE, OS_NAME
get_system_info() {
    ARCH=$(uname -m)
    OS_TYPE=$(uname -s | tr '[:upper:]' '[:lower:]')

    case "$OS_TYPE" in
        darwin) OS_NAME="macos" ;;
        linux) OS_NAME="linux" ;;
        *) OS_NAME="unknown" ;;
    esac
}

# Fetch latest release data from GitHub API
# Usage: get_github_release_data "owner/repo"
get_github_release_data() {
    local repo="$1"
    local releases
    releases=$(curl -s "https://api.github.com/repos/$repo/releases")

    # Try latest stable release first
    local release_data
    release_data=$(echo "$releases" | jq -c '.[] |
        select(.assets | length > 0) |
        select(.prerelease == false)' | head -n 1)

    # Fallback to any release with assets if no stable ones found
    if [[ -z "$release_data" ]]; then
        release_data=$(echo "$releases" | jq -c '.[] |
            select(.assets | length > 0)' | head -n 1)
    fi

    echo "$release_data"
}

# Download and install a binary from a GitHub release asset
# Usage: install_from_github "owner/repo" "os_tag" "arch_tag" "extension" "bin_name"
install_from_github() {
    local repo="$1"
    local os_tag="$2"
    local arch_tag="$3"
    local extension="$4"
    local bin_name="$5"

    local release_data
    release_data=$(get_github_release_data "$repo")

    if [[ -z "$release_data" ]]; then
        color_echo RED "  ❌  No releases with assets found for $repo"
        return 1
    fi

    local tag
    tag=$(echo "$release_data" | jq -r .tag_name)

    local filename
    filename=$(
        echo "$release_data" \
            | jq -r \
                --arg os "$os_tag" \
                --arg arch "$arch_tag" \
                --arg ext "$extension" \
                '.assets[].name
                    | select(
                        contains($os)
                        and ($arch == "" or contains($arch))
                        and endswith($ext)
                    )' \
            | head -n 1
    )

    if [[ -z "$filename" || "$filename" == "null" ]]; then
        color_echo RED "  ❌  No matching asset found (os=$os_tag arch=$arch_tag ext=$extension)"
        return 1
    fi

    local base_url="https://github.com/$repo/releases/download"
    local url="${base_url}/$tag/$filename"
    local tmp_file="/tmp/$filename"
    local extract_dir="/tmp/${bin_name}-extract"

    color_echo YELLOW "  📥  Downloading $filename ($tag)..."
    curl -L "$url" -o "$tmp_file"

    mkdir -p "$HOME/.local/bin"

    case "$filename" in
        *.deb)
            sudo dpkg -i "$tmp_file"
            rm -f "$tmp_file"
            color_echo GREEN "  ✅  $bin_name $tag installed via dpkg"
            return 0
            ;;
        *.tar.gz|*.tgz)
            mkdir -p "$extract_dir"
            tar -xzf "$tmp_file" -C "$extract_dir"
            ;;
        *.tar.xz)
            mkdir -p "$extract_dir"
            tar -xf "$tmp_file" -C "$extract_dir"
            ;;
        *.zip)
            mkdir -p "$extract_dir"
            unzip -o "$tmp_file" -d "$extract_dir"
            ;;
        *.gz)
            gunzip -c "$tmp_file" > "$HOME/.local/bin/$bin_name"
            chmod +x "$HOME/.local/bin/$bin_name"
            rm -f "$tmp_file"
            color_echo GREEN "  ✅  $bin_name $tag installed to ~/.local/bin"
            return 0
            ;;
        *)
            cp "$tmp_file" "$HOME/.local/bin/$bin_name"
            chmod +x "$HOME/.local/bin/$bin_name"
            rm -f "$tmp_file"
            color_echo GREEN "  ✅  $bin_name $tag installed to ~/.local/bin"
            return 0
            ;;
    esac

    local bin_path
    bin_path=$(find "$extract_dir" -name "$bin_name" -type f | head -n 1)

    if [[ -n "$bin_path" ]]; then
        cp "$bin_path" "$HOME/.local/bin/$bin_name"
        chmod +x "$HOME/.local/bin/$bin_name"
        color_echo GREEN "  ✅  $bin_name $tag installed to ~/.local/bin"
    else
        color_echo RED "  ❌  Failed to find executable '$bin_name' in extracted files"
        rm -rf "$tmp_file" "$extract_dir"
        return 1
    fi

    rm -rf "$tmp_file" "$extract_dir"
}

###############################################################################
# Internet & Command Helpers
###############################################################################

has_internet() {
    ping -c 1 -W 2 google.com >/dev/null 2>&1
}

isinstalled() {
    command -v "$1" >/dev/null 2>&1
}

###############################################################################
# Dotfiles Git Helpers
###############################################################################

_dotfiles_remote_url() {
    local url
    url=$(git -C "$DOTDOTFILES" config remote.origin.url 2>/dev/null || true)
    if [[ -z "$url" && -f "$DOTDOTFILES/.git/wsm-url" ]]; then
        read -r url < "$DOTDOTFILES/.git/wsm-url"
    fi
    printf '%s' "$url"
}

_dotfiles_git() {
    local subcmd="$1" url mode
    shift
    if git -C "$DOTDOTFILES" remote | grep -q .; then
        git -C "$DOTDOTFILES" "$subcmd" "$@"
        return
    fi
    url=$(_dotfiles_remote_url) || return 1
    case "$subcmd" in
        fetch)
            git -C "$DOTDOTFILES" fetch "$url" '+refs/heads/*:refs/remotes/origin/*' "$@"
            ;;
        pull)
            _dotfiles_git fetch
            mode="merge"
            if [[ "$(git -C "$DOTDOTFILES" config --get pull.rebase 2>/dev/null)" == "true" ]]; then
                mode="rebase"
            fi
            git -C "$DOTDOTFILES" "$mode" origin/main "$@"
            ;;
        *) git -C "$DOTDOTFILES" "$subcmd" "$@" ;;
    esac
}

_check_git_health() {
    local d="$DOTDOTFILES"
    if ! git -C "$d" symbolic-ref -q HEAD >/dev/null 2>&1; then
        echo "detached HEAD"; return 1
    fi
    if git -C "$d" rev-parse MERGE_HEAD >/dev/null 2>&1; then
        echo "merge in progress"; return 1
    fi
    if [[ -d "$d/.git/rebase-merge" || -d "$d/.git/rebase-apply" ]]; then
        echo "rebase in progress"; return 1
    fi
    if [[ -n "$(git -C "$d" ls-files -u 2>/dev/null)" ]]; then
        echo "unmerged paths"; return 1
    fi
    return 0
}

_remote_status() {
    local current latest
    latest=$(git -C "$DOTDOTFILES" rev-parse origin/main 2>/dev/null) || { echo "unknown"; return; }
    current=$(git -C "$DOTDOTFILES" rev-parse HEAD 2>/dev/null) || { echo "unknown"; return; }
    if [[ "$current" == "$latest" ]]; then
        echo "up-to-date"
    elif git -C "$DOTDOTFILES" merge-base --is-ancestor "$current" "$latest" 2>/dev/null; then
        echo "behind"
    else
        if git -C "$DOTDOTFILES" merge-base --is-ancestor "$latest" "$current" 2>/dev/null; then
            echo "up-to-date"
        else
            echo "diverged"
        fi
    fi
}

_has_local_changes() {
    local changes
    changes="$(git -C "$DOTDOTFILES" status --porcelain \
        --untracked-files=no \
        --ignore-submodules 2>/dev/null)"
    if [[ -n "$changes" ]]; then
        return 0
    fi
    return 1
}

_has_conflicting_changes() {
    local upstream local_changed
    upstream=$(git -C "$DOTDOTFILES" diff --name-only \
        --ignore-submodules HEAD origin/main 2>/dev/null)
    local_changed=$(git -C "$DOTDOTFILES" diff --name-only \
        --ignore-submodules 2>/dev/null)
    if [[ -z "$upstream" || -z "$local_changed" ]]; then
        return 1
    fi
    comm -12 \
        <(echo "$upstream" | sort) \
        <(echo "$local_changed" | sort) | grep -q .
}

_handle_git_lock() {
    local lock
    lock="$DOTDOTFILES/.git/index.lock"
    if [[ -f "$lock" ]]; then
        rm -f "$lock" 2>/dev/null || true
    fi
    lock="$DOTDOTFILES/.git/objects/info/commit-graphs/commit-graph-chain.lock"
    if [[ -f "$lock" ]]; then
        rm -f "$lock" 2>/dev/null || true
    fi
}

###############################################################################
# Submodule Sync
###############################################################################

_sub_tracking_branch() {
    local d="$1" sub="$2" sub_path="$3"
    local branch
    branch=$(git config -f "$d/.gitmodules" \
        "submodule.${sub}.branch" 2>/dev/null || true)
    if [[ -z "$branch" ]]; then
        if git -C "$sub_path" rev-parse --verify \
            origin/main >/dev/null 2>&1; then
            branch="main"
        elif git -C "$sub_path" rev-parse --verify \
            origin/master >/dev/null 2>&1; then
            branch="master"
        else
            branch="main"
        fi
    fi
    printf '%s' "$branch"
}

###############################################################################
# Logging
###############################################################################

# Set up per-worker logging. Creates ~/.cache/dotfiles/<name>.log and
# exports DOTFILES_LOG so dotfiles_notify and nested calls inherit it.
dotfiles_log_init() {
    local name="$1"
    export DOTFILES_LOG="$HOME/.cache/dotfiles/${name}.log"
    mkdir -p "${DOTFILES_LOG%/*}"
    dotfiles_log "--- $(date '+%Y-%m-%d %H:%M:%S') $name ---"
}

# Timestamped log entry. Always writes to $DOTFILES_LOG when set.
# Also prints to terminal when stdout is a tty.
dotfiles_log() {
    local line="[$(date '+%Y-%m-%d %H:%M:%S')] $*"
    if [[ -n "${DOTFILES_LOG:-}" ]]; then
        printf '%s\n' "$line" >> "$DOTFILES_LOG"
    fi
    if [[ -t 1 ]]; then
        printf '%s\n' "$line"
    fi
    return 0
}

# Run a command with output routed through the logging system.
# Background: output goes to log only. Interactive: tee to both.
dotfiles_run() {
    if [[ -n "${DOTFILES_LOG:-}" && -t 1 ]]; then
        "$@" 2>&1 | tee -a "$DOTFILES_LOG"
    elif [[ -n "${DOTFILES_LOG:-}" ]]; then
        "$@" >> "$DOTFILES_LOG" 2>&1
    else
        "$@"
    fi
}

###############################################################################
# Notifications
###############################################################################

DOTFILES_NOTIFY_FILE="$HOME/.cache/dotfiles/notifications"

# Queue a notification for display at next interactive login.
#   dotfiles_notify <level> <message> [logfile]
# Levels: success, info, warn, error
# Log file falls back to $DOTFILES_LOG when the third argument is
# omitted. Format on disk: level|logfile|message (logfile is the
# second field so message, which may contain |, is always the
# remainder parsed by read).
dotfiles_notify() {
    local level="$1" msg="$2" logfile="${3:-${DOTFILES_LOG:-}}"
    mkdir -p "${DOTFILES_NOTIFY_FILE%/*}"
    printf '%s|%s|%s\n' "$level" "$logfile" "$msg" >> "$DOTFILES_NOTIFY_FILE"
    if [[ "$level" == "warn" || "$level" == "error" ]]; then
        echo "$level: $msg" >&2
    fi
    return 0
}

_sync_one_submodule() {
    local d="$1" sub="$2"
    local sub_path="$d/$sub"
    if [[ ! -d "$sub_path/.git" && ! -f "$sub_path/.git" ]]; then
        return 0
    fi

    local branch
    branch=$(_sub_tracking_branch "$d" "$sub" "$sub_path")

    dotfiles_log "syncing $sub (branch: $branch)"
    dotfiles_run git -C "$sub_path" fetch || return 0

    local stashed=false
    if [[ "$sub" == "lib/scripts" ]]; then
        local _status_out
        _status_out="$(git -C "$sub_path" status --porcelain 2>/dev/null)"
        if [[ -n "$_status_out" ]]; then
            stashed=true
            dotfiles_run git -C "$sub_path" stash --include-untracked || {
                dotfiles_notify warn "stash failed in $sub"
                return 0
            }
        fi
    fi

    dotfiles_run git -C "$sub_path" checkout "$branch" || true

    if ! dotfiles_run git -C "$sub_path" pull --rebase origin "$branch"; then
        dotfiles_run git -C "$sub_path" rebase --abort || true
        dotfiles_notify warn \
            "pull --rebase failed in $sub, local state preserved"
    fi

    if [[ "$stashed" == true ]]; then
        if ! dotfiles_run git -C "$sub_path" stash pop; then
            dotfiles_run git -C "$sub_path" checkout -- . || true
            dotfiles_notify warn \
                "stash pop conflict in $sub, recover with: cd $sub && git stash pop"
        fi
    fi
}

# Pull all submodules to their latest upstream commit.
# lib/scripts: stash local work (including untracked), rebase,
# pop. Others: just pull. Auto-commits pointer updates in the
# parent only when the index is otherwise clean.
_sync_submodules() {
    local d="$DOTDOTFILES"
    dotfiles_run git -C "$d" submodule update --init || true

    local sub
    for sub in lib/zinit lib/scripts lib/zsh-defer; do
        _sync_one_submodule "$d" "$sub"
    done

    # Only auto-commit pointer updates when the parent index
    # has no other staged changes; avoids hijacking unrelated work.
    if [[ -n "$(git -C "$d" diff --cached --name-only \
        --ignore-submodules 2>/dev/null)" ]]; then
        return 0
    fi

    local pointer_dirty=false
    for sub in lib/zinit lib/scripts lib/zsh-defer; do
        if ! git -C "$d" diff --quiet -- "$sub" 2>/dev/null; then
            git -C "$d" add -- "$sub"
            pointer_dirty=true
        fi
    done
    if [[ "$pointer_dirty" == true ]]; then
        dotfiles_log "committing submodule pointer updates"
        dotfiles_run git -C "$d" commit \
            -m "Update submodule pointers" \
            -- lib/zinit lib/scripts lib/zsh-defer || true
    fi
}

###############################################################################
# Dotfiles Update
###############################################################################

dotfiles_update_repo() {
    local d="$DOTDOTFILES"
    local reason remote_status has_changes pre_pull_head

    reason=$(_check_git_health 2>&1) || {
        dotfiles_log "skip: $reason"
        echo "skip: $reason" >&2
        return 1
    }

    _handle_git_lock

    dotfiles_log "fetching origin"
    dotfiles_run _dotfiles_git fetch || {
        dotfiles_log "fetch failed"
        echo "fetch failed" >&2
        return 1
    }

    remote_status=$(_remote_status)
    dotfiles_log "remote status: $remote_status"
    case "$remote_status" in
        up-to-date)
            _sync_submodules
            return 0
            ;;
        diverged)
            echo "local history diverged from origin/main, needs manual fix" >&2
            return 1
            ;;
        behind) ;;
        *)
            echo "unable to determine remote status" >&2
            return 1
            ;;
    esac

    has_changes=false
    if _has_local_changes; then
        has_changes=true
    fi

    if [[ "$has_changes" == true ]]; then
        if _has_conflicting_changes; then
            echo "upstream changes conflict with local work (overlapping files)" >&2
            return 1
        fi
        dotfiles_run git -C "$d" stash || {
            echo "stash failed" >&2
            return 1
        }
    fi

    pre_pull_head=$(git -C "$d" rev-parse HEAD)
    dotfiles_log "pulling (pre: ${pre_pull_head:0:8})"

    if ! dotfiles_run _dotfiles_git pull --ff; then
        dotfiles_run git -C "$d" reset --hard "$pre_pull_head"
        if [[ "$has_changes" == true ]]; then
            dotfiles_run git -C "$d" stash pop || true
        fi
        echo "pull failed, rolled back" >&2
        return 1
    fi

    if [[ "$has_changes" == true ]]; then
        if ! dotfiles_run git -C "$d" stash pop; then
            dotfiles_run git -C "$d" reset --hard "$pre_pull_head"
            dotfiles_run git -C "$d" stash pop || true
            echo "stash pop failed after pull, rolled back to pre-update state" >&2
            return 1
        fi
    fi

    local post_pull_head
    post_pull_head=$(git -C "$d" rev-parse HEAD)
    if [[ "$pre_pull_head" != "$post_pull_head" ]]; then
        dotfiles_log "updated ${pre_pull_head:0:8} -> ${post_pull_head:0:8}"
        echo "pulled:${pre_pull_head}:${post_pull_head}"
    fi

    # tools.bash is already sourced once at shell startup so this function
    # exists, but the pull above may have delivered a newer version on disk.
    # Re-source so _sync_submodules (and everything else) runs the updated code.
    source "$DOTDOTFILES/bash/core/tools.bash"

    _sync_submodules
    return 0
}
