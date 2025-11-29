###############################################################################
# plugins #####################################################################
###############################################################################

# Reduce disk access for faster startup
typeset -A ZINIT
ZINIT[OPTIMIZE_OUT_DISK_ACCESSES]=1

### Zinit
source "$DOTDOTFILES/lib/zinit/zinit.zsh"
autoload -Uz _zinit
(( ${+_comps} )) && _comps[zinit]=_zinit

# Completions with custom fpath
fpath=("$DOTDOTFILES/lib/completions" $fpath)

# Core plugins with staggered loading
# shellcheck disable=SC2016
zinit wait lucid atload'
    # OMZP::git creates gcp alias for git cherry-pick
    # Remove it so the real gcp command is used on Linux
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

# Syntax highlighting with compinit (must run before zicdreplay)
# shellcheck disable=SC2016
zinit wait lucid atinit'
    ZINIT[COMPINIT_OPTS]=-C
    zicompinit
    zicdreplay
    # Ensure gcp uses file completion (not git)
    if (( $+commands[gcp] )); then
        unalias gcp 2>/dev/null
        compdef -d gcp 2>/dev/null
        compdef _files gcp 2>/dev/null
    fi
' for \
    zdharma-continuum/fast-syntax-highlighting

# fzf-tab with configuration
# shellcheck disable=SC2016
zinit wait'1' lucid atload'
    zstyle ":fzf-tab:complete:cd:*" fzf-preview "eza -1 --color=always \$realpath"
    zstyle ":fzf-tab:complete:git-(add|diff|restore):*" fzf-preview "git diff \$word | delta"
    zstyle ":completion:*" list-colors ${(s.:.)LS_COLORS}
    zstyle ":completion:*" menu no
    # Ensure gcp uses file completion (not git)
    if (( $+commands[gcp] )); then
        unalias gcp 2>/dev/null
        compdef -d gcp 2>/dev/null
        compdef _files gcp 2>/dev/null
    fi
' for \
    Aloxaf/fzf-tab \
    Freed-Wu/fzf-tab-source

###############################################################################