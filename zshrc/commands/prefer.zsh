# shellcheck shell=bash

# Helper to resolve binary target and args
# Sets _PREFER_RESOLVED variable to the full command string
function _resolve_prefer_target() {
    local binary="$1"
    shift
    local args=("$@")

    local qargs=""
    local arg
    for arg in "${args[@]}"; do
        if [[ -z "$arg" ]]; then
            continue
        fi
        qargs+=" ${(q)arg}"
    done

    if [[ "$binary" == /* && -x "$binary" ]]; then
        _PREFER_RESOLVED="command $binary$qargs"
        return 0
    fi

    if (( $+functions[$binary] != 0 )); then
        _PREFER_RESOLVED="$binary$qargs"
        return 0
    fi

    if [[ -n "${aliases[$binary]}" ]]; then
        _PREFER_RESOLVED="${aliases[$binary]}$qargs"
        return 0
    fi

    isinstalled "$binary" || return 1
    _PREFER_RESOLVED="command $binary$qargs"
    return 0
}

# -----------------------------------------------------------------------------
# Alias Caching & Performance
# -----------------------------------------------------------------------------

PREFER_CACHE_FILE="$HOME/.cache/zsh_prefer_aliases.zsh"
_PREFER_CACHE_VALID=false

function _prefer_check_cache() {
    if [[ -f "$PREFER_CACHE_FILE" && -s "$PREFER_CACHE_FILE" ]]; then
        return 0
    fi
    return 1
}

function _prefer_init_cache() {
    mkdir -p "$(dirname "$PREFER_CACHE_FILE")"
    : > "$PREFER_CACHE_FILE"
}

if _prefer_check_cache; then
    source "$PREFER_CACHE_FILE"
    _PREFER_CACHE_VALID=true
else
    _prefer_init_cache
fi

# -----------------------------------------------------------------------------
# Implementation Functions (Slow Path)
# -----------------------------------------------------------------------------

function _prefer_impl() {
    local name="$1"
    local binary="$2"
    shift 2 || true

    _resolve_prefer_target "$binary" "$@" || return

    alias "$name=$_PREFER_RESOLVED"
    echo "alias ${(q)name}=${(q)_PREFER_RESOLVED}" >> "$PREFER_CACHE_FILE"
}

function _prefer_tty_impl() {
    local name="$1"
    local binary="$2"
    shift 2 || true

    _resolve_prefer_target "$binary" "$@" || return

    local func_body="
$name() {
    if [[ -t 1 ]]; then
        $_PREFER_RESOLVED \"\$@\";
    else
        command $name \"\$@\";
    fi;
}"

    eval "$func_body"
    echo "$func_body" >> "$PREFER_CACHE_FILE"
}

# -----------------------------------------------------------------------------
# Public API (Fast Check + Fallback)
# -----------------------------------------------------------------------------

function prefer() {
    local name="$1"
    if [[ -n "${aliases[$name]}" ]]; then
        return 0
    fi
    _prefer_impl "$@"
}

function prefer_tty() {
    local name="$1"
    if (( $+functions[$name] != 0 )); then
        return 0
    fi
    _prefer_tty_impl "$@"
}
