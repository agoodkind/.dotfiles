# Declaration file — sourced by tools.bash, not executed standalone.
TOOL_ID="fastfetch"
TOOL_BIN="fastfetch"
TOOL_REPO="fastfetch-cli/fastfetch"

tool_check_status() {
    tool_check_status_default "$(github_latest_release_version "$TOOL_REPO" || true)"
}

tool_upgrade_to_latest() {
    linux_only "fastfetch (deb) is Linux-only, skipping..."
    install_from_github "$TOOL_REPO" "linux" "amd64" ".deb" "$TOOL_BIN"
}
