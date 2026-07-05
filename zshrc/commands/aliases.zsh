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
    local uuid
    if (($+commands[uuidgen])); then
        uuid=$(command uuidgen) || return $?
    elif [[ -r /proc/sys/kernel/random/uuid ]]; then
        uuid=$(</proc/sys/kernel/random/uuid) || return $?
    else
        echo "_uuid: uuidgen not found; install uuid-runtime (Debian/Ubuntu) or util-linux" >&2
        return 127
    fi
    print -r -- ${uuid:l}
}

# thefuck wrapper: lazy load on first use
function fuck() {
    unfunction fuck
    eval "$(thefuck --alias)"
    fuck "$(fc -ln -1)"
}
