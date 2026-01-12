#!/usr/bin/env zsh
# shellcheck shell=bash
# Background dotfiles updater - checks for updates and applies them silently

source "$DOTDOTFILES/lib/shell/zsh/utils.zsh"

# Prevent concurrent runs
LOCK_FILE="$HOME/.cache/dotfiles_update.lock"
if [[ -f "$LOCK_FILE" ]]; then
    # Check if lock is stale (older than 10 minutes)
    if [[ $(find "$LOCK_FILE" -mmin +10 2>/dev/null) ]]; then
        rm -f "$LOCK_FILE"
    else
        exit 0
    fi
fi

mkdir -p ~/.cache
touch "$LOCK_FILE"
trap "rm -f '$LOCK_FILE'" EXIT

LOG_FILE="$HOME/.cache/dotfiles_update.log"
ERROR_FILE="$HOME/.cache/dotfiles_update_error"
DOTFILES_GIT_DIR="$DOTDOTFILES/.git"

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$LOG_FILE"; }
error() { echo "$*" >> "$ERROR_FILE"; log "ERROR: $*"; }

run_logged() {
    local label="$1"
    shift

    log "$label"
    "$@" >> "$LOG_FILE" 2>&1
    local exit_code=$?
    log "$label exited with code $exit_code"
    return $exit_code
}

do_brew_upgrade() {
    is_macos || return 0
    isinstalled brew || return 0

    run_logged "Running brew update" brew update || true
    run_logged "Running brew upgrade" brew upgrade || true
    run_logged "Running brew upgrade --cask" brew upgrade --cask || true
}

do_apt_upgrade() {
    is_debian_based || return 0
    isinstalled apt-get || return 0

    if ! sudo -n true >> "$LOG_FILE" 2>&1; then
        log "Skipping apt upgrade (sudo requires a password)"
        return 0
    fi

    run_logged "Running apt-get update" sudo -n apt-get update || true
    local -a dpkg_opts=(
        -o Dpkg::Options::=--force-confdef
        -o Dpkg::Options::=--force-confold
    )

    run_logged "Running apt-get dist-upgrade" \
        sudo -n env DEBIAN_FRONTEND=noninteractive \
        apt-get -y "${dpkg_opts[@]}" dist-upgrade || true
    run_logged "Running apt-get autoremove" sudo -n apt-get -y autoremove || true
}

# Fetch latest from remote
fetch_latest() {
    git --git-dir="$DOTFILES_GIT_DIR" --work-tree="$DOTDOTFILES" fetch --quiet --all 2>/dev/null && return 0
    
    # If fetch failed, try removing commit-graph lock file and retry
    local lock_file="$DOTFILES_GIT_DIR/objects/info/commit-graphs/commit-graph-chain.lock"
    if [[ -f "$lock_file" ]]; then
        rm -f "$lock_file" 2>/dev/null
        git --git-dir="$DOTFILES_GIT_DIR" --work-tree="$DOTDOTFILES" fetch --quiet --all 2>/dev/null || return 1
    else
        return 1
    fi
}

# Check if local is behind remote
is_behind_remote() {
    local current_hash latest_hash
    latest_hash=$(git --git-dir="$DOTFILES_GIT_DIR" rev-parse origin/main 2>/dev/null) || return 1
    current_hash=$(git --git-dir="$DOTFILES_GIT_DIR" rev-parse HEAD 2>/dev/null) || return 1
    
    [[ "$current_hash" == "$latest_hash" ]] && return 1
    
    git --git-dir="$DOTFILES_GIT_DIR" merge-base --is-ancestor "$current_hash" "$latest_hash" 2>/dev/null
}

# Check for local changes (modified tracked files)
has_local_changes() {
    [[ -n $(git --git-dir="$DOTFILES_GIT_DIR" --work-tree="$DOTDOTFILES" status --porcelain --untracked-files=no) ]]
}

# Update repo files (git pull, submodules)
update_repo_files() {
    cd "$DOTDOTFILES" || { error "Failed to cd to dotfiles"; return 1; }
    
    # Pull latest
    if ! git pull --quiet 2>&1 >> "$LOG_FILE"; then
        error "git pull failed"
        return 1
    fi
    
    # Update submodules
    if ! git submodule update --init --recursive --quiet 2>&1 >> "$LOG_FILE"; then
        error "submodule update failed"
        return 1
    fi
    
    # Checkout main branch in scripts submodule so it's not detached
    git -C "$DOTDOTFILES/lib/scripts" checkout main --quiet 2>/dev/null || true
    return 0
}

