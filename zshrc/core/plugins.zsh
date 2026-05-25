###############################################################################
# plugins #####################################################################
###############################################################################

fpath=("$DOTDOTFILES/zshrc/completions" $fpath)

typeset -gA ZINIT
ZINIT[OPTIMIZE_OUT_DISK_ACCESSES]=1

typeset -ga _PERF_TREE_DEFERRED=()
typeset -ga _DOTFILES_TAB_KEYMAPS=(main emacs viins)
typeset -gA _DOTFILES_TAB_FALLBACK_WIDGETS=()

function _dotfiles_capture_tab_fallback_widget() {
    local keymap=$1
    local current_binding=''
    local current_widget=''

    current_binding=$(builtin bindkey -M "$keymap" '^I') || current_binding=''
    current_widget="${${current_binding##* }:-expand-or-complete}"
    if [[ "$current_widget" == "_dotfiles_tab_accept_or_complete" ]]; then
        if [[ "$keymap" == main ]]; then
            current_widget="${_DOTFILES_TAB_FALLBACK_WIDGETS[$keymap]}"
        else
            current_widget="${_DOTFILES_TAB_FALLBACK_WIDGETS[main]:-${_DOTFILES_TAB_FALLBACK_WIDGETS[$keymap]}}"
        fi
    fi
    if [[ -z "$current_widget" ]]; then
        current_widget=expand-or-complete
    fi

    _DOTFILES_TAB_FALLBACK_WIDGETS[$keymap]=$current_widget
}

function _dotfiles_rebind_tab_accept_widget() {
    local keymap

    for keymap in "${_DOTFILES_TAB_KEYMAPS[@]}"; do
        _dotfiles_capture_tab_fallback_widget "$keymap"
        builtin bindkey -M "$keymap" '^I' _dotfiles_tab_accept_or_complete
    done
}

function _dotfiles_tab_accept_or_complete() {
    local active_keymap=${KEYMAP:-main}
    local fallback_widget=${_DOTFILES_TAB_FALLBACK_WIDGETS[$active_keymap]:-${_DOTFILES_TAB_FALLBACK_WIDGETS[main]:-expand-or-complete}}

    if [[ -n "$POSTDISPLAY" ]]; then
        if builtin zle -l autosuggest-accept >/dev/null 2>&1; then
            zle autosuggest-accept
            return 0
        fi
    fi
    if [[ -z "$fallback_widget" || "$fallback_widget" == "_dotfiles_tab_accept_or_complete" ]]; then
        fallback_widget=expand-or-complete
    fi

    zle "$fallback_widget"
}
zle -N _dotfiles_tab_accept_or_complete

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

    zinit light zsh-users/zsh-autosuggestions
    _zsh_autosuggest_start
    ZSH_AUTOSUGGEST_ACCEPT_WIDGETS=(${ZSH_AUTOSUGGEST_ACCEPT_WIDGETS:#forward-char})
    ZSH_AUTOSUGGEST_ACCEPT_WIDGETS=(${ZSH_AUTOSUGGEST_ACCEPT_WIDGETS:#vi-forward-char})
    if ((${+functions[_zsh_autosuggest_bind_widgets]} != 0)); then
        _zsh_autosuggest_bind_widgets
    fi
    _dotfiles_rebind_tab_accept_widget
    _ready_mark 2 autosuggestions

    _PROFILE_TIMES[_time_to_ready_t1]=$(((EPOCHREALTIME - START_TIME) * 1000))
    sched +0 _load_tier2
}

# Tier 2: runs at next event loop pass after tier 1. Syntax highlighting and
# fzf-tab. Things you won't need in the first ~100ms.
function _load_tier2() {
    setopt local_options glob bare_glob_qual

    _ready_mark 1 "tier 2" "sched +0"

    zinit light zdharma-continuum/fast-syntax-highlighting
    _ready_mark 2 syntax-hl

    zinit light Aloxaf/fzf-tab
    zinit light Freed-Wu/fzf-tab-source
    if (($+commands[fzf] != 0)); then
        source <(fzf --zsh)
    fi
    _dotfiles_rebind_tab_accept_widget
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
