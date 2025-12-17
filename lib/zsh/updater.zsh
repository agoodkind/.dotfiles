#!/usr/bin/env zsh
# shellcheck shell=bash
# Background dotfiles updater - checks for updates and applies them silently

source "$DOTDOTFILES/lib/zsh/utils.zsh"

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
    
    # Checkout main branch in scripts submodule so it's not detached
    git -C "$DOTDOTFILES/lib/scripts" checkout main --quiet 2>/dev/null || true
    
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
    
    local last_update now
    last_update=$(stat -f %m "$WEEKLY_TIMESTAMP" 2>/dev/null) || \
        last_update=$(stat -c %Y "$WEEKLY_TIMESTAMP" 2>/dev/null) || return 0
    now=$(date +%s)
    
    (( now - last_update > WEEKLY_SECONDS ))
}

do_weekly_update() {
    log "Weekly full update started"
    
    cd "$DOTDOTFILES" || { 
        error "Failed to cd to dotfiles"
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
    
    # Homebrew cleanup (macOS only)
    if [[ "$OSTYPE" == darwin* ]] && (( $+commands[brew] )); then
        log "Running brew cleanup"
        brew cleanup --prune=all 2>&1 >> "$LOG_FILE" || true
        log "brew cleanup completed"
    fi
    
    # Update timestamp
    touch "$WEEKLY_TIMESTAMP"
    log "Weekly full update completed"
    touch "$HOME/.cache/dotfiles_weekly_update_success"
}

# Main
fetch_latest || exit 0

if is_behind_remote; then
    do_update
fi

# Check for weekly full update (independent of git changes)
# Run async so it doesn't block shell startup
if needs_weekly_update; then
    async_run do_weekly_update
fi
