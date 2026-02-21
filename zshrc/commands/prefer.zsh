# shellcheck shell=bash

# Helper to resolve binary target and args
# Sets _PREFER_RESOLVED variable to the full command string
_resolve_prefer_target() {
    local binary="$1"
    shift
    local args=("$@")

    local qargs=""
    local arg
    for arg in "${args[@]}"; do
        [[ -z "$arg" ]] && continue
        qargs+=" ${(q)arg}"
    done

    if [[ "$binary" == /* ]] && [[ -x "$binary" ]]; then
        _PREFER_RESOLVED="command $binary$qargs"
        return 0
    fi

    if [[ -z "${aliases[$binary]}" ]] && ! isinstalled "$binary"; then
        return 1
    fi

    if [[ -n "${aliases[$binary]}" ]]; then
        _PREFER_RESOLVED="${aliases[$binary]}$qargs"
    else
        _PREFER_RESOLVED="command $binary$qargs"
    fi
    return 0
}

# -----------------------------------------------------------------------------
# Alias Caching & Performance
# -----------------------------------------------------------------------------

PREFER_CACHE_FILE="$HOME/.cache/zsh_prefer_aliases.zsh"
_CURRENT_HASH=""

_check_prefer_cache_async() {
    local current_hash
    if (( $+functions[dotfiles_changed_hash] )); then
        current_hash=$(dotfiles_changed_hash)
    else
        return
    fi

    local cached_hash
    if [[ -f "$PREFER_CACHE_FILE" ]]; then
        read -r cached_hash < <(head -n 1 "$PREFER_CACHE_FILE" | sed 's/# HASH: //')
    fi

    if [[ "$current_hash" != "$cached_hash" ]]; then
        rm -f "$PREFER_CACHE_FILE"
    fi
}

_init_cache_if_needed() {
    if [[ -z "$_CURRENT_HASH" ]]; then
        if (( $+functions[dotfiles_changed_hash] )); then
            _CURRENT_HASH=$(dotfiles_changed_hash)
        fi

        mkdir -p "$(dirname "$PREFER_CACHE_FILE")"

        if [[ ! -f "$PREFER_CACHE_FILE" ]] || [[ ! -s "$PREFER_CACHE_FILE" ]]; then
            echo "# HASH: $_CURRENT_HASH" > "$PREFER_CACHE_FILE"
        fi
    fi
}

if [[ -f "$PREFER_CACHE_FILE" ]]; then
    source "$PREFER_CACHE_FILE"
    async_run _check_prefer_cache_async
else
    _init_cache_if_needed
fi

# -----------------------------------------------------------------------------
# Implementation Functions (Slow Path)
# -----------------------------------------------------------------------------

_prefer_impl() {
    local name="$1"
    local binary="$2"
    shift 2 || true

    _resolve_prefer_target "$binary" "$@" || return

    alias "$name=$_PREFER_RESOLVED"

    if [[ ! -f "$PREFER_CACHE_FILE" ]]; then
         _init_cache_if_needed
    fi
    echo "alias ${(q)name}=${(q)_PREFER_RESOLVED}" >> "$PREFER_CACHE_FILE"
}

_prefer_tty_impl() {
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

    if [[ ! -f "$PREFER_CACHE_FILE" ]]; then
         _init_cache_if_needed
    fi
    echo "$func_body" >> "$PREFER_CACHE_FILE"
}

# -----------------------------------------------------------------------------
# Public API (Fast Check + Fallback)
# -----------------------------------------------------------------------------

prefer() {
    local name="$1"
    [[ -n "${aliases[$name]}" ]] && return 0
    _prefer_impl "$@"
}

prefer_tty() {
    local name="$1"
    (( $+functions[$name] )) && return 0
    _prefer_tty_impl "$@"
}
