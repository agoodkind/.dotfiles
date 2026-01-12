# shellcheck shell=bash
###############################################################################
# Command Wrappers & Customizations ##########################################
###############################################################################

# man wrapper: use default pager when not in a TTY (e.g., Cursor commands)
man() {
    if [[ -t 1 ]]; then
        # TTY: use custom MANPAGER
        command man "$@"
    else
        # Non-TTY: use default pager
        MANPAGER= PAGER=less command man "$@"
    fi
}

_needs_sudoedit_for_any_path() {
    emulate -L zsh
    setopt localoptions no_unset

    local p parent
    for p in "$@"; do
        [[ -n "$p" ]] || continue

        # If the file exists and isn't writable, it will fail without sudo.
        if [[ -e "$p" ]]; then
            [[ -w "$p" ]] || return 0
            continue
        fi

        # If creating a new file, parent dir must be writable.
        parent="${p:h}"
        [[ -n "$parent" ]] || parent="."
        if [[ -d "$parent" ]] && [[ ! -w "$parent" ]]; then
            return 0
        fi
    done

    return 1
}

_edit_maybe_sudoedit() {
    emulate -L zsh
    setopt localoptions noshwordsplit

    local editor_bin="$1"
    shift || true

    if (( $# == 0 )); then
        command "$editor_bin"
        return $?
    fi

    # Keep this conservative: if any editor flags are present, don't rewrite.
    # Users can still do `sudo vim ...` (sudo wrapper rewrites that to sudoedit).
    local a
    for a in "$@"; do
        if [[ "$a" == -* || "$a" == +* ]]; then
            command "$editor_bin" "$@"
            return $?
        fi
    done

    if _needs_sudoedit_for_any_path "$@"; then
        sudo -e -- "$@"
        return $?
    fi

    command "$editor_bin" "$@"
}

# When you type `sudo vim/nvim/vi/nano <file>`, use sudoedit instead.
# This avoids running an editor as root, and ensures proper temp-file flow.
sudo() {
    emulate -L zsh
    setopt localoptions noshwordsplit

    # Non-TTY: use normal sudo (no editor rewriting)
    if [[ ! -t 1 ]]; then
        command sudo "$@"
        return $?
    fi

    local -a sudo_opts rest editor_args
    local cmd

    sudo_opts=()
    rest=("$@")

    # Parse a small, safe subset of sudo flags so we can preserve them
    # when rewriting to `sudo -e`. If we see an unknown flag, fall back
    # to real sudo.
    while (( ${#rest[@]} > 0 )); do
        case "${rest[1]}" in
            --)
                rest=("${rest[@]:1}")
                break
                ;;
            -u|-g|-h|-p|-C|-T|-t|-U)
                if (( ${#rest[@]} < 2 )); then
                    command sudo "$@"
                    return $?
                fi
                sudo_opts+=("${rest[1]}" "${rest[2]}")
                rest=("${rest[@]:2}")
                ;;
            -[AbEHnSsvVikKlL])
                sudo_opts+=("${rest[1]}")
                rest=("${rest[@]:1}")
                ;;
            -?*)
                command sudo "$@"
                return $?
                ;;
            *)
                break
                ;;
        esac
    done

    cmd="${rest[1]:-}"
    editor_args=("${rest[@]:1}")

    # Only rewrite known editor invocations (with file args). Keep it
    # conservative to avoid swallowing editor flags like `+123` or
    # `-u NORC`.
    if [[ -n "$cmd" ]] && (( ${#editor_args[@]} > 0 )); then
        case "$cmd" in
            nano|vim|vi|nvim)
                local a
                for a in "${editor_args[@]}"; do
                    if [[ "$a" == -* || "$a" == +* ]]; then
                        command sudo "$@"
                        return $?
                    fi
                done

                command sudo "${sudo_opts[@]}" -e -- "${editor_args[@]}"
                return $?
                ;;
        esac
    fi

    command sudo "$@"
}

# Helper to resolve binary target and args
# Sets _PREFER_RESOLVED variable to the full command string
_resolve_prefer_target() {
    local binary="$1"
    shift
    local args=("$@")

    # If binary is NOT an alias AND NOT installed, skip it
    if [[ -z "${aliases[$binary]}" ]] && ! isinstalled "$binary"; then
        return 1
    fi

    local qargs=""
    local arg
    for arg in "${args[@]}"; do
        [[ -z "$arg" ]] && continue
        qargs+=" $(printf '%q' "$arg")"
    done

    # Resolve target
    if [[ -n "${aliases[$binary]}" ]]; then
        # Binary is an alias (e.g., ll='eza -lah')
        _PREFER_RESOLVED="${aliases[$binary]}$qargs"
    else
        # Binary is a real command
        _PREFER_RESOLVED="command $binary$qargs"
    fi
    return 0
}

# Prefer running an alternate binary for a command when available
prefer() {
    local name="$1"
    local binary="$2"
    shift 2 || true
    
    _resolve_prefer_target "$binary" "$@" || return

    # Use alias instead of function for faster startup (avoids eval overhead)
    alias "$name=$_PREFER_RESOLVED"
}

# Prefer an alternate binary only when writing to a terminal
# (fallback otherwise)
prefer_tty() {
    local name="$1"
    local binary="$2"
    shift 2 || true
    
    _resolve_prefer_target "$binary" "$@" || return

    eval "$name() {
        if [[ -t 1 ]]; then
            $_PREFER_RESOLVED \"\$@\";
        else
            command $name \"\$@\";
        fi;
    }"
}

# pbcopy wrapper: on macOS use native, otherwise ssh to source host
pbcopy() {
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

# thefuck wrapper: lazy load on first use
fuck() {
    # Undefine this function and source the real one on first use
    unfunction fuck
    eval "$(thefuck --alias)"
    fuck "$(fc -ln -1)"
}