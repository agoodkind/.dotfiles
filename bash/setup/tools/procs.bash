# Declaration file — sourced by tools.bash, not executed standalone.
TOOL_ID="procs"
TOOL_BIN="procs"
TOOL_REPO="dalance/procs"

tool_check_status() {
    tool_check_status_default "$(github_latest_release_version "$TOOL_REPO" || true)"
}

tool_upgrade_to_latest() {
    get_system_info
    local os_tag arch_tag
    case "$OS_NAME" in
        macos)  os_tag="mac"   ;;
        linux)  os_tag="linux" ;;
        *) return 1 ;;
    esac
    case "$ARCH" in
        x86_64)        arch_tag="x86_64"  ;;
        arm64|aarch64) arch_tag="aarch64" ;;
        *) return 1 ;;
    esac
    install_from_github "$TOOL_REPO" "$os_tag" "$arch_tag" ".zip" "$TOOL_BIN"
}
