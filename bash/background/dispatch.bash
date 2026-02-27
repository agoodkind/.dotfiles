#!/usr/bin/env bash
DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

LOG_FILE="$HOME/.cache/dotfiles_dispatch.log"
mkdir -p "$(dirname "$LOG_FILE")"

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$LOG_FILE"; }

pids=()
jobs=()

cleanup() {
    for pid in "${pids[@]}"; do
        kill "$pid" 2>/dev/null
    done
}
trap 'cleanup; exit 130' INT TERM

launch() {
    local script="$1"
    bash "$script" &
    local pid=$!
    pids+=("$pid")
    jobs+=("$script:$pid")
    log "started $(basename "$script") (pid $pid)"
}

launch "$DOTDOTFILES/bash/background/updater.bash"
launch "$DOTDOTFILES/bash/background/prefer-cache-check.bash"
launch "$DOTDOTFILES/bash/background/zwc-recompile.bash"
launch "$DOTDOTFILES/bash/background/ssh-key-load-mac.bash"

for entry in "${jobs[@]}"; do
    local_script="${entry%%:*}"
    local_pid="${entry##*:}"
    wait "$local_pid" || true
    log "finished $(basename "$local_script") (pid $local_pid)"
done
