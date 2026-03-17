#!/usr/bin/env bash
DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

LOG_FILE="$HOME/.cache/dotfiles_dispatch.log"
mkdir -p "$(dirname "$LOG_FILE")"

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$LOG_FILE"; }

# Atomic lock: mkdir is atomic on all filesystems, so only the first
# terminal to create the directory wins. Every other dispatch exits
# immediately, making it safe to spam new tabs.
LOCK_DIR="$HOME/.cache/dotfiles_dispatch.lock"
LOCK_PID_FILE="$LOCK_DIR/pid"

_dispatch_stale_lock() {
    [[ -f "$LOCK_PID_FILE" ]] || return 0
    local lock_pid
    lock_pid=$(<"$LOCK_PID_FILE") 2>/dev/null || return 0
    kill -0 "$lock_pid" 2>/dev/null && return 1
    return 0
}

if ! mkdir "$LOCK_DIR" 2>/dev/null; then
    if _dispatch_stale_lock; then
        log "removing stale dispatch lock (owner pid gone)"
        rm -rf "$LOCK_DIR"
        mkdir "$LOCK_DIR" 2>/dev/null || { log "lock contention, exiting"; exit 0; }
    else
        log "another dispatch already running, exiting"
        exit 0
    fi
fi
echo $$ > "$LOCK_PID_FILE"

pids=()
jobs=()

cleanup() {
    for pid in "${pids[@]}"; do
        kill "$pid" 2>/dev/null
    done
    rm -rf "$LOCK_DIR"
}
trap 'cleanup; exit 130' INT TERM

# Export so updater.bash can write status ("sync"/"weekly") for incl.zsh
export DOTFILES_DISPATCH_LOCK_DIR="$LOCK_DIR"

launch() {
    local script="$1"
    bash "$script" &
    local pid=$!
    pids+=("$pid")
    jobs+=("$script:$pid")
    log "started $(basename "$script") (pid $pid)"
}

launch "$DOTDOTFILES/bash/background/updater.bash"
launch "$DOTDOTFILES/bash/background/prefer-cache-rebuild.bash"
launch "$DOTDOTFILES/bash/background/path-cache-rebuild.bash"
launch "$DOTDOTFILES/bash/background/zwc-recompile.bash"
launch "$DOTDOTFILES/bash/background/ssh-key-load-mac.bash"

for entry in "${jobs[@]}"; do
    local_script="${entry%%:*}"
    local_pid="${entry##*:}"
    wait "$local_pid" || true
    log "finished $(basename "$local_script") (pid $local_pid)"
done

rm -rf "$LOCK_DIR"
trap - EXIT INT TERM
