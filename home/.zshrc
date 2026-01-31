# shellcheck shell=bash

###############################################################################
###############################################################################
zmodload zsh/datetime
START_TIME=$EPOCHREALTIME
export DOTDOTFILES="$HOME/.dotfiles"
###############################################################################
# Include OS specific and common zshrc configs ################################
source $DOTDOTFILES/lib/shell/zsh/incl.zsh
###############################################################################

###############################################################################
# Theme #######################################################################
###############################################################################

# enables color in ls
export CLICOLOR=1

# Cache dircolors
if [[ ! -f ~/.cache/dircolors.cache ]] || [[ ~/.dir_colors -nt ~/.cache/dircolors.cache ]]; then
    mkdir -p ~/.cache
    dircolors -b > ~/.cache/dircolors.cache
fi
source ~/.cache/dircolors.cache

###############################################################################
# Prompt ######################################################################
###############################################################################
setopt PROMPT_SUBST

# Prompt Components & Colors
NL=$'\n'
ORANGE='%F{214}'
GRAY='%F{250}'
GREEN='%F{green}'
CYAN='%F{cyan}'
R='%f'

# Build Prompt with iTerm2 integration
if [[ -n "$ITERM_SESSION_ID" && -n "$(iterm2_prompt_mark &> /dev/null)" ]]; then
    # iTerm2 integration - include prompt mark for shell integration features
    PS1='%{$(iterm2_prompt_mark)%}${GREEN}%m${R} ${CYAN}%~${R} ❯ '
else
    # Standard prompt without iTerm2
    PROMPT='${ORANGE}%D{%H:%M:%S}${R}${GRAY}.%D{%.}${R} ${GREEN}%m${R} ${CYAN}%~${R} ${NL}❯ '
fi
###############################################################################

###############################################################################
# zsh Configs #################################################################
###############################################################################
export HISTFILE=~/.zsh_history
export HISTSIZE=9223372036854775807  # 2^63 - 1 (max signed 64-bit)
export SAVEHIST=9223372036854775807
setopt appendhistory
setopt incappendhistory
setopt share_history
setopt interactive_comments
setopt auto_cd 
setopt auto_pushd
setopt pushd_ignore_dups 
setopt pushd_silent

###############################################################################
# Aliases #####################################################################
###############################################################################

# Prefer enhanced replacements when the binaries exist

prefer ll eza -lah --icons --group-directories-first
prefer la eza -a --icons --group-directories-first
prefer lt eza --tree --level=2 --icons
prefer llt eza -lah --tree --level=2 --icons
prefer_tty ls ll

# cat / find / grep
prefer catt bat --style=auto
prefer rgi rg -i
prefer rgl rg -l

# disk + process tools
prefer top btop
prefer htop btop

# helper CLIs
prefer help tldr
prefer lg lazygit

# npm wrapper prefers pnpm implementation
# npm() { command pnpm "$@"; }

# ssh helper
sshrm() { command ssh-keygen -R "$@"; }

prefer docker podman

# Editor preference: nvim > vim > vi
# This logic must come AFTER all other `prefer` calls and alias definitions
# to avoid conflicts (since we are aliasing vim/nvim below)
EDITOR_BIN=
if isinstalled nvim; then
    EDITOR_BIN=nvim
    export SUDO_EDITOR="nvim -u $HOME/.config/nvim/init.lua"
    export MANPAGER='nvim +Man!'
    export PAGER="$DOTDOTFILES/bin/nvim-pager"
    export MANWIDTH=999
elif isinstalled vim; then
    EDITOR_BIN=vim
    export MANPAGER="vim -M +MANPAGER --not-a-term -"
    export PAGER=$MANPAGER
    export SUDO_EDITOR=vim
else
    EDITOR_BIN=vi
    export SUDO_EDITOR=vi
fi

# Fallback if nothing above matched
[[ -z "$EDITOR_BIN" ]] && EDITOR_BIN=vi
export EDITOR="$EDITOR_BIN"

# Define wrappers using `function` keyword to override any potential aliases
# or ensure they take precedence
function vim() { _edit_maybe_sudoedit "$EDITOR_BIN" "$@"; }
function vi() { _edit_maybe_sudoedit "$EDITOR_BIN" "$@"; }
function nvim() { _edit_maybe_sudoedit nvim "$@"; }

edit() { "$EDITOR_BIN" "$@"; }
nano() { edit "$@"; }
emacs() { edit "$@"; }

# Show profiling results if module was loaded
if [[ "${SHOULD_PROFILE:-false}" == "true" ]]; then
    do_profile
fi
