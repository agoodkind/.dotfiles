#!/usr/bin/env bash
set -e
set -o pipefail

export DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

source "${DOTDOTFILES}/bash/core/colors.bash"
source "${DOTDOTFILES}/bash/core/defaults.bash"
source "${DOTDOTFILES}/bash/core/tools.bash"

if ! command -v jq >/dev/null 2>&1; then
    color_echo RED "jq is required but not installed; skipping custom tools"
    exit 1
fi

FAILED_TOOLS=()

normalize_version() {
    local raw="$1"
    raw="${raw#v}"
    raw="${raw%% *}"
    raw="$(printf '%s' "$raw" | sed -E 's/[^0-9.].*$//')"
    printf '%s' "$raw"
}

semver_lt() {
    local a b
    a="$(normalize_version "$1")"
    b="$(normalize_version "$2")"

    if [[ -z "$a" || -z "$b" ]]; then
        return 1
    fi

    local a1 a2 a3 b1 b2 b3
    IFS=. read -r a1 a2 a3 <<< "$a"
    IFS=. read -r b1 b2 b3 <<< "$b"
    a1="${a1:-0}"; a2="${a2:-0}"; a3="${a3:-0}"
    b1="${b1:-0}"; b2="${b2:-0}"; b3="${b3:-0}"

    if (( a1 < b1 )); then return 0; fi
    if (( a1 > b1 )); then return 1; fi
    if (( a2 < b2 )); then return 0; fi
    if (( a2 > b2 )); then return 1; fi
    if (( a3 < b3 )); then return 0; fi
    return 1
}

github_latest_release_version() {
    local repo="$1"
    curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" 2>/dev/null \
        | jq -r '.tag_name | ltrimstr("v")'
}

crates_latest_version() {
    local crate="$1"
    curl -fsSL "https://crates.io/api/v1/crates/${crate}" 2>/dev/null \
        | jq -r '.crate.max_version'
}

# Shared helper — call from each tool declaration file's tool_check_status.
# Populates all TOOL_* globals. Extracts the current version by finding the
# first semver-shaped token (X.Y or X.Y.Z) in `$TOOL_BIN --version` output,
# so tool scripts never need to know the output format.
tool_check_status_default() {
    local target_version="$1"

    TOOL_INSTALLED=false
    TOOL_CURRENT_VERSION=""
    TOOL_TARGET_VERSION="$target_version"
    TOOL_UPGRADE_AVAILABLE=false

    if command -v "$TOOL_BIN" >/dev/null 2>&1; then
        TOOL_INSTALLED=true
        TOOL_CURRENT_VERSION="$(
            "$TOOL_BIN" --version 2>/dev/null \
                | grep -oE '[0-9]+\.[0-9]+(\.[0-9]+)?' | head -1
        )"
    fi
    if [[ "$TOOL_INSTALLED" == false ]]; then
        TOOL_UPGRADE_AVAILABLE=true
        return
    fi
    if semver_lt "$TOOL_CURRENT_VERSION" "$TOOL_TARGET_VERSION"; then
        TOOL_UPGRADE_AVAILABLE=true
    fi
}

run_tool() {
    local script="$1"

    # Reset shared globals before sourcing so stale values from the previous
    # tool never leak into a tool that omits a variable (e.g. TOOL_REPO).
    TOOL_ID="" TOOL_BIN="" TOOL_REPO=""

    # Source the declaration file; sets TOOL_ID, TOOL_BIN, TOOL_REPO,
    # tool_check_status, and tool_upgrade_to_latest into the current shell.
    source "$script"

    # tool_check_status sets: TOOL_INSTALLED, TOOL_CURRENT_VERSION,
    # TOOL_TARGET_VERSION, TOOL_UPGRADE_AVAILABLE
    tool_check_status

    if [[ "$TOOL_INSTALLED" == true ]]; then
        if [[ "$TOOL_UPGRADE_AVAILABLE" == true ]]; then
            color_echo YELLOW "  ⬆️  ${TOOL_ID} (${TOOL_CURRENT_VERSION} -> ${TOOL_TARGET_VERSION})"
        else
            color_echo GREEN "  ✅  ${TOOL_ID} is up to date (${TOOL_CURRENT_VERSION})"
            return 0
        fi
    else
        color_echo YELLOW "  📦  Installing ${TOOL_ID}..."
    fi

    if tool_upgrade_to_latest; then
        tool_check_status
        if [[ -n "$TOOL_TARGET_VERSION" ]] && semver_lt "$TOOL_CURRENT_VERSION" "$TOOL_TARGET_VERSION"; then
            color_echo YELLOW "  ⚠️  ${TOOL_ID} installed but PATH resolves ${TOOL_CURRENT_VERSION} (expected ${TOOL_TARGET_VERSION}); an older copy may shadow it"
            dotfiles_notify warn "${TOOL_ID}: installed ${TOOL_TARGET_VERSION} but PATH resolves ${TOOL_CURRENT_VERSION}"
        else
            color_echo GREEN "  ✅  ${TOOL_ID} now at ${TOOL_CURRENT_VERSION}"
        fi
    else
        color_echo RED "  ❌  Failed: ${TOOL_ID}"
        dotfiles_notify warn "tool install/upgrade failed: ${TOOL_ID}"
        FAILED_TOOLS+=("${TOOL_ID}")
    fi
}

main() {
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    color_echo BLUE "🔧  Installing Custom Tools..."
    color_echo BLUE "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    for tool_script in "${DOTDOTFILES}/bash/setup/tools"/*.bash; do
        [[ -f "$tool_script" ]] || continue
        run_tool "$tool_script"
    done

    if [[ ${#FAILED_TOOLS[@]} -gt 0 ]]; then
        color_echo YELLOW "  ⚠️  Non-critical tool failures: ${FAILED_TOOLS[*]}"
        dotfiles_notify warn "custom tools completed with failures: ${FAILED_TOOLS[*]}"
    fi

    color_echo GREEN "Custom tools installation complete!"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