# Perform the update
do_update() {
    log "Update started"
    
    update_repo_files || return 1
    
    # Run sync (non-interactive)
    log "Running sync.sh"
    local sync_output
    sync_output=$(USE_DEFAULTS=true "$DOTDOTFILES/sync.sh" --non-interactive --quick 2>&1)
    local sync_exit=$?
    echo "$sync_output" >> "$LOG_FILE"
    log "sync.sh exited with code $sync_exit"
    if [[ $sync_exit -ne 0 ]]; then
        error "sync.sh failed"
        return 1
    fi
    
    log "Update completed successfully"
    touch "$HOME/.cache/dotfiles_update_success"
}

# Weekly full update (repair + zinit + nvim)
WEEKLY_TIMESTAMP="$HOME/.cache/dotfiles_weekly_update"
WEEKLY_SECONDS=$((7 * 24 * 60 * 60))  # 7 days

needs_weekly_update() {
    [[ ! -f "$WEEKLY_TIMESTAMP" ]] && return 0
    
    # Use zsh/stat for fast, consistent, cross-platform timestamp checking
    zmodload -F zsh/stat b:zstat 2>/dev/null
    if (( ! $+builtins[zstat] )); then
        # Should not happen if environment is set up, but safe fallback
        return 0
    fi

    local -a file_stat
    zstat -A file_stat +mtime "$WEEKLY_TIMESTAMP" 2>/dev/null || return 0
    local last_update=$file_stat[1]
    local now=$EPOCHSECONDS
    
    (( now - last_update > WEEKLY_SECONDS ))
}

do_weekly_update() {
    log "Weekly full update started"
    
    # Ensure repo is up to date first
    update_repo_files || {
        error "Failed to update repo during weekly update"
        return 1
    }
    
    # Full sync with repair (not quick)
    log "Running sync.sh --repair"
    local sync_output
    sync_output=$(USE_DEFAULTS=true "$DOTDOTFILES/sync.sh" --non-interactive --repair 2>&1)
    local sync_exit=$?
    echo "$sync_output" >> "$LOG_FILE"
    log "sync.sh --repair exited with code $sync_exit"
    
    # Update zinit (self + plugins)
    log "Updating zinit"
    if (( $+commands[zinit] )) || [[ -f "$DOTDOTFILES/lib/zinit/zinit.zsh" ]]; then
        source "$DOTDOTFILES/lib/zinit/zinit.zsh" 2>/dev/null
        zinit self-update 2>&1 >> "$LOG_FILE" || true
        zinit update --all --quiet 2>&1 >> "$LOG_FILE" || true
        log "zinit update completed"
    fi

    do_apt_upgrade
    do_brew_upgrade
    
    # Homebrew cleanup (macOS only)
    if is_macos && isinstalled brew; then
        log "Running brew cleanup"
        brew cleanup --prune=all 2>&1 >> "$LOG_FILE" || true
        log "brew cleanup completed"
    fi
    
    # Update timestamp
    touch "$WEEKLY_TIMESTAMP"
    log "Weekly full update completed"
    touch "$HOME/.cache/dotfiles_weekly_update_success"
}

# Main entry point
main() {
    # Check for updates from remote
    fetch_latest || return 0

    local updates_available=false
    is_behind_remote && updates_available=true

    local weekly_needed=false
    needs_weekly_update && weekly_needed=true

    # If no updates are needed, exit early
    if [[ "$updates_available" == false && "$weekly_needed" == false ]]; then
        return 0
    fi

    # If upstream updates are available and we have local changes, warn and stop.
    # We block ONLY when there are incoming git changes that might conflict.
    # If local changes exist but we are up-to-date with upstream, we allow
    # weekly maintenance (package updates) to proceed.
    if [[ "$updates_available" == true ]] && has_local_changes; then
        local msg="upstream updates available but local changes detected. please clean working state"
        echo "$msg" > "$HOME/.cache/dotfiles_local_changes"
        return 0
    fi

    # Perform updates if safe
    if [[ "$weekly_needed" == true ]]; then
        # Weekly update includes repo update + full sync + system updates
        do_weekly_update
    elif [[ "$updates_available" == true ]]; then
        # Only normal update needed
        do_update
    fi
}

main
