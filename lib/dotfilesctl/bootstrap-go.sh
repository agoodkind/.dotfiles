#!/usr/bin/env bash
set -e
set -u
set -o pipefail

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
GO_BINARY="${GO_BINARY:-go}"
GO_LOCAL_ROOT="${GO_LOCAL_ROOT:-$HOME/.local/go}"
GO_LOCAL_BIN="${GO_LOCAL_ROOT}/bin"
GO_BOOTSTRAP_VERSION="${GO_BOOTSTRAP_VERSION:-}"
DOTFILESCTL_BINARY_DIR="${DOTFILESCTL_BINARY_DIR:-$HOME/.cache/dotfilesctl/bin}"
DOTFILESCTL_BINARY="${DOTFILESCTL_BINARY:-$DOTFILESCTL_BINARY_DIR/dotfilesctl}"
DOTFILESCTL_BUILD_LOCK_FILE="${DOTFILESCTL_BUILD_LOCK_FILE:-$DOTFILESCTL_BINARY_DIR/.dotfilesctl.build.lock}"
DEFAULT_GO_BOOTSTRAP_VERSION="go1.22.7"
GO_DARWIN11_BOOTSTRAP_VERSION="${GO_DARWIN11_BOOTSTRAP_VERSION:-go1.24.13}"

require_tools() {
    local missing=0

    while [ $# -gt 0 ]; do
        if ! check_command "$1"; then
            missing=1
        fi
        shift
    done

    if [ "$missing" -eq 1 ]; then
        return 1
    fi

    return 0
}

check_command() {
    local cmd="$1"

    if command -v "$cmd" >/dev/null 2>&1; then
        return 0
    fi

    return 1
}

dotfilesctl_binary_stale() {
	if [ ! -x "$DOTFILESCTL_BINARY" ]; then
		return 0
	fi

	if ! check_command rg; then
		return 0
	fi

	local source_file
	while IFS= read -r source_file; do
		if [ "$source_file" -nt "$DOTFILESCTL_BINARY" ]; then
			return 0
		fi
	done < <(
		rg --files \
			-g '*.go' \
			-g '*.toml' \
			-g '*.mod' \
			-g '*.sum' \
			"$DOTDOTFILES/lib/dotfilesctl"
	)

	return 1
}

emit_warning_if_legacy_bash() {
    if [ -n "${BASH_VERSINFO-}" ]; then
        if [ "${BASH_VERSINFO[0]}" -lt 3 ] || { [ "${BASH_VERSINFO[0]}" -eq 3 ] && [ "${BASH_VERSINFO[1]}" -lt 2; }; then
            echo "warning: unsupported bash ${BASH_VERSINFO[0]}.${BASH_VERSINFO[1]}; continuing without unsupported features" >&2
            return
        fi
    fi
}

download_file() {
    local url="$1"
    local destination="$2"

    if check_command curl; then
        curl --location --silent --show-error --fail "$url" --output "$destination"
        return 0
    fi

    if check_command wget; then
        wget --quiet --output-document "$destination" "$url"
        return 0
    fi

    echo "missing curl or wget for bootstrap download" >&2
    return 1
}

fetch_go_version() {
    local tmpdir
    local versions_json
    local first_version
    local os
    local os_version
    local os_major

    if [ -n "$GO_BOOTSTRAP_VERSION" ]; then
        echo "$GO_BOOTSTRAP_VERSION"
        return
    fi

    if ! require_tools mktemp sed; then
        echo "$DEFAULT_GO_BOOTSTRAP_VERSION"
        return
    fi

    tmpdir="$(mktemp -d)"
    versions_json="$tmpdir/go-versions.json"

    if download_file "https://go.dev/dl/?mode=json" "$versions_json"; then
        first_version="$(sed -n 's/.*"version":"\(go[0-9][^\"]*\)".*/\1/p' "$versions_json" | head -n 1)"
        rm -rf "$tmpdir"
        if [ -n "$first_version" ]; then
            os="$(uname -s)"
            if [ "$os" = "Darwin" ]; then
                os_version="$(sw_vers -productVersion 2>/dev/null)"
                os_major="${os_version%%.*}"
                if [ "$os_major" = "11" ]; then
                    echo "$GO_DARWIN11_BOOTSTRAP_VERSION"
                    return
                fi
            fi
            echo "$first_version"
            return
        fi
    else
        rm -rf "$tmpdir"
    fi

    os="$(uname -s)"
    if [ "$os" = "Darwin" ]; then
        os_version="$(sw_vers -productVersion 2>/dev/null)"
        os_major="${os_version%%.*}"
        if [ "$os_major" = "11" ]; then
            echo "$GO_DARWIN11_BOOTSTRAP_VERSION"
            return
        fi
    fi

    echo "$DEFAULT_GO_BOOTSTRAP_VERSION"
}

go_system_arch() {
    local os
    local arch

    os="$(uname -s)"
    arch="$(uname -m)"

    case "$os" in
    Darwin)
        case "$arch" in
        x86_64)
            echo "darwin-amd64"
            ;;
        arm64)
            echo "darwin-arm64"
            ;;
        *)
            echo "unsupported"
            ;;
        esac
        ;;
    Linux)
        case "$arch" in
        x86_64)
            echo "linux-amd64"
            ;;
        arm64|aarch64)
            echo "linux-arm64"
            ;;
        *)
            echo "unsupported"
            ;;
        esac
        ;;
    *)
        echo "unsupported"
        ;;
    esac
}

