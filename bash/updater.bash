#!/usr/bin/env bash
# Background dotfiles updater - checks for updates and applies them silently

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "$DOTDOTFILES/bash/core/tools.bash"
source "$DOTDOTFILES/bash/core/colors.bash"

LOCK_FILE="$HOME/.cache/dotfiles_update.lock"
if [[ -f "$LOCK_FILE" ]]; then
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
WEEKLY_TIMESTAMP="$HOME/.cache/dotfiles_weekly_update"
WEEKLY_SECONDS=$((7 * 24 * 60 * 60))

log() { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" >> "$LOG_FILE"; }

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

    if isinstalled brew; then
        log "Running brew cleanup"
        brew cleanup --prune=all >> "$LOG_FILE" 2>&1 || true
    fi
}

do_apt_upgrade() {
    is_ubuntu || return 0
    isinstalled apt-get || return 0

    if ! sudo -n true >> "$LOG_FILE" 2>&1; then
        log "Skipping apt upgrade (sudo requires a password)"
        return 0
    fi

    run_logged "Running apt-get update" sudo -n apt-get update || true
    local dpkg_opts=(
        -o Dpkg::Options::=--force-confdef
        -o Dpkg::Options::=--force-confold
    )

    run_logged "Running apt-get dist-upgrade" \
        sudo -n env DEBIAN_FRONTEND=noninteractive \
        apt-get -y "${dpkg_opts[@]}" dist-upgrade || true
    run_logged "Running apt-get autoremove" sudo -n apt-get -y autoremove || true
}

needs_weekly_update() {
    [[ ! -f "$WEEKLY_TIMESTAMP" ]] && return 0
    local last_update now
    last_update=$(<"$WEEKLY_TIMESTAMP") 2>/dev/null
    if [[ ! "$last_update" =~ ^[0-9]+$ ]]; then
        log "WARN: weekly timestamp file missing or corrupt, migrating to epoch format"
        last_update=$(epoch_now)
        echo "$last_update" > "$WEEKLY_TIMESTAMP"
        return 1
    fi
    now=$(epoch_now)
    (( now - last_update > WEEKLY_SECONDS ))
}

do_sync_only() {
    log "Running sync.sh --quick --skip-git"
    local out
    out=$(USE_DEFAULTS=true PROGRESS_NO_TTY=1 "$DOTDOTFILES/sync.sh" --non-interactive --quick --skip-git 2>&1)
    echo "$out" >> "$LOG_FILE"
    log "sync.sh exited"
    touch "$HOME/.cache/dotfiles_update_success"
}

do_weekly_update() {
    log "Weekly full update started"

    local sync_output
    sync_output=$(USE_DEFAULTS=true PROGRESS_NO_TTY=1 "$DOTDOTFILES/sync.sh" --non-interactive --repair --skip-git 2>&1)
    echo "$sync_output" >> "$LOG_FILE"
    log "sync.sh --repair exited"

    log "Updating zinit"
    if [[ -f "$DOTDOTFILES/lib/zinit/zinit.zsh" ]]; then
        zsh -c "
            source '$DOTDOTFILES/lib/zinit/zinit.zsh'
            zinit self-update
            zinit update --all --quiet
        " >> "$LOG_FILE" 2>&1 || true
        log "zinit update completed"
    fi

    do_apt_upgrade
    do_brew_upgrade

    epoch_now > "$WEEKLY_TIMESTAMP"
    log "weekly update completed"
    touch "$HOME/.cache/dotfiles_weekly_update_success"
}

main() {
    if ! has_internet; then
        log "no internet, skipping"
        return 0
    fi

    local pre_head post_head
    pre_head=$(git -C "$DOTDOTFILES" rev-parse HEAD 2>/dev/null)

    log "checking for latest changes"
    local update_output
    if ! update_output=$(dotfiles_update_repo 2>&1); then
        log "fetch failed: $update_output"
        if [[ "$update_output" == *"conflict"* ]]; then
            echo "$update_output" > "$HOME/.cache/dotfiles_local_changes"
        fi
        return 1
    fi

    post_head=$(git -C "$DOTDOTFILES" rev-parse HEAD 2>/dev/null)

    local repo_updated=false
    [[ "$pre_head" != "$post_head" ]] && repo_updated=true

    if [[ "$repo_updated" == true ]]; then
        log "new changes found (${pre_head:0:7} -> ${post_head:0:7})... running sync"
        echo "sync" > "$LOCK_FILE"
        do_sync_only
        log "sync completed"
        return 0
    fi

    log "no new changes"
    log "checking if weekly update is due"

    local weekly_needed=false
    if needs_weekly_update; then
        weekly_needed=true
        local last_epoch last_date
        last_epoch=$(<"$WEEKLY_TIMESTAMP")
        last_date=$(epoch_to_date "$last_epoch")
        log "weekly update due (last: $last_date)... running"
        echo "weekly" > "$LOCK_FILE"
        do_weekly_update
        return 0
    fi

    log "weekly update not due"
    log "nothing to do"
}

main
