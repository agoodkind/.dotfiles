# shellcheck shell=bash

# pbcopy wrapper: on macOS use native, otherwise ssh to source host
function pbcopy() {
    if is_macos; then
        if [[ $# -gt 0 ]]; then
            echo -n "$*" | /usr/bin/pbcopy
        else
            /usr/bin/pbcopy
        fi
    else
        if [[ -z "$SSH_SOURCE_HOST" ]]; then
            echo "pbcopy: SSH_SOURCE_HOST not set" >&2
            return 1
        fi
        if [[ $# -gt 0 ]]; then
            echo -n "$*" | ssh "$SSH_SOURCE_HOST" /usr/bin/pbcopy 2>/dev/null
        else
            ssh "$SSH_SOURCE_HOST" /usr/bin/pbcopy 2>/dev/null
        fi
    fi
}

function _uuid() {
    if command -v uuidgen >/dev/null 2>&1; then
        uuidgen | tr '[:upper:]' '[:lower:]'
        return 0
    fi
    if [[ -r /proc/sys/kernel/random/uuid ]]; then
        tr -d '\n' </proc/sys/kernel/random/uuid
        return 0
    fi
    echo "_uuid: uuidgen not found; install uuid-runtime (Debian/Ubuntu) or util-linux" >&2
    return 127
}

# thefuck wrapper: lazy load on first use
function fuck() {
    unfunction fuck
    eval "$(thefuck --alias)"
    fuck "$(fc -ln -1)"
}
