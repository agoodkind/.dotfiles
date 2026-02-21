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
    if is_macos; then
        last_update=$(stat -f %m "$WEEKLY_TIMESTAMP" 2>/dev/null) || return 0
    else
        last_update=$(stat -c %Y "$WEEKLY_TIMESTAMP" 2>/dev/null) || return 0
    fi
    now=$(date +%s)
    (( now - last_update > WEEKLY_SECONDS ))
}

do_sync_only() {
    log "Running sync.sh --quick --skip-git"
    local out
    out=$(USE_DEFAULTS=true "$DOTDOTFILES/sync.sh" --non-interactive --quick --skip-git 2>&1)
    echo "$out" >> "$LOG_FILE"
    log "sync.sh exited"
    touch "$HOME/.cache/dotfiles_update_success"
}

do_weekly_update() {
    log "Weekly full update started"

    local sync_output
    sync_output=$(USE_DEFAULTS=true "$DOTDOTFILES/sync.sh" --non-interactive --repair --skip-git 2>&1)
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

    touch "$WEEKLY_TIMESTAMP"
    log "Weekly full update completed"
    touch "$HOME/.cache/dotfiles_weekly_update_success"
}

main() {
    has_internet || return 0

    local update_output
    if update_output=$(dotfiles_update_repo 2>&1); then
        log "repo updated"
    else
        log "repo update: $update_output"
        if [[ "$update_output" == *"conflict"* ]]; then
            echo "$update_output" > "$HOME/.cache/dotfiles_local_changes"
        fi
    fi

    local repo_updated=true
    [[ -n "$update_output" ]] && repo_updated=false

    local weekly_needed=false
    needs_weekly_update && weekly_needed=true

    if [[ "$repo_updated" == false && "$weekly_needed" == false ]]; then
        return 0
    fi

    if [[ "$weekly_needed" == true ]]; then
        do_weekly_update
    elif [[ "$repo_updated" == true ]]; then
        do_sync_only
    fi
}

main
