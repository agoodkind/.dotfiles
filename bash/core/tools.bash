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
    [[ "$(uname -s)" == "Linux" ]] && return 0
    [[ -n "${1:-}" ]] && color_echo YELLOW "  ⏭  $1"
    return 1
}

# Returns 0 on macOS, 1 otherwise. Optional msg: echo skip reason when not macOS.
# Usage: is_macos "reason" || return
is_macos() {
    [[ "$(uname -s)" == "Darwin" ]] && return 0
    [[ -n "${1:-}" ]] && color_echo YELLOW "  ⏭  $1"
    return 1
}

# Returns 0 on Debian/Ubuntu, 1 otherwise.
is_ubuntu() {
    [[ -f /etc/os-release ]] && grep -qiE 'ubuntu|debian' /etc/os-release
}

# Returns 0 if running on work laptop (WORK_DIR_PATH set).
is_work_laptop() {
    [[ -n "${WORK_DIR_PATH:-}" ]]
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
    command -v sudo >/dev/null 2>&1 && (sudo -n true 2>/dev/null || sudo -v 2>/dev/null)
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
# Usage: install_from_github "owner/repo" "asset_pattern" "bin_name"
install_from_github() {
    local repo="$1"
    local pattern="$2"
    local bin_name="$3"

    local release_data
    release_data=$(get_github_release_data "$repo")

    if [[ -z "$release_data" ]]; then
        color_echo RED "  ❌  No releases with assets found for $repo"
        return 1
    fi

    local tag
    tag=$(echo "$release_data" | jq -r .tag_name)

    local filename
    filename=$(echo "$release_data" |
        jq -r ".assets[].name | select($pattern)" | head -n 1)

    if [[ -z "$filename" || "$filename" == "null" ]]; then
        color_echo RED "  ❌  No matching asset found for pattern: $pattern"
        return 1
    fi

    local base_url="https://github.com/$repo/releases/download"
    local url="${base_url}/$tag/$filename"
    local tmp_file="/tmp/$filename"
    local extract_dir="/tmp/${bin_name}-extract"

    color_echo YELLOW "  📥  Downloading $filename ($tag)..."
    curl -L "$url" -o "$tmp_file"

    mkdir -p "$HOME/.cargo/bin"

    case "$filename" in
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
            gunzip -c "$tmp_file" > "$HOME/.cargo/bin/$bin_name"
            chmod +x "$HOME/.cargo/bin/$bin_name"
            rm -f "$tmp_file"
            color_echo GREEN "  ✅  $bin_name $tag installed to ~/.cargo/bin"
            return 0
            ;;
        *)
            cp "$tmp_file" "$HOME/.cargo/bin/$bin_name"
            chmod +x "$HOME/.cargo/bin/$bin_name"
            rm -f "$tmp_file"
            color_echo GREEN "  ✅  $bin_name $tag installed to ~/.cargo/bin"
            return 0
            ;;
    esac

    # Find the binary in the extract directory - use -executable instead of -perm +111
    local bin_path
    bin_path=$(find "$extract_dir" -name "$bin_name" \
        -type f -executable | head -n 1)

    if [[ -x "$bin_path" ]]; then
        cp "$bin_path" "$HOME/.cargo/bin/$bin_name"
        chmod +x "$HOME/.cargo/bin/$bin_name"
        color_echo GREEN "  ✅  $bin_name $tag installed to ~/.cargo/bin"
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
            [[ "$(git -C "$DOTDOTFILES" config --get pull.rebase 2>/dev/null)" == "true" ]] && mode="rebase"
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
    [[ -n "$(git -C "$DOTDOTFILES" status --porcelain \
        --untracked-files=no \
        --ignore-submodules 2>/dev/null)" ]]
}

