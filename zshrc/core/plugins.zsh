###############################################################################
# plugins #####################################################################
###############################################################################

fpath=("$DOTDOTFILES/zshrc/completions" $fpath)

typeset -gA ZINIT
ZINIT[OPTIMIZE_OUT_DISK_ACCESSES]=1

typeset -ga _PERF_TREE_DEFERRED=()

# Runs zsh completion without fzf-tab and without printing the match list.
function _dotfiles_complete_without_list() {
    local complete_status=0
    _ftb__main_complete "$@" || complete_status=$?
    compstate[list]=''
    if ((compstate[nmatches] > 1)); then
        compstate[insert]=unambiguous
    fi
    return $complete_status
}

# Accepts the autosuggestion only when completion cannot change the line.
function _dotfiles_tab_complete_or_accept() {
    if [[ -z "$POSTDISPLAY" ]]; then
        zle fzf-tab-complete
        return
    fi

    local buffer_before=$BUFFER
    zle _dotfiles_complete_without_list
    if [[ "$BUFFER" == "$buffer_before" ]]; then
        zle autosuggest-accept
        return
    fi
    # zsh-autosuggestions does not wrap widgets that start with an underscore.
    POSTDISPLAY=
    zle autosuggest-fetch
}

# Offers every known host in fzf when the typed host has at most one match.
function _dotfiles_hosts_or_all() {
    # Plain zsh completion would replace the typed host with the shared prefix.
    if ((! IN_FZF_TAB)); then
        _dotfiles_hosts_original "$@"
        return
    fi

    local -a host_matches
    _dotfiles_hosts_original -O host_matches "$@"
    if ((${#host_matches} > 1)); then
        _dotfiles_hosts_original "$@"
        return
    fi

    local PREFIX='' SUFFIX=''
    _dotfiles_hosts_original "$@"
    # fzf-tab skips its menu when an unambiguous prefix exists unless list has force.
    compstate[list]+=' force'
}

function _ready_mark() {
    local mark_depth=$1 mark_label=$2 mark_tag=${3:-}
    local mark_now=$EPOCHREALTIME
    local mark_ms=$(((mark_now - _ready_lap) * 1000))
    _PERF_TREE_DEFERRED+=("${mark_depth}:${mark_label}:${mark_ms}${mark_tag:+:${mark_tag}}")
    _ready_lap=$mark_now
}

# Tier 1 runs on first keystroke via zle-line-init. It loads the minimal set for
# basic interactive use: zinit core, compinit, and autosuggestions.
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

    # Accept widgets for zsh-autosuggestions without forward-char and
    # vi-forward-char, so Right Arrow does not accept the suggestion. Set BEFORE
    # the plugin loads so its single bind pass picks this up.
    typeset -ga ZSH_AUTOSUGGEST_ACCEPT_WIDGETS=(
        end-of-line
        vi-end-of-line
        vi-add-eol
    )
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
    # fzf --zsh binds Tab, so bind Tab after sourcing it.
    zle -C _dotfiles_complete_without_list .complete-word _dotfiles_complete_without_list
    zle -N _dotfiles_tab_complete_or_accept
    bindkey -M emacs '^I' _dotfiles_tab_complete_or_accept
    bindkey -M viins '^I' _dotfiles_tab_complete_or_accept
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
    # Complete on an empty command line. zsh inserts a literal tab by default.
    zstyle ":completion:*" insert-tab false
    # Complete IP addresses from known_hosts. zsh drops them by default.
    zstyle ":completion:*:hosts" use-ip true
    if ((! $+functions[_dotfiles_hosts_original])); then
        autoload +X -Uz _hosts
        functions[_dotfiles_hosts_original]=$functions[_hosts]
        functions[_hosts]=$functions[_dotfiles_hosts_or_all]
    fi

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

# Fires after each prompt is drawn, before the next line accepts input.
# _load_tier1 returns immediately after the first load.
function _zinit_line_init() {
    _load_tier1
}
zle -N zle-line-init _zinit_line_init

###############################################################################
