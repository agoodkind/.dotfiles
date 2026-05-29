#!/usr/bin/env bash
set -e
set -u
set -o pipefail

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
GO_BINARY="${GO_BINARY:-go}"
GO_LOCAL_ROOT="${GO_LOCAL_ROOT:-$HOME/.local/go}"
GO_LOCAL_BIN="${GO_LOCAL_ROOT}/bin"
GO_BOOTSTRAP_VERSION="${GO_BOOTSTRAP_VERSION:-}"
DOTS_BINARY_DIR="${DOTS_BINARY_DIR:-$HOME/.cache/dots/bin}"
DOTS_BINARY="${DOTS_BINARY:-$DOTS_BINARY_DIR/dots}"
DOTS_BUILD_LOCK_FILE="${DOTS_BUILD_LOCK_FILE:-$DOTS_BINARY_DIR/.dots.build.lock}"
# Sidecar recording the content hash of the sources the cached binary was built
# from. Staleness compares against this instead of file mtimes, so git checkouts
# and syncs that touch mtimes without changing content do not force a rebuild.
DOTS_BUILD_HASH_FILE="${DOTS_BUILD_HASH_FILE:-$DOTS_BINARY_DIR/.dots.build.hash}"
# Bound the wait for the build lock so a wedged build cannot pile up one blocked
# login shell per connection (set to 0 to wait indefinitely, the legacy behavior).
DOTS_BUILD_LOCK_WAIT_SECONDS="${DOTS_BUILD_LOCK_WAIT_SECONDS:-120}"
# Bound the build itself so a hung toolchain download fails instead of hanging
# forever while holding the lock (set to 0 to disable the timeout).
DOTS_BUILD_TIMEOUT_SECONDS="${DOTS_BUILD_TIMEOUT_SECONDS:-600}"
# Bound the Go toolchain download so a host that cannot reach go.dev fails fast
# in bootstrap_go instead of hanging a login shell on every connection.
DOTS_DOWNLOAD_CONNECT_TIMEOUT="${DOTS_DOWNLOAD_CONNECT_TIMEOUT:-20}"
DOTS_DOWNLOAD_MAX_TIME="${DOTS_DOWNLOAD_MAX_TIME:-600}"
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

# required_go_version prints the version from the `go X.Y[.Z]` directive in
# dots/go.mod (e.g. 1.26.3), or an empty string when it cannot be read.
required_go_version() {
    local gomod="$DOTDOTFILES/dots/go.mod"
    if [ ! -f "$gomod" ]; then
        echo ""
        return
    fi
    sed -n 's/^go \([0-9][0-9.]*\).*/\1/p' "$gomod" | head -n 1
}

# current_go_version prints the numeric version (e.g. 1.22.7) reported by the
# given go binary, or an empty string when it cannot be determined.
current_go_version() {
    local gobin="$1"
    local raw=""
    if [ -x "$gobin" ] || command -v "$gobin" >/dev/null 2>&1; then
        raw="$("$gobin" version 2>/dev/null || true)"
    fi
    printf '%s\n' "$raw" | sed -n 's/.*go\([0-9][0-9.]*\).*/\1/p' | head -n 1
}

# go_version_ge returns 0 when HAVE >= WANT, comparing the first three numeric
# fields. An empty WANT (unknown requirement) is always satisfied.
go_version_ge() {
    local have="$1"
    local want="$2"
    if [ -z "$want" ]; then
        return 0
    fi
    if [ -z "$have" ]; then
        return 1
    fi
    local IFS=.
    # shellcheck disable=SC2206
    local have_parts=($have)
    # shellcheck disable=SC2206
    local want_parts=($want)
    local index=0
    local have_field want_field
    while [ "$index" -lt 3 ]; do
        have_field="${have_parts[$index]:-0}"
        want_field="${want_parts[$index]:-0}"
        have_field="${have_field%%[!0-9]*}"
        want_field="${want_field%%[!0-9]*}"
        have_field="${have_field:-0}"
        want_field="${want_field:-0}"
        if [ "$have_field" -gt "$want_field" ]; then
            return 0
        fi
        if [ "$have_field" -lt "$want_field" ]; then
            return 1
        fi
        index=$((index + 1))
    done
    return 0
}

