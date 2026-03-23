# Declaration file — sourced by tools.bash, not executed standalone.
TOOL_ID="atuin"
TOOL_BIN="atuin"
TOOL_REPO="atuinsh/atuin"

tool_check_status() {
    tool_check_status_default "$(github_latest_release_version "$TOOL_REPO" || true)"
}

tool_upgrade_to_latest() {
    cargo install atuin --locked
}
