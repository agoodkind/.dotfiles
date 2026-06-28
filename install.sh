#!/usr/bin/env bash
set -e
set -u
set -o pipefail

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
DOTFILES_ARCHIVE_URL="${DOTFILES_ARCHIVE_URL:-https://codeload.github.com/agoodkind/.dotfiles/tar.gz/refs/heads/main}"

check_command() {
    command -v "$1" >/dev/null 2>&1
}

download_file() {
    local url="$1"
    local destination="$2"

    if check_command curl; then
        curl --location --silent --show-error --fail "$url" --output "$destination"
        return $?
    fi

    if check_command wget; then
        wget --quiet --output-document "$destination" "$url"
        return $?
    fi

    echo "dotfiles bootstrap requires curl or wget" >&2
    return 1
}

bootstrap_repo_from_archive() {
    local tmpdir
    local archive_path
    local extracted_root
    local dir_count

    if [ -e "$DOTDOTFILES" ] && [ ! -d "$DOTDOTFILES" ]; then
        echo "dotfiles bootstrap target exists and is not a directory: $DOTDOTFILES" >&2
        return 1
    fi

    if [ -d "$DOTDOTFILES" ] && [ ! -f "$DOTDOTFILES/dots/bootstrap-go.sh" ] && { [ -d "$DOTDOTFILES/.git" ] || [ -n "$(ls -A "$DOTDOTFILES" 2>/dev/null)" ]; }; then
        echo "dotfiles bootstrap target already exists but is not a checkout: $DOTDOTFILES" >&2
        return 1
    fi

    if ! check_command mktemp || ! check_command tar; then
        echo "dotfiles bootstrap requires mktemp and tar" >&2
        return 1
    fi

    tmpdir="$(mktemp -d)"
    archive_path="$tmpdir/dotfiles.tar.gz"
    trap 'rm -rf "$tmpdir"' EXIT

    echo "dotfiles: downloading installer repository archive..." >&2
    download_file "$DOTFILES_ARCHIVE_URL" "$archive_path"
    tar -xzf "$archive_path" -C "$tmpdir"
    dir_count="$(find "$tmpdir" -mindepth 1 -maxdepth 1 -type d | wc -l | awk '{print $1}')"
    if [ "$dir_count" -ne 1 ]; then
        echo "dotfiles bootstrap archive must contain exactly one top-level directory" >&2
        return 1
    fi
    extracted_root="$(find "$tmpdir" -mindepth 1 -maxdepth 1 -type d)"
    if [ ! -f "$extracted_root/dots/bootstrap-go.sh" ]; then
        echo "dotfiles bootstrap archive is missing dots/bootstrap-go.sh" >&2
        return 1
    fi

    mkdir -p "$DOTDOTFILES"
    cp -R "$extracted_root"/. "$DOTDOTFILES"/
    trap - EXIT
    rm -rf "$tmpdir"
}

if [ ! -f "$DOTDOTFILES/dots/bootstrap-go.sh" ]; then
    bootstrap_repo_from_archive
fi

source "$DOTDOTFILES/dots/bootstrap-go.sh"
if [ ! -d "$DOTDOTFILES/.git" ]; then
    set -- --skip-git "$@"
fi
bootstrap_and_run install "$@"