bootstrap_go() {
    emit_warning_if_legacy_bash

    if check_command "$GO_BINARY"; then
        return
    fi

    if [ -x "$GO_LOCAL_BIN/go" ]; then
        GO_BINARY="$GO_LOCAL_BIN/go"
        return
    fi

    if ! require_tools mktemp tar; then
        echo "bootstrap prerequisites missing: mktemp and tar are required" >&2
        return 1
    fi

    local platform
    local archive
    local tmpdir
    local archive_path
    local download_url

    platform="$(go_system_arch)"
    if [ "$platform" = "unsupported" ]; then
        echo "unsupported platform: $(uname -s)/$(uname -m)" >&2
        return 1
    fi

    GO_BOOTSTRAP_VERSION="$(fetch_go_version)"
    if [ -z "$GO_BOOTSTRAP_VERSION" ]; then
        echo "failed to resolve Go bootstrap version" >&2
        return 1
    fi

    archive="$GO_BOOTSTRAP_VERSION.$platform.tar.gz"
    tmpdir="$(mktemp -d)"
    archive_path="$tmpdir/$archive"
    download_url="https://go.dev/dl/$archive"

    if ! download_file "$download_url" "$archive_path"; then
        echo "failed to download Go bootstrap archive" >&2
        rm -rf "$tmpdir"
        return 1
    fi

    rm -rf "$GO_LOCAL_ROOT"
    mkdir -p "$(dirname "$GO_LOCAL_ROOT")"
    tar -xzf "$archive_path" -C "$(dirname "$GO_LOCAL_ROOT")"
    rm -rf "$tmpdir"

    if [ ! -x "$GO_LOCAL_BIN/go" ]; then
        echo "go binary not found after bootstrap install" >&2
        return 1
    fi

    GO_BINARY="$GO_LOCAL_BIN/go"
}

run_dotfiles_go_command() {
    local command="$1"
    shift
	if ! bootstrap_go; then
		return 1
	fi

	if run_dotfiles_binary "$command" "$@"; then
		return 0
	fi

	if [ "$GO_BINARY" = "go" ] && ! check_command "$GO_BINARY"; then
		echo "go command not found on PATH" >&2
		return 1
	fi

    builtin cd "$DOTDOTFILES/lib/dotfilesctl"
    GO111MODULE=on "$GO_BINARY" run ./cmd/dotfilesctl "$command" "$@"
}

run_dotfiles_binary() {
    local command="$1"
    shift

    while true; do
		if [ -x "$DOTFILESCTL_BINARY" ] && ! dotfilesctl_binary_stale; then
            "$DOTFILESCTL_BINARY" "$command" "$@"
            return $?
        fi

        if ! ensure_dotfilesctl_binary; then
            return 1
        fi
    done
}

ensure_dotfilesctl_binary() {
    mkdir -p "$DOTFILESCTL_BINARY_DIR"
	if [ -x "$DOTFILESCTL_BINARY" ] && ! dotfilesctl_binary_stale; then
		return 0
	fi

	if [ "$GO_BINARY" = "go" ]; then
		if ! check_command "$GO_BINARY"; then
			echo "go command not found on PATH" >&2
			return 1
		fi
	elif [ ! -x "$GO_BINARY" ]; then
		echo "go command not executable: $GO_BINARY" >&2
		return 1
    fi

    if ! check_command flock; then
        GO111MODULE=on "$GO_BINARY" build -o "$DOTFILESCTL_BINARY" "$DOTDOTFILES/lib/dotfilesctl/cmd/dotfilesctl"
        return $?
    fi

    (
        flock 9
		if [ -x "$DOTFILESCTL_BINARY" ] && ! dotfilesctl_binary_stale; then
            exit 0
        fi
        GO111MODULE=on "$GO_BINARY" build -o "$DOTFILESCTL_BINARY" "$DOTDOTFILES/lib/dotfilesctl/cmd/dotfilesctl"
        exit $?
    ) 9>"$DOTFILESCTL_BUILD_LOCK_FILE"
    return $?
}

bootstrap_and_run() {
    run_dotfiles_go_command "$@"
}
