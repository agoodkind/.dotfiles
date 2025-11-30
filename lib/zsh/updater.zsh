#!/usr/bin/env zsh
# shellcheck shell=bash
# Background dotfiles updater - checks for updates and applies them silently

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

# Perform the update
do_update() {
    log "Update started"
    
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
    
    # Run sync (non-interactive)
    if ! USE_DEFAULTS=true "$DOTDOTFILES/sync.sh" --non-interactive >> "$LOG_FILE" 2>&1; then
        error "sync.sh failed"
        return 1
    fi
    
    log "Update completed successfully"
    touch "$HOME/.cache/dotfiles_update_success"
}

# Main
fetch_latest || exit 0

if is_behind_remote; then
    do_update
fi

