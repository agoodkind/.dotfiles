# Declaration file — sourced by tools.bash, not executed standalone.
TOOL_ID="fzf"
TOOL_BIN="fzf"
TOOL_REPO="junegunn/fzf"

tool_check_status() {
    tool_check_status_default "$(github_latest_release_version "$TOOL_REPO" || true)"
}

tool_upgrade_to_latest() {
    get_system_info
    local os_tag arch_tag
    case "$OS_NAME" in
        macos) os_tag="darwin" ;;
        linux) os_tag="linux"  ;;
        *) return 1 ;;
    esac
    case "$ARCH" in
        x86_64)        arch_tag="amd64" ;;
        arm64|aarch64) arch_tag="arm64" ;;
        *) return 1 ;;
    esac
    install_from_github "$TOOL_REPO" "$os_tag" "$arch_tag" ".tar.gz" "$TOOL_BIN"
}
