###############################################################################
# plugins #####################################################################
###############################################################################

# fpath must be set synchronously so completions work on first tab
fpath=("$DOTDOTFILES/zshrc/completions" $fpath)

# Must be declared before zsh-defer fires so zinit doesn't
# hit an already-typed scalar on re-source
typeset -gA ZINIT
ZINIT[OPTIMIZE_OUT_DISK_ACCESSES]=1
ZINIT[AUTO_UPDATE_DAYS]=365

source "$DOTDOTFILES/lib/zsh-defer/zsh-defer.plugin.zsh"

_load_zinit() {

    source "$DOTDOTFILES/lib/zinit/zinit.zsh"
    autoload -Uz _zinit
    (( ${+_comps} )) && _comps[zinit]=_zinit

    # shellcheck disable=SC2016
    zinit wait lucid atload'
        if (( $+commands[gcp] )); then
            unalias gcp 2>/dev/null
            compdef -d gcp 2>/dev/null
            compdef _files gcp 2>/dev/null
        fi
    ' for \
        OMZP::git

    zinit wait lucid blockf atpull'zinit creinstall -q .' for \
        zsh-users/zsh-completions

    zinit wait lucid atload'_zsh_autosuggest_start' for \
        zsh-users/zsh-autosuggestions

    # shellcheck disable=SC2016
    zinit wait lucid atinit'
        local _zc_dir="${ZINIT[COMPLETIONS_DIR]:-$HOME/.local/share/zinit/completions}"
        for _zc_link in "$_zc_dir"/*(N@); do
            [[ -e "$_zc_link" ]] || rm -f "$_zc_link"
        done
        unset _zc_dir _zc_link
        ZINIT[COMPINIT_OPTS]=-C
        zicompinit
        zicdreplay
        if (( $+commands[gcp] )); then
            unalias gcp 2>/dev/null
            compdef -d gcp 2>/dev/null
            compdef _files gcp 2>/dev/null
        fi
    ' for \
        zdharma-continuum/fast-syntax-highlighting

    # shellcheck disable=SC2016
    zinit wait'1' lucid atload'
        zstyle ":fzf-tab:complete:cd:*" fzf-preview "eza -1 --color=always \$realpath"
        zstyle ":fzf-tab:complete:git-(add|diff|restore):*" fzf-preview "git diff \$word | delta"
        zstyle ":completion:*" list-colors ${(s.:.)LS_COLORS}
        zstyle ":completion:*" menu no
        if (( $+commands[gcp] )); then
            unalias gcp 2>/dev/null
            compdef -d gcp 2>/dev/null
            compdef _files gcp 2>/dev/null
        fi
    ' for \
        Aloxaf/fzf-tab \
        Freed-Wu/fzf-tab-source

    zinit wait lucid for \
        is-snippet /opt/homebrew/opt/fzf/shell/key-bindings.zsh

    zinit wait'2' lucid \
        atload'_PROFILE_TIMES[zinit_turbo]=$(( (EPOCHREALTIME - START_TIME) * 1000 )); _write_startup_log; do_profile' \
        for is-snippet "$DOTDOTFILES/zshrc/core/_sentinel.zsh"
}

zsh-defer _load_zinit

###############################################################################
