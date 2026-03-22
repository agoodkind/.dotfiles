# Declaration file — sourced by tools.bash, not executed standalone.
TOOL_ID="zoxide"
TOOL_BIN="zoxide"
TOOL_REPO="ajeetdsouza/zoxide"

tool_check_status() {
    tool_check_status_default "$(github_latest_release_version "$TOOL_REPO" || true)"
}

tool_upgrade_to_latest() {
    curl -sSfL https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | sh
}
