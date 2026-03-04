###############################################################################
# plugins #####################################################################
###############################################################################

fpath=("$DOTDOTFILES/zshrc/completions" $fpath)

typeset -gA ZINIT
ZINIT[OPTIMIZE_OUT_DISK_ACCESSES]=1

function _ready_mark() {
    local mark_depth=$1 mark_label=$2 mark_tag=${3:-}
    local mark_now=$EPOCHREALTIME
    local mark_ms=$(( (mark_now - _ready_lap) * 1000 ))
    _PERF_TREE+=("${mark_depth}:${mark_label}:${mark_ms}${mark_tag:+:${mark_tag}}")
    _ready_lap=$mark_now
}

# Tier 1: runs on first keystroke (zle-line-init). Minimal set needed for
# basic interactive use — zinit core, compinit, and autosuggestions.
function _load_tier1() {
    (( _ZINIT_LOADED )) && return 0
    _ZINIT_LOADED=1
    typeset -g _ready_lap=$EPOCHREALTIME

    _PERF_TREE+=("1:deferred:0")
    _ready_mark 2 "tier 1" "zle-line-init"

    source "$DOTDOTFILES/lib/zinit/zinit.zsh"
    ZINIT[AUTO_UPDATE_DAYS]=365
    autoload -Uz _zinit
    (( ${+_comps} )) && _comps[zinit]=_zinit
    _ready_mark 3 zinit_core

    ZINIT[COMPINIT_OPTS]=-C
    zicompinit
    zicdreplay
    _ready_mark 3 compinit

    zinit light zsh-users/zsh-autosuggestions
    _zsh_autosuggest_start
    _ready_mark 3 autosuggestions

    _PROFILE_TIMES[_time_to_ready_t1]=$(( (EPOCHREALTIME - START_TIME) * 1000 ))
    sched +0 _load_tier2
}

# Tier 2: runs at next event loop pass after tier 1. Syntax highlighting,
# fzf-tab, iterm2 integration — things you won't need in the first ~100ms.
function _load_tier2() {
    _ready_mark 2 "tier 2" "sched +0"

    zinit light zdharma-continuum/fast-syntax-highlighting
    _ready_mark 3 syntax-hl

    zinit light Aloxaf/fzf-tab
    zinit light Freed-Wu/fzf-tab-source
    source /opt/homebrew/opt/fzf/shell/key-bindings.zsh 2>/dev/null
    _ready_mark 3 fzf-tab

    # shellcheck disable=SC2016
    zstyle ":fzf-tab:complete:cd:*" fzf-preview "eza -1 --color=always \$realpath"
    zstyle ":fzf-tab:complete:git-(add|diff|restore):*" fzf-preview "git diff \$word | delta"
    zstyle ":fzf-tab:complete:curl:*" fzf-preview "echo \$desc"
    zstyle ":fzf-tab:complete:*:options" fzf-preview "echo \$desc"
    zstyle ":fzf-tab:complete:*:argument-rest" fzf-preview
    zstyle ":completion:*" list-colors ${(s.:.)LS_COLORS}
    zstyle ":completion:*" menu no

    if (( $+commands[gcp] )); then
        unalias gcp 2>/dev/null
        compdef -d gcp 2>/dev/null
        compdef _files gcp 2>/dev/null
    fi

    local _iterm_si="/Applications/iTerm.app/Contents/Resources/iterm2_shell_integration.zsh"
    if [[ "$TERM_PROGRAM" == "iTerm.app" && -f "$_iterm_si" ]]; then
        source "$_iterm_si"
        ITERM2_PRECMD_PS1="$PS1"
    fi
    _ready_mark 3 iterm2_si

    sched +0 _load_tier3
}

# Tier 3: extra completions and cleanup. Runs after tier 2.
function _load_tier3() {
    _ready_mark 2 "tier 3" "sched +0"

    zinit light zsh-users/zsh-completions
    _ready_mark 3 completions

    local _zc_dir="${ZINIT[COMPLETIONS_DIR]:-$HOME/.local/share/zinit/completions}"
    for _zc_link in "$_zc_dir"/*(N@); do
        [[ -e "$_zc_link" ]] || rm -f "$_zc_link"
    done
    unset _zc_dir _zc_link
    _ready_mark 3 cleanup

    _PROFILE_TIMES[_time_to_ready]=$(( (EPOCHREALTIME - START_TIME) * 1000 ))

    _write_startup_log
    _ready_mark 3 startup_log
}

# Fires once after prompt is drawn, before first keystroke is accepted.
function _zinit_line_init() {
    _load_tier1
    zle -D zle-line-init 2>/dev/null
}
zle -N zle-line-init _zinit_line_init

###############################################################################