# dots_hash_tool prints a content-hashing command available on the host, or an
# empty string. sha256sum ships with coreutils on Linux; shasum is present by
# default on macOS. Both run on a fresh host before package provisioning.
dots_hash_tool() {
    if check_command sha256sum; then
        echo "sha256sum"
        return
    fi
    if check_command shasum; then
        echo "shasum -a 256"
        return
    fi
    echo ""
}

# dots_source_hash prints a content hash of the build-relevant dots sources
# (path names plus file contents), or an empty string when no hash tool exists.
# Hashing content rather than mtimes keeps the binary from rebuilding when git
# operations touch files without changing them.
dots_source_hash() {
    local tool
    tool="$(dots_hash_tool)"
    if [ -z "$tool" ]; then
        echo ""
        return
    fi
    # shellcheck disable=SC2086
    find "$DOTDOTFILES/dots" -type f \
        \( -name '*.go' -o -name '*.toml' -o -name '*.mod' -o -name '*.sum' \) |
        LC_ALL=C sort |
        while IFS= read -r source_file; do
            printf '%s\n' "$source_file"
            cat "$source_file"
        done | $tool | awk '{print $1}'
}

# write_build_hash records the current source hash next to the binary so the
# next staleness check can compare against it.
write_build_hash() {
    local hash
    hash="$(dots_source_hash)"
    if [ -n "$hash" ]; then
        printf '%s\n' "$hash" >"$DOTS_BUILD_HASH_FILE"
    fi
}

# default_go_bootstrap_version prints the version to install when the go.dev
# version list is unreachable. It tracks go.mod so an offline host still gets a
# go that satisfies the module rather than the hardcoded legacy floor.
default_go_bootstrap_version() {
    local want
    want="$(required_go_version)"
    if [ -n "$want" ]; then
        echo "go$want"
        return
    fi
    echo "$DEFAULT_GO_BOOTSTRAP_VERSION"
}

# dots_go_toolchain prints "local" when the resolved go already satisfies
# go.mod, so the build never reaches out to download a toolchain, or "auto"
# when go cannot be upgraded that far (the macOS 11 cap) and a fetch is the only
# option left.
dots_go_toolchain() {
    if go_version_ge "$(current_go_version "$GO_BINARY")" "$(required_go_version)"; then
        echo "local"
        return
    fi
    echo "auto"
}

dots_binary_stale() {
    if [ ! -x "$DOTS_BINARY" ]; then
        return 0
    fi

    local current_hash
    current_hash="$(dots_source_hash)"
    if [ -n "$current_hash" ]; then
        if [ -f "$DOTS_BUILD_HASH_FILE" ] && [ "$current_hash" = "$(cat "$DOTS_BUILD_HASH_FILE")" ]; then
            return 1
        fi
        return 0
    fi

    # Fallback for hosts without a hash tool: compare source mtimes.
    local source_file
    while IFS= read -r source_file; do
        if [ "$source_file" -nt "$DOTS_BINARY" ]; then
            return 0
        fi
    done < <(
        find "$DOTDOTFILES/dots" -type f \
            \( -name '*.go' -o -name '*.toml' -o -name '*.mod' -o -name '*.sum' \)
    )

    return 1
}

emit_warning_if_legacy_bash() {
    if [ -n "${BASH_VERSINFO-}" ]; then
        if [ "${BASH_VERSINFO[0]}" -lt 3 ] || { [ "${BASH_VERSINFO[0]}" -eq 3 ] && [ "${BASH_VERSINFO[1]}" -lt 2 ]; }; then
            echo "warning: unsupported bash ${BASH_VERSINFO[0]}.${BASH_VERSINFO[1]}; continuing without unsupported features" >&2
            return
        fi
    fi
}

