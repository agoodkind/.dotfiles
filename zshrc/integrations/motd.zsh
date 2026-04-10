# shellcheck shell=bash
###############################################################################
# MOTD (Message of the Day) and Login Info
#
# Startup hot path is at the bottom. zsh/stat loaded once at file top.
#
# MOTD: runs scripts from lib/motd/ based on cache expiry (new day or N hours)
# LOGIN INFO: caches session in ~/.cache/logininfo_session to skip `who`
###############################################################################

zmodload -F zsh/stat b:zstat 2>/dev/null

MOTD_INTERVAL_HOURS="${MOTD_INTERVAL_HOURS:-12}"
MOTD_CACHE_FILE="$HOME/.cache/motd_last_shown"
MOTD_FORCE_FILE="$HOME/.cache/motd_show_next"
MOTD_DISABLE_SSH="$HOME/.cache/motd_disable_ssh"
LOGININFO_CACHE="$HOME/.cache/logininfo_session"

function _motd_run_scripts() {
    local motd_dir="$DOTDOTFILES/lib/motd"
    if [[ ! -d "$motd_dir" ]]; then
        return 0
    fi
    local script
    for script in "$motd_dir"/*; do
        if [[ -f "$script" && -x "$script" ]]; then
            "$script"
        fi
    done
}

function motd() { _motd_run_scripts; }

function _motd_cache_expired() {
    if ! (($+builtins[zstat])); then
        return 0
    fi
    local -a last_shown
    zstat -A last_shown +mtime "$MOTD_CACHE_FILE"
    local now=$EPOCHSECONDS
    local today=${(%):-"%D{%Y-%m-%d}"}
    local cache_date
    strftime -s cache_date "%Y-%m-%d" "$last_shown"
    if [[ "$cache_date" != "$today" ]]; then
        return 0
    fi
    if (((now - last_shown) / 3600 >= MOTD_INTERVAL_HOURS)); then
        return 0
    fi
    return 1
}

# ==============================================================================
# LOGIN INFO (hot path)
# ==============================================================================

function _logininfo() {
    local now=$EPOCHSECONDS this_year
    strftime -s this_year "%Y" $now

    local last_terminal="" last_remote="" last_epoch=0
    if [[ -f "$LOGININFO_CACHE" ]]; then
        IFS='|' read -r last_terminal last_remote last_epoch <"$LOGININFO_CACHE"
    fi

    local current_terminal current_remote
    if [[ -n "$TTY" ]]; then
        current_terminal="${TTY#/dev/}"
        if [[ -n "$SSH_CLIENT" ]]; then
            current_remote="${SSH_CLIENT%% *}"
        fi
        if [[ -z "$current_remote" && -n "$SSH_CONNECTION" ]]; then
            current_remote="${SSH_CONNECTION%% *}"
        fi
    else
        # $TTY is unset: prefer tty(1) for this shell's stdin, fall back to who(1).
        local ttry
        ttry=$(command tty 2>/dev/null)
        if [[ -n "$ttry" && "$ttry" != *"not a tty"* ]]; then
            current_terminal="${ttry#/dev/}"
            if [[ -n "$SSH_CLIENT" ]]; then
                current_remote="${SSH_CLIENT%% *}"
            fi
            if [[ -z "$current_remote" && -n "$SSH_CONNECTION" ]]; then
                current_remote="${SSH_CONNECTION%% *}"
            fi
        else
            local current_line
            current_line=$(who am i 2>/dev/null || who -m 2>/dev/null)
            if [[ -z "$current_line" ]]; then
                return 0
            fi
            local -a words=(${=current_line})
            current_terminal=${words[2]}
            if [[ "$current_line" =~ '\(([^)]+)\)' ]]; then
                current_remote=${match[1]}
            fi
        fi
    fi

    local current_src="${current_remote:-$current_terminal}"
    local last_src="${last_remote:-$last_terminal}"

    print -r "$current_terminal|$current_remote|$now" >"$LOGININFO_CACHE" &|

    if ((last_epoch > 0)); then
        local diff=$((now - last_epoch)) REPLY
        if ((diff < 60)); then
            REPLY="just now"
        elif ((diff < 120)); then
            REPLY="1 minute ago"
        elif ((diff < 3600)); then
            REPLY="$((diff / 60)) minutes ago"
        elif ((diff < 172800)); then
            local time_str
            strftime -s time_str "%H:%M" $last_epoch
            if ((diff < 86400)); then
                REPLY="at $time_str"
            else
                REPLY="at $time_str yesterday"
            fi
        else
            local day weekday month time login_year
            strftime -s day "%d" $last_epoch
            strftime -s weekday "%A" $last_epoch
            strftime -s month "%b" $last_epoch
            strftime -s time "%H:%M" $last_epoch
            strftime -s login_year "%Y" $last_epoch
            day=${day#0}
            local suffix="th"
            case $day in
                1 | 21 | 31) suffix="st" ;;
                2 | 22) suffix="nd" ;;
                3 | 23) suffix="rd" ;;
            esac
            if [[ "$login_year" == "$this_year" ]]; then
                REPLY="on $weekday $month ${day}${suffix} at $time"
            else
                REPLY="on $weekday $month ${day}${suffix}, $login_year at $time"
            fi
        fi

        if [[ "$current_src" == "$last_src" ]]; then
            print -P "%B%F{cyan}Currently logged in from %f%b%B%F{green}${current_src}%f%b, %B%F{cyan}last logged in %f%b%B%F{yellow}${REPLY}%f%b"
        else
            print -P "%B%F{cyan}Currently logged in from %f%b%B%F{green}${current_src}%f%b, %B%F{cyan}last logged in from %f%b%B%F{green}${last_src}%f%b %B%F{yellow}${REPLY}%f%b"
        fi
    else
        print -P "%B%F{cyan}Currently logged in from %f%b%B%F{green}${current_src}%f%b"
    fi
}

# ==============================================================================
# IDE TERMINAL DETECTION
#
# VSCode and Cursor set VSCODE_PID in their integrated terminals. MOTD and
# logininfo are noisy in small IDE panes, and (critically) we must not touch
# the cache file so the first iTerm session that day still triggers MOTD.
# ==============================================================================

function _motd_is_ide_terminal() {
    if [[ -n "$VSCODE_PID" ]]; then
        return 0
    fi
    return 1
}

# ==============================================================================
# STARTUP EXECUTION
#
# Guard: no TTY → skip entirely (non-interactive SSH, cron, pipes)
# Guard: IDE terminal → skip entirely, do not update cache
#
# Trigger rules (first match wins):
#   1. Force file present    → always run, consume the flag
#   2. Interactive SSH (PTY) → always run, skip cache update
#   3. Local shell           → run only when cache is missing or stale
# ==============================================================================

function _motd_should_run() {
    if [[ ! -t 1 ]]; then
        return 1
    fi
    if _motd_is_ide_terminal; then
        return 1
    fi

    if [[ -f "$MOTD_FORCE_FILE" ]]; then
        rm -f "$MOTD_FORCE_FILE"
        return 0
    fi

    if [[ -n "$SSH_TTY" ]] && [[ ! -f "$MOTD_DISABLE_SSH" ]]; then
        return 0
    fi

    if [[ ! -f "$MOTD_CACHE_FILE" ]]; then
        mkdir -p "$HOME/.cache"
        return 0
    fi

    _motd_cache_expired
}

if _motd_should_run; then
    _motd_run_scripts
    if [[ -z "$SSH_TTY" ]]; then
        touch "$MOTD_CACHE_FILE"
    fi
fi

if ! _motd_is_ide_terminal; then
    _logininfo
fi
