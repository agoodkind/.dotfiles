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

    local seen_secret_names=""
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

        case "$secret_name" in
            HOME | PATH | LD_PRELOAD | DYLD_INSERT_LIBRARIES)
                continue
                ;;
        esac

        case " $seen_secret_names " in
            *" $secret_name "*)
                continue
                ;;
        esac

        if [[ ! -r $secret_file ]]; then
            continue
        fi
        if ! secret_value="$(<"$secret_file" 2>/dev/null)"; then
            continue
        fi
        secret_value="${secret_value%$'\r'}"
        if [[ -z $secret_value ]]; then
            continue
        fi

        export "${secret_name}=${secret_value}"
        seen_secret_names="$seen_secret_names $secret_name"
    done

    if [[ -n ${GH_TOKEN:-} && -z ${GITHUB_TOKEN+x} ]]; then
        export GITHUB_TOKEN="$GH_TOKEN"
    fi
}
_dotfiles_load_secrets
