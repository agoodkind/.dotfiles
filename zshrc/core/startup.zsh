# Interactive-only startup helpers. Sourced from incl.zsh after the agent and
# non-TTY early return, so agent shells never pay for install-lock, banner, or
# notification display.

# A live install may be writing shell state, so fall back to minimal startup
# while the install flock is held. Returns success when startup should stop.
function _dotfiles_install_in_progress() {
    local install_status_dir=~/.cache/dotfiles_install.lock
    if [[ ! -d "$install_status_dir" ]]; then
        return 1
    fi

    local install_flock=~/.cache/dotfiles_install.flock
    local install_lock_fd=-1

    if [[ ! -e "$install_flock" ]]; then
        rm -rf "$install_status_dir" 2>/dev/null || true
        return 1
    fi

    # The installer only creates this status directory after it acquires the
    # install flock, so a busy flock means the install is still live.
    zmodload -F zsh/system b:zsystem
    if ! zsystem flock -t 0 -f install_lock_fd "$install_flock"; then
        print -P "%F{blue}↻ dotfiles install is running in another terminal, so this shell is using minimal startup%f"
        return 0
    fi
    zsystem flock -u $install_lock_fd

    # If the flock is free, the marker directory is not enough on its own and
    # can be cleaned up.
    rm -rf "$install_status_dir" 2>/dev/null || true
    return 1
}

# Show transient "running now" state from background dispatch.
function _dotfiles_show_dispatch_banner() {
    if [[ ! -d ~/.cache/dotfiles_dispatch.lock ]]; then
        return 0
    fi

    local update_type=""
    if [[ -f ~/.cache/dotfiles_dispatch.lock/status ]]; then
        update_type=$(<~/.cache/dotfiles_dispatch.lock/status)
    fi
    if [[ "$update_type" == "weekly" ]]; then
        print -P "%F{blue}↻ weekly update running in background%f"
    elif [[ "$update_type" == "sync" ]]; then
        print -P "%F{blue}↻ dotfiles sync running in background%f"
    fi
}

# Display all queued notifications from background processes, one per line.
# Format on disk, written only by telemetry.Notify: timestamp|level|logfile|runid|message
# The message is the final field and may itself contain '|'.
function _dotfiles_show_notifications() {
    local notify_file="$HOME/.cache/dotfiles/notifications"
    if [[ ! -f "$notify_file" ]]; then
        return 0
    fi

    # Move the file aside before reading, so a concurrent writer's new lines land
    # in a fresh file instead of being dropped by the rm below.
    local notify_staged="${notify_file}.$$"
    if ! mv "$notify_file" "$notify_staged" 2>/dev/null; then
        return 0
    fi

    local created_at level logfile runid msg rest line idtag display_msg
    while IFS= read -r line; do
        created_at="${line%%|*}"
        rest="${line#*|}"
        level="${rest%%|*}"
        rest="${rest#*|}"
        logfile="${rest%%|*}"
        rest="${rest#*|}"
        runid="${rest%%|*}"
        msg="${rest#*|}"
        msg="${msg//\%/%%}"
        idtag=""
        if [[ -n "$runid" ]]; then
            idtag="%F{242}[#${runid[1,12]}]%f "
        fi
        display_msg="%F{242}${created_at}%f ${idtag}${msg}"
        case "$level" in
            success) print -P "%F{green}✓ ${display_msg}%f" ;;
            info) print -P "%F{blue}↻ ${display_msg}%f" ;;
            warn) print -P "%F{yellow}⚠  ${display_msg}%f" ;;
            error) print -P "%F{red}✗ ${display_msg}%f" ;;
        esac
        if [[ -n "$logfile" && -f "$logfile" ]]; then
            print -P "  %F{242}log: ${logfile//\%/%%}%f"
        fi
    done <"$notify_staged"
    rm -f "$notify_staged"
}
