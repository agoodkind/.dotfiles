#!/usr/bin/env bash
DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
# shellcheck source=bash/core/init.bash
source "$DOTDOTFILES/bash/core/init.bash"

LOG_FILE="$HOME/.cache/dotfiles_dispatch.log"
mkdir -p "$(dirname "$LOG_FILE")"

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$LOG_FILE"; }

# flock on a regular file provides kernel-enforced mutual exclusion.
# The lock is released automatically when this process exits for any reason,
# including SIGKILL, so no stale-lock recovery logic is needed.
LOCK_FILE="$HOME/.cache/dotfiles_dispatch.flock"
# Lock dir is a status sentinel only -- incl.zsh reads it to show
# "running in background" and the status file inside it.
LOCK_DIR="$HOME/.cache/dotfiles_dispatch.lock"

exec 9>"$LOCK_FILE"
if ! flock -n 9; then
    log "another dispatch already running, exiting"
    exit 0
fi

# Create status dir after acquiring the lock so incl.zsh can detect
# a running dispatch and read the optional status file.
mkdir -p "$LOCK_DIR"
echo $$ > "$LOCK_DIR/pid"

pids=()
jobs=()

cleanup() {
    for pid in "${pids[@]}"; do
        kill "$pid" 2>/dev/null
    done
    rm -rf "$LOCK_DIR"
}
trap 'cleanup' EXIT
trap 'exit 130' INT TERM

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
