# Declaration file — sourced by tools.bash, not executed standalone.
TOOL_ID="starship"
TOOL_BIN="starship"
TOOL_REPO="starship/starship"

tool_check_status() {
    tool_check_status_default "$(github_latest_release_version "$TOOL_REPO" || true)"
}

tool_upgrade_to_latest() {
    curl -sS https://starship.rs/install.sh | sh -s -- --yes
}
