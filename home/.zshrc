# shellcheck shell=bash

###############################################################################
###############################################################################
zmodload zsh/datetime
START_TIME=$EPOCHREALTIME
export DOTDOTFILES="$HOME/.dotfiles"
export PATH="$PATH:$HOME/.local/bin:$HOME/.local/bin/scripts:/opt/scripts"
export NVM_LAZY_LOAD=true
###############################################################################
# Include OS specific and common zshrc configs ################################
source $DOTDOTFILES/lib/zsh/incl.zsh
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
export HISTSIZE=100000
export SAVEHIST=100000
setopt appendhistory
setopt incappendhistory
setopt share_history

###############################################################################
# Aliases #####################################################################
###############################################################################

# Editor preference: nvim > vim > vi
EDITOR_BIN=
if isinstalled nvim; then
    EDITOR_BIN=nvim
    export SUDO_EDITOR="nvim -u $HOME/.config/nvim/init.lua"
    export MANPAGER='nvim +Man!'
    export PAGER="$DOTDOTFILES/bin/nvim-pager"
    export MANWIDTH=999
    prefer vim nvim
elif isinstalled vim; then
    EDITOR_BIN=vim
    export MANPAGER="vim -M +MANPAGER --not-a-term -"
    export PAGER=$MANPAGER
    export SUDO_EDITOR=vim
    prefer nvim vim
else
    EDITOR_BIN=vi
    export SUDO_EDITOR=vi
fi

# Fallback if nothing above matched
[[ -z "$EDITOR_BIN" ]] && EDITOR_BIN=vi
export EDITOR="$EDITOR_BIN"

edit() { command "$EDITOR_BIN" "$@"; }
nano() { edit "$@"; }
emacs() { edit "$@"; }

# sudo
please() { command sudo "$@"; }
sudoedit() {
    SUDO_EDITOR="nvim -u $HOME/.config/nvim/init.lua" sudo -e "$@"
}

# clear screen
c() { command clear; }

# Prefer enhanced replacements when the binaries exist
prefer ls eza --icons  --group-directories-first
prefer ll eza -lah --icons --group-directories-first
prefer la eza -a --icons --group-directories-first
prefer lt eza --tree --level=2 --icons
prefer llt eza -lah --tree --level=2 --icons
ls() { ll "$@"; }

# cat / find / grep
prefer_tty cat bat --style=auto --paging=never
prefer catt bat --style=auto
prefer find fd
prefer grep rg
prefer rgi rg -i
prefer rgl rg -l

# disk + process tools
prefer du dust
prefer df duf
prefer ps procs
prefer top btop
prefer htop btop

# helper CLIs
prefer help tldr
prefer dig doggo
prefer curl curlie
prefer lg lazygit

# npm wrapper prefers pnpm implementation
npm() { command pnpm "$@"; }

# ssh helper
sshrm() { command ssh-keygen -R "$@"; }

# Show profiling results if module was loaded
if [[ "${SHOULD_PROFILE:-false}" == "true" ]]; then
    do_profile
fi
