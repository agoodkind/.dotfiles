#!/usr/bin/env bash
set -e
set -u
set -o pipefail

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
# Load shell-side bootstrap config (Go versions, timeouts) that must be available
# before Go exists, so it cannot live in the Go-read TOML catalog. Each value is
# still defaulted below, so a missing file is harmless.
if [ -f "$DOTDOTFILES/config/bootstrap.env" ]; then
    # shellcheck disable=SC1090,SC1091
    . "$DOTDOTFILES/config/bootstrap.env"
fi
GO_BINARY="${GO_BINARY:-go}"
GO_LOCAL_ROOT="${GO_LOCAL_ROOT:-$HOME/.local/go}"
GO_LOCAL_BIN="${GO_LOCAL_ROOT}/bin"
GO_BOOTSTRAP_VERSION="${GO_BOOTSTRAP_VERSION:-}"
DOTS_BINARY_DIR="${DOTS_BINARY_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/dots/bin}"
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
DOTS_DOWNLOAD_RETRY_COUNT="${DOTS_DOWNLOAD_RETRY_COUNT:-3}"
DOTS_DOWNLOAD_RETRY_DELAY_SECONDS="${DOTS_DOWNLOAD_RETRY_DELAY_SECONDS:-2}"
DEFAULT_GO_BOOTSTRAP_VERSION="${DEFAULT_GO_BOOTSTRAP_VERSION:-go1.22.7}"
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

