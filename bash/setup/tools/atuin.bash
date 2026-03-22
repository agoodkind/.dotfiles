# Declaration file — sourced by tools.bash, not executed standalone.
TOOL_ID="atuin"
TOOL_BIN="atuin"
TOOL_REPO="atuinsh/atuin"

tool_check_status() {
    tool_check_status_default "$(github_latest_release_version "$TOOL_REPO" || true)"
}

tool_upgrade_to_latest() {
    get_system_info
    # atuin has no prebuilt x86_64-apple-darwin binary; fall back to cargo.
    if [[ "$OS_NAME" == "macos" ]] && [[ "$ARCH" == "x86_64" ]]; then
        cargo install atuin --locked
        return
    fi
    curl --proto '=https' --tlsv1.2 -sSf https://setup.atuin.sh | sh -s -- --yes
}
