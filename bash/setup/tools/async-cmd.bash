# Declaration file — sourced by tools.bash, not executed standalone.
TOOL_ID="async-cmd"
TOOL_BIN="async-cmd"

tool_check_status() {
    tool_check_status_default "$(crates_latest_version "async-cmd" || true)"
}

tool_upgrade_to_latest() {
    if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
        return 0
    fi
    cargo install async-cmd --locked --force
}
