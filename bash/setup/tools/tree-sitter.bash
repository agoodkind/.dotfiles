# Declaration file — sourced by tools.bash, not executed standalone.
TOOL_ID="tree-sitter"
TOOL_BIN="tree-sitter"
TOOL_REPO="tree-sitter/tree-sitter"

tool_check_status() {
    tool_check_status_default "$(github_latest_release_version "$TOOL_REPO" || true)"
}

tool_upgrade_to_latest() {
    get_system_info
    local arch_tag os_tag
    case "$ARCH" in
        x86_64)        arch_tag="x64"   ;;
        arm64|aarch64) arch_tag="arm64" ;;
        *) return 1 ;;
    esac
    case "$OS_NAME" in
        macos)  os_tag="macos" ;;
        linux)  os_tag="linux" ;;
        *) return 1 ;;
    esac
    install_from_github "$TOOL_REPO" "$os_tag" "$arch_tag" ".gz" "$TOOL_BIN"
}
