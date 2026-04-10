# Declaration file — sourced by tools.bash, not executed standalone.
TOOL_ID="cloudflare-speed-cli"
TOOL_BIN="cloudflare-speed-cli"
TOOL_REPO="kavehtehrani/cloudflare-speed-cli"

tool_check_status() {
    tool_check_status_default "$(github_latest_release_version "$TOOL_REPO" || true)"
}

tool_upgrade_to_latest() {
    get_system_info
    local arch_tag os_tag
    case "$ARCH" in
        x86_64) arch_tag="x86_64" ;;
        arm64 | aarch64) arch_tag="aarch64" ;;
        *) return 1 ;;
    esac
    case "$OS_NAME" in
        macos) os_tag="apple-darwin" ;;
        linux) os_tag="unknown-linux-musl" ;;
        *) return 1 ;;
    esac
    install_from_github "$TOOL_REPO" "$os_tag" "$arch_tag" ".tar.xz" "$TOOL_BIN"
}