# with_lock runs a command while holding an exclusive flock on LOCKFILE. It tries
# non-blocking first, then waits up to WAIT_SECONDS (0 waits forever), and gives
# up with status 75 rather than blocking a login shell indefinitely. When flock
# is unavailable the command runs unlocked. This is the single primitive that
# serializes every mutating bootstrap step (the Go install and the binary build)
# so concurrent logins cannot race the same files.
with_lock() {
    local lockfile="$1"
    local wait_seconds="$2"
    shift 2
    mkdir -p "$(dirname "$lockfile")"
    if ! check_command flock; then
        "$@"
        return $?
    fi
    (
        if ! flock -n 9; then
            echo "dots: waiting for lock $lockfile..." >&2
            if [ "$wait_seconds" -gt 0 ]; then
                if ! flock -w "$wait_seconds" 9; then
                    echo "dots: timed out after ${wait_seconds}s waiting for lock $lockfile; skipping" >&2
                    exit 75
                fi
            else
                flock 9
            fi
        fi
        "$@"
    ) 9>"$lockfile"
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

# current_go_version prints the numeric version (e.g. 1.22.7) of the given go
# binary itself, or an empty string when it cannot be determined. GOTOOLCHAIN
# is forced to local: without it, running `go version` from inside a module
# whose go.mod requires a newer toolchain makes go silently switch to and report
# that downloaded toolchain's version instead of its own, which fooled the reuse
# check into keeping an old go that then failed the GOTOOLCHAIN=local build.
current_go_version() {
    local gobin="$1"
    local raw=""
    if [ -x "$gobin" ] || command -v "$gobin" >/dev/null 2>&1; then
        raw="$(GOTOOLCHAIN=local "$gobin" version 2>/dev/null || true)"
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

# dots_build_input_hash prints a content hash of the files that actually compile
# into the dots binary: this module's GoFiles and EmbedFiles as reported by
# `go list -deps`, plus go.mod and go.sum. Test files and the runtime-read config
# (catalog.toml) are excluded because go list does not report them as build
# inputs, so editing a test or a config file never forces a rebuild. Prints an
# empty string when go or a hash tool is unavailable, so the caller can fall back
# to an mtime comparison.
dots_build_input_hash() {
    local tool
    tool="$(dots_hash_tool)"
    if [ -z "$tool" ]; then
        echo ""
        return
    fi
    if [ ! -x "$GO_BINARY" ] && ! check_command "$GO_BINARY"; then
        echo ""
        return
    fi
    local module="goodkind.io/.dotfiles"
    local list_tmpl
    list_tmpl='{{if .Module}}{{if eq .Module.Path "'"$module"'"}}{{$d:=.Dir}}{{range .GoFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .EmbedFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{end}}{{end}}'
    # shellcheck disable=SC2086
    {
        (
            cd "$DOTDOTFILES/dots" 2>/dev/null || exit 0
            GOTOOLCHAIN=local GO111MODULE=on GOWORK=off \
                "$GO_BINARY" list -deps -f "$list_tmpl" ./cmd/dots 2>/dev/null || true
        )
        printf '%s\n%s\n' "$DOTDOTFILES/dots/go.mod" "$DOTDOTFILES/dots/go.sum"
    } | LC_ALL=C sort -u |
        while IFS= read -r input_file; do
            [ -n "$input_file" ] || continue
            printf '%s\n' "$input_file"
            cat "$input_file" 2>/dev/null || true
        done | $tool | awk '{print $1}'
}

# write_build_hash records the build-input hash next to the binary so the next
# staleness check can compare against it without rebuilding.
write_build_hash() {
    local hash
    hash="$(dots_build_input_hash)"
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
    current_hash="$(dots_build_input_hash)"
    if [ -n "$current_hash" ]; then
        if [ -f "$DOTS_BUILD_HASH_FILE" ] && [ "$current_hash" = "$(cat "$DOTS_BUILD_HASH_FILE")" ]; then
            return 1
        fi
        return 0
    fi

    # Fallback when go or a hash tool is unavailable: compare mtimes of the
    # compiled sources, excluding test files and the runtime-read config so that
    # a test edit or a config edit does not force a rebuild.
    local source_file
    while IFS= read -r source_file; do
        if [ "$source_file" -nt "$DOTS_BINARY" ]; then
            return 0
        fi
    done < <(
        find "$DOTDOTFILES/dots" -type f \
            \( -name '*.go' -o -name '*.mod' -o -name '*.sum' -o -name '*.tmpl' \) \
            ! -name '*_test.go'
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
    local attempt_count=1

    if ! require_tools rm sleep; then
        echo "bootstrap prerequisites missing: rm and sleep are required" >&2
        return 1
    fi

    while true; do
        rm -f "$destination"

        if check_command curl; then
            if curl --location --silent --show-error --fail \
                --connect-timeout "$DOTS_DOWNLOAD_CONNECT_TIMEOUT" \
                --max-time "$DOTS_DOWNLOAD_MAX_TIME" \
                "$url" --output "$destination"; then
                return 0
            fi
        elif check_command wget; then
            if wget --quiet --tries=1 \
                --timeout="$DOTS_DOWNLOAD_CONNECT_TIMEOUT" \
                --output-document "$destination" "$url"; then
                return 0
            fi
        else
            echo "missing curl or wget for bootstrap download" >&2
            return 1
        fi

        if [ "$attempt_count" -ge "$DOTS_DOWNLOAD_RETRY_COUNT" ]; then
            return 1
        fi

        attempt_count=$((attempt_count + 1))
        sleep "$DOTS_DOWNLOAD_RETRY_DELAY_SECONDS"
    done
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

# do_go_install downloads a Go that satisfies go.mod and installs it into
# GO_LOCAL_ROOT. It is always run through with_lock so the rm -rf and extract
# never overlap another login's install or build.
do_go_install() {
    # Another login may have installed a satisfying Go while we waited for the lock.
    if [ -x "$GO_LOCAL_BIN/go" ] && go_version_ge "$(current_go_version "$GO_LOCAL_BIN/go")" "$(required_go_version)"; then
        return 0
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

    # Install under the shared lock so concurrent logins cannot race the rm -rf
    # and re-extract of GO_LOCAL_ROOT, which on vault clobbered a freshly
    # downloaded 1.26 back to the old 1.22 and broke the build.
    if ! with_lock "$DOTS_BUILD_LOCK_FILE" "$DOTS_BUILD_LOCK_WAIT_SECONDS" do_go_install; then
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
    local build_log
    toolchain="$(dots_go_toolchain)"
    if ! build_log="$(mktemp)"; then
        echo "dots: failed to create build log" >&2
        return 1
    fi

    if [ "$DOTS_BUILD_TIMEOUT_SECONDS" -gt 0 ] && check_command timeout; then
        if ! GOTOOLCHAIN="$toolchain" GO111MODULE=on GOWORK=off \
            timeout "$DOTS_BUILD_TIMEOUT_SECONDS" \
            "$GO_BINARY" build -C "$DOTDOTFILES/dots" -o "$DOTS_BINARY" ./cmd/dots >"$build_log" 2>&1; then
            cat "$build_log" >&2
            rm -f "$build_log"
            echo "dots: build failed or timed out after ${DOTS_BUILD_TIMEOUT_SECONDS}s" >&2
            return 1
        fi
    else
        if ! GOTOOLCHAIN="$toolchain" GO111MODULE=on GOWORK=off \
            "$GO_BINARY" build -C "$DOTDOTFILES/dots" -o "$DOTS_BINARY" ./cmd/dots >"$build_log" 2>&1; then
            cat "$build_log" >&2
            rm -f "$build_log"
            echo "dots: build failed" >&2
            return 1
        fi
    fi

    rm -f "$build_log"
    write_build_hash
}

# locked_build re-checks staleness under the lock then builds. It runs inside
# with_lock so a concurrent Go reinstall cannot delete the toolchain mid-build,
# and a second login that lost the race skips rebuilding the up-to-date binary.
locked_build() {
    if [ -x "$DOTS_BINARY" ] && ! dots_binary_stale; then
        return 0
    fi
    build_dots_binary
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

    with_lock "$DOTS_BUILD_LOCK_FILE" "$DOTS_BUILD_LOCK_WAIT_SECONDS" locked_build
}

bootstrap_and_run() {
    run_dots_go_command "$@"
}
