# shellcheck shell=bash
###############################################################################
# Secrets loader: export each ~/.secrets/<name> as an env var
###############################################################################
function _dotfiles_load_secrets() {
    local secrets_dir="$HOME/.secrets"
    if [[ ! -d $secrets_dir ]]; then
        return 0
    fi

    setopt local_options null_glob

    local secret_file base
    for secret_file in "$secrets_dir"/*; do
        if [[ ! -f $secret_file ]]; then
            continue
        fi

        base="${secret_file##*/}"
        # File-based secrets keep an extension and are referenced by path.
        if [[ $base == *.* ]]; then
            continue
        fi

        export "${base:u}=$(<"$secret_file")"
    done
}

# Export the gh keyring login as GH_TOKEN so the value stays current across
# `gh auth login` and `gh auth refresh`. A ~/.secrets/gh_token file still wins.
# The inherited variables are cleared for the lookup because gh echoes a set
# GH_TOKEN back, which would keep a stale parent value alive.
function _dotfiles_load_gh_token() {
    if [[ -f "$HOME/.secrets/gh_token" ]]; then
        return 0
    fi
    if ! command -v gh >/dev/null 2>&1; then
        return 0
    fi

    local token
    if ! token="$(env -u GH_TOKEN -u GITHUB_TOKEN gh auth token --hostname github.com 2>/dev/null)" || [[ -z $token ]]; then
        print -u2 "secrets.zsh: gh auth token failed; GH_TOKEN and GITHUB_TOKEN are unset (run gh auth status)"
        unset GH_TOKEN GITHUB_TOKEN
        return 0
    fi
    export GH_TOKEN="$token"
}

# Mirror GH_TOKEN onto GITHUB_TOKEN for tools that accept either name.
function _dotfiles_mirror_gh_token() {
    if [[ -n ${GH_TOKEN:-} ]]; then
        export GITHUB_TOKEN="$GH_TOKEN"
    fi
}

function _dotfiles_load_gh_token_and_mirror() {
    _dotfiles_load_gh_token
    _dotfiles_mirror_gh_token
}

_dotfiles_load_secrets
_dotfiles_mirror_gh_token
# `gh auth token` costs about 50ms, over the 20ms zshrc budget. Interactive
# shells defer it from incl.zsh once zsh-defer is loaded. Other shells stop
# before the plugins load and need the token for their first command, so they
# look it up now.
if ((! DOTFILES_INTERACTIVE)); then
    _dotfiles_load_gh_token_and_mirror
fi
