# Defaults handling module for install/update scripts
# Provides --use-defaults/-d flag parsing and read_with_default helper

# Parse command line flags (if not already set via environment)
# When sourced, $@ refers to the parent script's arguments
if [[ -z "${USE_DEFAULTS:-}" ]]; then
    USE_DEFAULTS=false
    # Check parent script's arguments for --use-defaults or -d flag
    for arg in "$@"; do
        case $arg in
            --use-defaults|-d)
                USE_DEFAULTS=true
                export USE_DEFAULTS
                break
                ;;
        esac
    done
else
    # Use exported value if already set (from parent script or environment)
    USE_DEFAULTS="${USE_DEFAULTS}"
fi

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

