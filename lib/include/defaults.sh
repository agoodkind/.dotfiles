# Defaults handling module for install/update scripts
# Provides --use-defaults/-d flag parsing and read_with_default helper

# Parse command line flags (if not already set via environment)
# When sourced, $@ refers to the parent script's arguments
# Always check command-line arguments first, as they take precedence
found_use_defaults=false
for arg in "$@"; do
    case $arg in
        --use-defaults|-d)
            found_use_defaults=true
            break
            ;;
    esac
done

if [[ "$found_use_defaults" == "true" ]]; then
    USE_DEFAULTS=true
    export USE_DEFAULTS
elif [[ -z "${USE_DEFAULTS:-}" ]]; then
    USE_DEFAULTS=false
    export USE_DEFAULTS
fi
# If USE_DEFAULTS was already set and we didn't find --use-defaults in args, keep existing value
unset found_use_defaults

# Helper function to read with defaults
read_with_default() {
    local prompt="$1"
    local default="$2"
    if [[ "$USE_DEFAULTS" == "true" ]]; then
        REPLY="$default"
        echo "$default"
    else
        read -p "$prompt" -n 1 -r
        echo
    fi
}

# Helper function to read with defaults (for multi-line input)
read_with_default_multiline() {
    local prompt="$1"
    local default="$2"
    if [[ "$USE_DEFAULTS" == "true" ]]; then
        REPLY="$default"
        echo "$default"
    else
        read -r -p "$prompt" REPLY
    fi
}

# Helper function to run a command with --use-defaults flag if USE_DEFAULTS is true
# Usage: run_with_defaults "command" [args...]
run_with_defaults() {
    if [[ "$USE_DEFAULTS" == "true" ]]; then
        "$@" --use-defaults
    else
        "$@"
    fi
}

# print [DEBUG] message if DEBUG is true
# Usage: debug_echo "message"
debug_echo() {
    if [[ -f "$HOME/.cache/dotfiles_debug_enabled" ]] || [[ "${DEBUG:-false}" == "true" ]]; then
        printf "%b[DEBUG]%b %s%b\n" "${GRAY}" "${NC}" "$*" "${NC}"
    fi
}

enable_debug() {
    touch "$HOME/.cache/dotfiles_debug_enabled"
}

disable_debug() {
    rm -f "$HOME/.cache/dotfiles_debug_enabled"
}