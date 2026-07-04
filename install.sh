#!/usr/bin/env bash
set -e
set -u
set -o pipefail

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
DOTFILES_ARCHIVE_URL="${DOTFILES_ARCHIVE_URL:-https://codeload.github.com/agoodkind/.dotfiles/tar.gz/refs/heads/main}"

# Fresh-host bootstrap needs a tiny dependency probe before the repo is present.
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

# When the checkout is missing entirely, pull down a source archive first so we
# can reach dots/bootstrap-go.sh without requiring git on the host yet.
bootstrap_repo_from_archive() {
    local tmpdir
    local archive_path
    local extracted_root
    local dir_count

    if [ -e "$DOTDOTFILES" ] && [ ! -d "$DOTDOTFILES" ]; then
        echo "dotfiles bootstrap target exists and is not a directory: $DOTDOTFILES" >&2
        return 1
    fi

    # Refuse to copy an archive on top of an unrelated populated directory.
    # A partial dotfiles checkout is fine because we can complete it in place.
    local target_has_content=0
    if [ -d "$DOTDOTFILES/.git" ] || [ -n "$(find "$DOTDOTFILES" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]; then
        target_has_content=1
    fi
    if [ -d "$DOTDOTFILES" ] && [ ! -f "$DOTDOTFILES/dots/bootstrap-go.sh" ] && [ "$target_has_content" -eq 1 ]; then
        echo "dotfiles bootstrap target already exists but is not a checkout: $DOTDOTFILES" >&2
        return 1
    fi

    if ! check_command mktemp || ! check_command tar; then
        echo "dotfiles bootstrap requires mktemp and tar" >&2
        return 1
    fi

    # Older bash can keep running after a failed assignment under set -e, so
    # guard mktemp before deriving archive paths or installing the cleanup trap.
    if ! tmpdir="$(mktemp -d)"; then
        echo "dotfiles bootstrap could not create a temporary directory" >&2
        return 1
    fi
    archive_path="$tmpdir/dotfiles.tar.gz"
    trap 'rm -rf "$tmpdir"' EXIT

    echo "dotfiles: downloading installer repository archive..." >&2
    download_file "$DOTFILES_ARCHIVE_URL" "$archive_path"

    # GitHub source archives unpack into a single top-level directory. Validate
    # that shape so the copy step only imports the expected repository tree.
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
    # Archive bootstraps do not have git metadata yet, so the first install run
    # must skip git-backed steps until dots hydrates a managed checkout.
    set -- --skip-git "$@"
fi
bootstrap_and_run install "$@"
