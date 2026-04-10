# shellcheck shell=bash

function man() {
    local -a resolved_args
    local arg val
    for arg in "$@"; do
        if [[ "$arg" == -* ]] || [[ "$arg" == [0-9] ]]; then
            resolved_args+=("$arg")
            continue
        fi
        val=""
        if [[ -n "${aliases[$arg]}" ]]; then
            val="${aliases[$arg]}"
        elif (($+functions[$arg])); then
            local body="${functions[$arg]}"
            local inner="${body#*command }"
            inner="${inner%%[[:space:]]*}"
            if [[ -n "${aliases[$inner]}" ]]; then
                val="${aliases[$inner]}"
            elif [[ -n "$inner" ]]; then
                val="$inner"
            fi
        fi
        if [[ -n "$val" ]]; then
            val="${val#command }"
            val="${val%%[[:space:]]*}"
            resolved_args+=("$val")
        else
            resolved_args+=("$arg")
        fi
    done
    if [[ -t 1 ]]; then
        command man "${resolved_args[@]}"
    else
        MANPAGER= PAGER=less command man "${resolved_args[@]}"
    fi
}

function _needs_sudoedit_for_any_path() {
    emulate -L zsh
    setopt localoptions no_unset

    local p parent
    for p in "$@"; do
        if [[ -z "$p" ]]; then
            continue
        fi

        if [[ -e "$p" ]]; then
            if [[ ! -w "$p" ]]; then
                return 0
            fi
            continue
        fi

        parent="${p:h}"
        if [[ -z "$parent" ]]; then
            parent="."
        fi
        if [[ -d "$parent" ]] && [[ ! -w "$parent" ]]; then
            return 0
        fi
    done

    return 1
}

function _edit_maybe_sudoedit() {
    emulate -L zsh
    setopt localoptions noshwordsplit

    local editor_bin="$1"
    shift || true

    if (($# == 0)); then
        command "$editor_bin"
        return $?
    fi

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
function sudo() {
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

    while ((${#rest[@]} > 0)); do
        case "${rest[1]}" in
            --)
                rest=("${rest[@]:1}")
                break
                ;;
            -u | -g | -h | -p | -C | -T | -t | -U)
                if ((${#rest[@]} < 2)); then
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

    if [[ -n "$cmd" ]] && ((${#editor_args[@]} > 0)); then
        case "$cmd" in
            nano | vim | vi | nvim)
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
