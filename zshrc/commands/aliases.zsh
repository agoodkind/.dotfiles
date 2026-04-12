# shellcheck shell=bash

# pbcopy wrapper: on macOS use native, otherwise ssh to source host
function pbcopy() {
    if is_macos; then
        if [[ $# -gt 0 ]]; then
            echo -n "$*" | /usr/bin/pbcopy
        else
            /usr/bin/pbcopy
        fi
    else
        if [[ -z "$SSH_SOURCE_HOST" ]]; then
            echo "pbcopy: SSH_SOURCE_HOST not set" >&2
            return 1
        fi
        if [[ $# -gt 0 ]]; then
            echo -n "$*" | ssh "$SSH_SOURCE_HOST" /usr/bin/pbcopy 2>/dev/null
        else
            ssh "$SSH_SOURCE_HOST" /usr/bin/pbcopy 2>/dev/null
        fi
    fi
}

# claude wrapper: route through agent-gate daemon for per-session model
# isolation, then inject default flags for the real claude binary.
# ARGV0=claude makes agent-gate enter wrapper mode (checks argv[0]).
function claude() {
    ARGV0=claude "$HOME/go/bin/agent-gate" --remote-control --dangerously-skip-permissions "$@"
}

# gh wrapper: intercept `gh upload` subcommand
function gh() {
    if [[ "${1:-}" == "upload" ]]; then
        shift
        "${DOTFILES_DIR:-$HOME/.dotfiles}/lib/scripts/gh-upload" "$@"
    else
        command gh "$@"
    fi
}
prefer gh-upload "${DOTFILES_DIR:-$HOME/.dotfiles}/lib/scripts/gh-upload"
prefer disable-macos-resume "${DOTFILES_DIR:-$HOME/.dotfiles}/bin/disable-macos-resume"

# thefuck wrapper: lazy load on first use
function fuck() {
    unfunction fuck
    eval "$(thefuck --alias)"
    fuck "$(fc -ln -1)"
}