_has_conflicting_changes() {
    local upstream local_changed
    upstream=$(git -C "$DOTDOTFILES" diff --name-only \
        --ignore-submodules HEAD origin/main 2>/dev/null)
    local_changed=$(git -C "$DOTDOTFILES" diff --name-only \
        --ignore-submodules 2>/dev/null)
    [[ -z "$upstream" || -z "$local_changed" ]] && return 1
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

DOTFILES_NOTIFY_FILE="$HOME/.cache/dotfiles/notifications"

# Write a structured notification for display at next login.
# Levels: success (green), info (blue), warn (yellow), error (red).
# warn and error are also echoed to stderr for interactive visibility.
dotfiles_notify() {
    local level="$1" msg="$2"
    mkdir -p "${DOTFILES_NOTIFY_FILE%/*}"
    printf '%s|%s\n' "$level" "$msg" >> "$DOTFILES_NOTIFY_FILE"
    [[ "$level" == "warn" || "$level" == "error" ]] \
        && echo "$level: $msg" >&2
}

_sync_one_submodule() {
    local d="$1" sub="$2"
    local sub_path="$d/$sub"
    [[ -d "$sub_path/.git" || -f "$sub_path/.git" ]] \
        || return 0

    local branch
    branch=$(_sub_tracking_branch "$d" "$sub" "$sub_path")

    git -C "$sub_path" fetch --quiet \
        2>/dev/null || return 0

    local stashed=false
    if [[ "$sub" == "lib/scripts" ]] && \
       [[ -n "$(git -C "$sub_path" \
           status --porcelain 2>/dev/null)" ]]; then
        stashed=true
        git -C "$sub_path" stash --include-untracked \
            --quiet 2>/dev/null || {
            dotfiles_notify warn "stash failed in $sub"
            return 0
        }
    fi

    git -C "$sub_path" checkout "$branch" --quiet \
        2>/dev/null || true

    if ! git -C "$sub_path" pull --rebase --quiet \
        2>/dev/null; then
        git -C "$sub_path" rebase --abort \
            2>/dev/null || true
        dotfiles_notify warn \
            "pull --rebase failed in $sub, local state preserved"
    fi

    if [[ "$stashed" == true ]]; then
        if ! git -C "$sub_path" stash pop --quiet \
            2>/dev/null; then
            git -C "$sub_path" checkout -- . \
                2>/dev/null || true
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
    git -C "$d" submodule update --init --quiet \
        2>/dev/null || true

    local sub
    for sub in lib/zinit lib/scripts lib/zsh-defer; do
        _sync_one_submodule "$d" "$sub"
    done

    # Only auto-commit pointer updates when the parent index
    # has no other staged changes — avoids hijacking unrelated
    # work (especially from the background updater).
    if [[ -n "$(git -C "$d" diff --cached --name-only \
        --ignore-submodules 2>/dev/null)" ]]; then
        return 0
    fi

    local pointer_dirty=false
    for sub in lib/zinit lib/scripts lib/zsh-defer; do
        if ! git -C "$d" diff --quiet \
            -- "$sub" 2>/dev/null; then
            git -C "$d" add -- "$sub"
            pointer_dirty=true
        fi
    done
    if [[ "$pointer_dirty" == true ]]; then
        git -C "$d" commit --quiet \
            -m "Update submodule pointers" \
            -- lib/zinit lib/scripts lib/zsh-defer \
            2>/dev/null || true
    fi
}

###############################################################################
# Dotfiles Update
###############################################################################

dotfiles_update_repo() {
    local d="$DOTDOTFILES"
    local reason remote_status has_changes pre_pull_head

    reason=$(_check_git_health 2>&1) || {
        echo "skip: $reason" >&2
        return 1
    }

    _handle_git_lock

    _dotfiles_git fetch --quiet || {
        echo "fetch failed" >&2
        return 1
    }

    remote_status=$(_remote_status)
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
    _has_local_changes && has_changes=true

    if [[ "$has_changes" == true ]]; then
        if _has_conflicting_changes; then
            echo "upstream changes conflict with local work (overlapping files)" >&2
            return 1
        fi
        git -C "$d" stash --quiet || {
            echo "stash failed" >&2
            return 1
        }
    fi

    pre_pull_head=$(git -C "$d" rev-parse HEAD)

    if ! _dotfiles_git pull --ff --quiet; then
        git -C "$d" reset --hard "$pre_pull_head" --quiet 2>/dev/null
        if [[ "$has_changes" == true ]]; then
            git -C "$d" stash pop --quiet 2>/dev/null || true
        fi
        echo "pull failed, rolled back" >&2
        return 1
    fi

    if [[ "$has_changes" == true ]]; then
        if ! git -C "$d" stash pop --quiet 2>/dev/null; then
            git -C "$d" reset --hard "$pre_pull_head" --quiet 2>/dev/null
            git -C "$d" stash pop --quiet 2>/dev/null || true
            echo "stash pop failed after pull, rolled back to pre-update state" >&2
            return 1
        fi
    fi

    local post_pull_head
    post_pull_head=$(git -C "$d" rev-parse HEAD)
    if [[ "$pre_pull_head" != "$post_pull_head" ]]; then
        echo "pulled:${pre_pull_head}:${post_pull_head}"
    fi

    _sync_submodules
    return 0
}
