#!/usr/bin/env bash
# Background dotfiles updater - checks for updates and applies them silently

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "$DOTDOTFILES/bash/core/tools.bash"
source "$DOTDOTFILES/bash/core/colors.bash"

dotfiles_log_init "updater"

WEEKLY_TIMESTAMP="$HOME/.cache/dotfiles_weekly_update"
WEEKLY_SECONDS=$((7 * 24 * 60 * 60))

do_brew_upgrade() {
    is_macos || return 0
    isinstalled brew || return 0

    dotfiles_log "brew update"
    dotfiles_run brew update || true
    dotfiles_log "brew upgrade"
    dotfiles_run brew upgrade || true
    dotfiles_log "brew upgrade --cask"
    dotfiles_run brew upgrade --cask || true

    if isinstalled brew; then
        dotfiles_log "brew cleanup"
        dotfiles_run brew cleanup --prune=all || true
    fi
}

do_apt_upgrade() {
    is_ubuntu || return 0
    isinstalled apt-get || return 0

    if ! dotfiles_run sudo -n true; then
        dotfiles_log "skipping apt upgrade (sudo requires a password)"
        return 0
    fi

    dotfiles_log "apt-get update"
    dotfiles_run sudo -n apt-get update || true
    local dpkg_opts=(
        -o Dpkg::Options::=--force-confdef
        -o Dpkg::Options::=--force-confold
    )

    dotfiles_log "apt-get dist-upgrade"
    dotfiles_run sudo -n env DEBIAN_FRONTEND=noninteractive \
        apt-get -y "${dpkg_opts[@]}" dist-upgrade || true
    dotfiles_log "apt-get autoremove"
    dotfiles_run sudo -n apt-get -y autoremove || true
}

needs_weekly_update() {
    [[ ! -f "$WEEKLY_TIMESTAMP" ]] && return 0
    local last_update now
    last_update=$(<"$WEEKLY_TIMESTAMP") 2>/dev/null
    if [[ ! "$last_update" =~ ^[0-9]+$ ]]; then
        dotfiles_log "WARN: weekly timestamp file missing or corrupt, migrating to epoch format"
        last_update=$(epoch_now)
        echo "$last_update" > "$WEEKLY_TIMESTAMP"
        return 1
    fi
    now=$(epoch_now)
    (( now - last_update > WEEKLY_SECONDS ))
}

do_sync_only() {
    dotfiles_log "running sync.sh --quick --skip-git"
    dotfiles_run USE_DEFAULTS=true "$DOTDOTFILES/sync.sh" --quick --skip-git
    dotfiles_log "sync.sh exited"
    dotfiles_notify success "Dotfiles updated in background"
}

do_weekly_update() {
    dotfiles_log "weekly full update started"

    dotfiles_run USE_DEFAULTS=true "$DOTDOTFILES/sync.sh" --repair --skip-git
    dotfiles_log "sync.sh --repair exited"

    dotfiles_log "updating zinit"
    if [[ -f "$DOTDOTFILES/lib/zinit/zinit.zsh" ]]; then
        dotfiles_run zsh -c "
            source '$DOTDOTFILES/lib/zinit/zinit.zsh'
            zinit self-update
            zinit update --all --quiet
        " || true
        dotfiles_log "zinit update completed"
    fi

    do_apt_upgrade
    do_brew_upgrade

    epoch_now > "$WEEKLY_TIMESTAMP"
    dotfiles_log "weekly update completed"
    dotfiles_notify success "Weekly full update completed (zinit, nvim, repair)"
}

main() {
    if ! has_internet; then
        dotfiles_log "no internet, skipping"
        return 0
    fi

    dotfiles_log "checking for latest changes"
    local update_output
    if ! update_output=$(dotfiles_update_repo 2>&1); then
        dotfiles_log "fetch failed: $update_output"
        if [[ "$update_output" == *"conflict"* ]]; then
            dotfiles_notify warn "$update_output"
        fi
        return 1
    fi

    local pull_line old_sha new_sha
    pull_line=$(grep '^pulled:' <<< "$update_output" || true)

    if [[ -n "$pull_line" ]]; then
        old_sha="${pull_line#pulled:}"; old_sha="${old_sha%%:*}"
        new_sha="${pull_line##*:}"
        dotfiles_log "new changes found (${old_sha:0:7} -> ${new_sha:0:7})... running sync"
        [[ -n "${DOTFILES_DISPATCH_LOCK_DIR:-}" ]] \
            && echo "sync" > "$DOTFILES_DISPATCH_LOCK_DIR/status"
        do_sync_only
        dotfiles_log "sync completed"
        return 0
    fi

    dotfiles_log "no new changes"
    dotfiles_log "checking if weekly update is due"

    if needs_weekly_update; then
        local last_epoch last_date
        last_epoch=$(<"$WEEKLY_TIMESTAMP")
        last_date=$(epoch_to_date "$last_epoch")
        dotfiles_log "weekly update due (last: $last_date)... running"
        [[ -n "${DOTFILES_DISPATCH_LOCK_DIR:-}" ]] \
            && echo "weekly" > "$DOTFILES_DISPATCH_LOCK_DIR/status"
        do_weekly_update
        return 0
    fi

    dotfiles_log "weekly update not due"
    dotfiles_log "nothing to do"
}

main
