###############################################################################
# plugins #####################################################################
###############################################################################

fpath=("$DOTDOTFILES/zshrc/completions" $fpath)

typeset -gA ZINIT
ZINIT[OPTIMIZE_OUT_DISK_ACCESSES]=1

typeset -ga _READY_ORDER=()

function _ready_mark() {
    local mark_label=$1
    local mark_now=$EPOCHREALTIME
    local mark_ms=$(( (mark_now - _ready_lap) * 1000 ))
    _READY_ORDER+=("${mark_label}:${mark_ms}")
    _ready_lap=$mark_now
}

function _load_zinit() {
    (( _ZINIT_LOADED )) && return 0
    _ZINIT_LOADED=1
    local _ready_lap=$EPOCHREALTIME

    source "$DOTDOTFILES/lib/zinit/zinit.zsh"
    ZINIT[AUTO_UPDATE_DAYS]=365
    autoload -Uz _zinit
    (( ${+_comps} )) && _comps[zinit]=_zinit
    _ready_mark zinit_core

    zinit snippet OMZP::git
    zinit light zsh-users/zsh-completions
    _ready_mark git+completions

    ZINIT[COMPINIT_OPTS]=-C
    zicompinit
    zicdreplay
    _ready_mark compinit

    zinit light zsh-users/zsh-autosuggestions
    _zsh_autosuggest_start
    _ready_mark autosuggestions

    zinit light zdharma-continuum/fast-syntax-highlighting
    _ready_mark syntax-hl

    zinit light Aloxaf/fzf-tab
    zinit light Freed-Wu/fzf-tab-source
    source /opt/homebrew/opt/fzf/shell/key-bindings.zsh 2>/dev/null
    _ready_mark fzf-tab

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

    local _zc_dir="${ZINIT[COMPLETIONS_DIR]:-$HOME/.local/share/zinit/completions}"
    for _zc_link in "$_zc_dir"/*(N@); do
        [[ -e "$_zc_link" ]] || rm -f "$_zc_link"
    done
    unset _zc_dir _zc_link
    _ready_mark zstyle+cleanup

    local _iterm_si="/Applications/iTerm.app/Contents/Resources/iterm2_shell_integration.zsh"
    if [[ "$TERM_PROGRAM" == "iTerm.app" && -f "$_iterm_si" ]]; then
        source "$_iterm_si"
        # iterm2_preexec restores PS1 from ITERM2_PRECMD_PS1. Since we source
        # the integration inside zle-line-init (before iterm2_precmd ever runs),
        # that variable is empty, which blanks the prompt on the first command.
        ITERM2_PRECMD_PS1="$PS1"
    fi
    _ready_mark iterm2_si

    _PROFILE_TIMES[_time_to_ready]=$(( (EPOCHREALTIME - START_TIME) * 1000 ))

    _write_startup_log
    _ready_mark startup_log
}

# Fires once after prompt is drawn, before first keystroke is accepted.
# Loads all plugins synchronously, then unregisters itself.
function _zinit_line_init() {
    _load_zinit
    zle -D zle-line-init 2>/dev/null
}
zle -N zle-line-init _zinit_line_init

###############################################################################