download_file() {
    local url="$1"
    local destination="$2"

    if check_command curl; then
        curl --location --silent --show-error --fail \
            --connect-timeout "$DOTS_DOWNLOAD_CONNECT_TIMEOUT" \
            --max-time "$DOTS_DOWNLOAD_MAX_TIME" \
            "$url" --output "$destination"
        return $?
    fi

    if check_command wget; then
        wget --quiet --tries=2 \
            --timeout="$DOTS_DOWNLOAD_CONNECT_TIMEOUT" \
            --output-document "$destination" "$url"
        return $?
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
        default_go_bootstrap_version
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

    default_go_bootstrap_version
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
                arm64 | aarch64)
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

    local want
    want="$(required_go_version)"

    # Prefer a system go, but only if it satisfies the version dots/go.mod needs.
    # Reusing an older go is what left vault on 1.22.7 against a go 1.26 module,
    # forcing GOTOOLCHAIN=auto to fetch a toolchain over the network on every build.
    if check_command "$GO_BINARY"; then
        if go_version_ge "$(current_go_version "$GO_BINARY")" "$want"; then
            return
        fi
        echo "dots: system go does not satisfy go.mod (need $want); upgrading" >&2
    fi

    # Reuse a previously bootstrapped local go only when it still satisfies go.mod.
    if [ -x "$GO_LOCAL_BIN/go" ]; then
        if go_version_ge "$(current_go_version "$GO_LOCAL_BIN/go")" "$want"; then
            GO_BINARY="$GO_LOCAL_BIN/go"
            return
        fi
        echo "dots: cached go does not satisfy go.mod (need $want); re-downloading" >&2
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

run_dots_go_command() {
    local command="$1"
    shift
    if ! bootstrap_go; then
        return 1
    fi

    if [ -x "$DOTS_BINARY" ] && ! dots_binary_stale; then
        "$DOTS_BINARY" "$command" "$@"
        return $?
    fi

    if ensure_dots_binary; then
        "$DOTS_BINARY" "$command" "$@"
        return $?
    fi

    if [ "$GO_BINARY" = "go" ] && ! check_command "$GO_BINARY"; then
        echo "go command not found on PATH" >&2
        return 1
    fi

    GOTOOLCHAIN="$(dots_go_toolchain)" GO111MODULE=on GOWORK=off "$GO_BINARY" run -C "$DOTDOTFILES/dots" ./cmd/dots "$command" "$@"
}

run_dots_binary() {
    local command="$1"
    shift

    while true; do
        if [ -x "$DOTS_BINARY" ] && ! dots_binary_stale; then
            "$DOTS_BINARY" "$command" "$@"
            return $?
        fi

        if ! ensure_dots_binary; then
            return 1
        fi
    done
}

build_dots_binary() {
    echo "dots: building binary (first run or source changed)..." >&2
    local toolchain
    toolchain="$(dots_go_toolchain)"

    if [ "$DOTS_BUILD_TIMEOUT_SECONDS" -gt 0 ] && check_command timeout; then
        if ! GOTOOLCHAIN="$toolchain" GO111MODULE=on GOWORK=off \
            timeout "$DOTS_BUILD_TIMEOUT_SECONDS" \
            "$GO_BINARY" build -C "$DOTDOTFILES/dots" -o "$DOTS_BINARY" ./cmd/dots; then
            echo "dots: build failed or timed out after ${DOTS_BUILD_TIMEOUT_SECONDS}s" >&2
            return 1
        fi
    else
        if ! GOTOOLCHAIN="$toolchain" GO111MODULE=on GOWORK=off \
            "$GO_BINARY" build -C "$DOTDOTFILES/dots" -o "$DOTS_BINARY" ./cmd/dots; then
            echo "dots: build failed" >&2
            return 1
        fi
    fi

    write_build_hash
}

ensure_dots_binary() {
    mkdir -p "$DOTS_BINARY_DIR"
    if [ -x "$DOTS_BINARY" ] && ! dots_binary_stale; then
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
        build_dots_binary
        return $?
    fi

    (
        if ! flock -n 9; then
            echo "dots: waiting for binary build lock ($DOTS_BUILD_LOCK_FILE)..." >&2
            if [ "$DOTS_BUILD_LOCK_WAIT_SECONDS" -gt 0 ]; then
                if ! flock -w "$DOTS_BUILD_LOCK_WAIT_SECONDS" 9; then
                    echo "dots: timed out after ${DOTS_BUILD_LOCK_WAIT_SECONDS}s waiting for build lock; skipping build" >&2
                    exit 1
                fi
            else
                flock 9
            fi
        fi
        if [ -x "$DOTS_BINARY" ] && ! dots_binary_stale; then
            exit 0
        fi
        build_dots_binary
        exit $?
    ) 9>"$DOTS_BUILD_LOCK_FILE"
    return $?
}

bootstrap_and_run() {
    run_dots_go_command "$@"
}
