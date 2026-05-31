###############################################################################
# plugins #####################################################################
###############################################################################

fpath=("$DOTDOTFILES/zshrc/completions" $fpath)

typeset -gA ZINIT
ZINIT[OPTIMIZE_OUT_DISK_ACCESSES]=1

typeset -ga _PERF_TREE_DEFERRED=()

# Tab: accept the zsh-autosuggestion if one is showing, otherwise call the
# fzf-tab completion menu. POSTDISPLAY is set by zsh-autosuggestions to the
# grayed-out suggestion text. The widget and bindkey lines are installed in
# tier 2, after fzf-tab has registered fzf-tab-complete.
function _dotfiles_tab_accept_or_complete() {
    if [[ -n "$POSTDISPLAY" ]]; then
        zle autosuggest-accept
    else
        zle fzf-tab-complete
    fi
}

function _ready_mark() {
    local mark_depth=$1 mark_label=$2 mark_tag=${3:-}
    local mark_now=$EPOCHREALTIME
    local mark_ms=$(((mark_now - _ready_lap) * 1000))
    _PERF_TREE_DEFERRED+=("${mark_depth}:${mark_label}:${mark_ms}${mark_tag:+:${mark_tag}}")
    _ready_lap=$mark_now
}

# Tier 1: runs on first keystroke (zle-line-init). Minimal set needed for
# basic interactive use — zinit core, compinit, and autosuggestions.
function _load_tier1() {
    setopt local_options glob bare_glob_qual

    if ((_ZINIT_LOADED != 0)); then
        return 0
    fi
    _ZINIT_LOADED=1
    typeset -g _ready_lap=$EPOCHREALTIME

    _ready_mark 1 "tier 1" "zle-line-init"

    source "$DOTDOTFILES/lib/zinit/zinit.zsh"
    ZINIT[AUTO_UPDATE_DAYS]=365
    autoload -Uz _zinit
    if ((${+_comps} != 0)); then
        _comps[zinit]=_zinit
    fi
    _ready_mark 2 zinit_core

    ZINIT[COMPINIT_OPTS]=-C
    zicompinit
    zicdreplay
    _ready_mark 2 compinit

    # Drop forward-char and vi-forward-char from the accept-widget list BEFORE
    # the plugin loads, so its single bind pass picks up the trimmed list.
    typeset -ga ZSH_AUTOSUGGEST_ACCEPT_WIDGETS=(end-of-line vi-end-of-line vi-add-eol)
    zinit light zsh-users/zsh-autosuggestions
    _zsh_autosuggest_start
    _ready_mark 2 autosuggestions

    _PROFILE_TIMES[_time_to_ready_t1]=$(((EPOCHREALTIME - START_TIME) * 1000))
    sched +0 _load_tier2
}

# Tier 2: runs at next event loop pass after tier 1. Syntax highlighting and
# fzf-tab. Things you won't need in the first ~100ms.
function _load_tier2() {
    setopt local_options glob bare_glob_qual

    _ready_mark 1 "tier 2" "sched +0"

    zinit light zsh-users/zsh-syntax-highlighting
    _ready_mark 2 syntax-hl

    zinit light Aloxaf/fzf-tab
    zinit light Freed-Wu/fzf-tab-source
    if (($+commands[fzf] != 0)); then
        source <(fzf --zsh)
    fi
    # Install the Tab widget after fzf-tab has registered fzf-tab-complete.
    zle -N _dotfiles_tab_accept_or_complete
    bindkey -M emacs '^I' _dotfiles_tab_accept_or_complete
    bindkey -M viins '^I' _dotfiles_tab_accept_or_complete
    _ready_mark 2 fzf-tab

    # shellcheck disable=SC2016
    zstyle ":fzf-tab:complete:cd:*" fzf-preview "eza -1 --color=always \$realpath"
    zstyle ":fzf-tab:complete:git-(add|diff|restore):*" fzf-preview "git diff \$word | delta"
    zstyle ":fzf-tab:complete:curl:*" fzf-preview "echo \$desc"
    zstyle ":fzf-tab:complete:*:options" fzf-preview "echo \$desc"
    zstyle ":fzf-tab:complete:*:argument-rest" fzf-preview
    _zsplit_colon "$LS_COLORS"
    zstyle ":completion:*" list-colors "${_ZSH_ARR[@]}"
    zstyle ":completion:*" menu no

    if (($+commands[gcp] != 0)); then
        unalias gcp 2>/dev/null
        compdef -d gcp 2>/dev/null
        compdef _files gcp 2>/dev/null
    fi

    sched +0 _load_tier3
}

# Tier 3: extra completions and cleanup. Runs after tier 2.
function _load_tier3() {
    setopt local_options glob bare_glob_qual

    _ready_mark 1 "tier 3" "sched +0"

    zinit light zsh-users/zsh-completions
    _ready_mark 2 completions

    local _zc_dir="${ZINIT[COMPLETIONS_DIR]:-$HOME/.local/share/zinit/completions}"
    setopt local_options null_glob
    for _zc_link in "$_zc_dir"/*; do
        if [[ ! -L "$_zc_link" ]]; then
            continue
        fi
        if [[ ! -e "$_zc_link" ]]; then
            rm -f "$_zc_link"
        fi
    done
    unset _zc_dir _zc_link
    _ready_mark 2 cleanup

    _PROFILE_TIMES[_time_to_ready]=$(((EPOCHREALTIME - START_TIME) * 1000))

    _write_startup_log
    _ready_mark 2 startup_log
}

# Fires once after prompt is drawn, before first keystroke is accepted.
function _zinit_line_init() {
    _load_tier1
    zle -D zle-line-init 2>/dev/null
}
zle -N zle-line-init _zinit_line_init

###############################################################################
