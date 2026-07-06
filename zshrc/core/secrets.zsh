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

    # Mirror GH_TOKEN onto GITHUB_TOKEN for tools that accept either name.
    if [[ -n ${GH_TOKEN:-} ]]; then
        export GITHUB_TOKEN="$GH_TOKEN"
    fi
}
_dotfiles_load_secrets
