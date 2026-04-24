# shellcheck shell=bash

# We control the init order, so the doctor diagnostic is always a false alarm.
_ZO_DOCTOR=0

# =============================================================================
#
# Utility functions for zoxide.
#

# pwd based on the value of _ZO_RESOLVE_SYMLINKS.
function __zoxide_pwd() {
    \builtin pwd -L
}

# cd + custom logic based on the value of _ZO_ECHO.
function __zoxide_cd() {
    # shellcheck disable=SC2164
    \builtin cd -- "$@"
}

# =============================================================================
#
# Hook configuration for zoxide.
#

# Hook to add new entries to the database.
function __zoxide_hook() {
    # shellcheck disable=SC2312
    \command zoxide add -- "$(__zoxide_pwd)"
}

# Initialize hook.
\builtin typeset -ga precmd_functions
\builtin typeset -ga chpwd_functions
# shellcheck disable=SC2034,SC2296
_zarr_filter precmd_functions __zoxide_hook
precmd_functions=("${_ZSH_ARR[@]}")
# shellcheck disable=SC2034,SC2296
_zarr_filter chpwd_functions __zoxide_hook
chpwd_functions=("${_ZSH_ARR[@]}")
chpwd_functions+=(__zoxide_hook)

# Report common issues.
function __zoxide_doctor() {
    if [[ ${_ZO_DOCTOR:-1} -eq 0 ]]; then
        return 0
    fi
    _zarr_find chpwd_functions __zoxide_hook
    if [[ ${_ZSH_INT:-0} -ne 0 ]]; then
        return 0
    fi

    _ZO_DOCTOR=0
    \builtin printf '%s\n' \
        'zoxide: detected a possible configuration issue.' \
        'Please ensure that zoxide is initialized right at the end of your shell configuration file (usually ~/.zshrc).' \
        '' \
        'If the issue persists, consider filing an issue at:' \
        'https://github.com/ajeetdsouza/zoxide/issues' \
        '' \
        'Disable this message by setting _ZO_DOCTOR=0.' \
        '' >&2
}

# =============================================================================
#
# When using zoxide with --no-cmd, alias these internal functions as desired.
#

# Jump to a directory using only keywords.
function __zoxide_z() {
    __zoxide_doctor
    if [[ "$#" -eq 0 ]]; then
        __zoxide_cd ~
    elif [[ "$#" -eq 1 ]] && { [[ -d "$1" || "$1" = '-' || "$1" =~ ^[-+][0-9]$ ]]; }; then
        __zoxide_cd "$1"
    elif [[ "$#" -eq 2 ]] && [[ "$1" = "--" ]]; then
        __zoxide_cd "$2"
    else
        \builtin local result
        # shellcheck disable=SC2312
        if result="$(\command zoxide query --exclude "$(__zoxide_pwd)" -- "$@")"; then
            __zoxide_cd "${result}"
        fi
    fi
}

# Jump to a directory using interactive search.
function __zoxide_zi() {
    __zoxide_doctor
    \builtin local result
    if result="$(\command zoxide query --interactive -- "$@")"; then
        __zoxide_cd "${result}"
    fi
}

# =============================================================================
#
# Commands for zoxide. Disable these using --no-cmd.
#

if ((${+functions[z]} == 0)); then
    function z() { __zoxide_z "$@"; }
fi

# zi conflicts with zinit, so use zxi instead
if ((${+functions[zxi]} == 0)); then
    function zxi() { __zoxide_zi "$@"; }
fi

if ((${+functions[cdi]} == 0)); then
    function cdi() { __zoxide_zi "$@"; }
fi

# cd override is handled by prefer_tty in prefer-decls.zsh so that non-TTY shells
# (Claude Code, scripts) get the plain builtin cd.

# Completions.
if [[ -o zle ]]; then
    __zoxide_result=''

    __zoxide_z_complete() {
        # Only show completions when the cursor is at the end of the line.
        # shellcheck disable=SC2154
        if [[ "${#words[@]}" -ne "${CURRENT}" ]]; then
            return 0
        fi

        if [[ "${#words[@]}" -eq 2 ]]; then
            # Show completions for local directories.
            _cd -/

        elif [[ "${words[-1]}" == '' ]]; then
            # Show completions for Space-Tab.
            # shellcheck disable=SC2086
            if ! __zoxide_result="$(\command zoxide query --exclude "$(__zoxide_pwd || \builtin true)" --interactive -- ${words[2,-1]})"; then
                __zoxide_result=''
            fi

            # Set a result to ensure completion doesn't re-run
            compadd -Q ""

            # Bind '\e[0n' to helper function.
            \builtin bindkey '\e[0n' '__zoxide_z_complete_helper'
            # Sends query device status code, which results in a '\e[0n' being sent to console input.
            \builtin printf '\e[5n'

            # Report that the completion was successful, so that we don't fall back
            # to another completion function.
            return 0
        fi
    }

    __zoxide_z_complete_helper() {
        if [[ -n "${__zoxide_result}" ]]; then
            # shellcheck disable=SC2034,SC2296
            _zqs "$__zoxide_result"
            BUFFER="z $_ZSH_Q"
            __zoxide_result=''
            \builtin zle reset-prompt
            \builtin zle accept-line
        else
            \builtin zle reset-prompt
        fi
    }
    \builtin zle -N __zoxide_z_complete_helper

    if [[ "${+functions[compdef]}" -ne 0 ]]; then
        \compdef __zoxide_z_complete z cd
    fi
fi

# =============================================================================
#
# To initialize zoxide, add this to your shell configuration file (usually ~/.zshrc):
#
# eval "$(zoxide init zsh)"
