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

    local secret_file base secret_name secret_value
    for secret_file in "$secrets_dir"/*; do
        if [[ ! -f $secret_file ]]; then
            continue
        fi

        base="${secret_file##*/}"
        # File-based secrets keep an extension and are referenced by path.
        if [[ $base == *.* ]]; then
            continue
        fi

        secret_name="$(
            printf '%s' "$base" |
                tr '[:lower:]' '[:upper:]' |
                tr -c 'A-Za-z0-9_' '_'
        )"
        if [[ -z $secret_name ]]; then
            continue
        fi
        if [[ $secret_name == [0-9]* ]]; then
            continue
        fi

        secret_value="$(<"$secret_file")"
        export "${secret_name}=${secret_value}"
    done

    if [[ -n ${GH_TOKEN:-} ]]; then
        export GITHUB_TOKEN="$GH_TOKEN"
    fi
}
_dotfiles_load_secrets
