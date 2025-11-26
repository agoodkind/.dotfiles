# shellcheck shell=bash

###############################################################################
###############################################################################    
export START_TIME=$(date +%s%N) 
export DOTDOTFILES="$HOME/.dotfiles"
export PATH="$PATH:$HOME/.local/bin:$HOME/.local/bin/scripts"
export NVM_LAZY_LOAD=true
###############################################################################
# Include OS specific and common zshrc configs ################################
source $DOTDOTFILES/lib/include/.zshrc.incl
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
setopt PROMPT_SUBST

# Prompt Components & Colors
NL=$'\n'
ORANGE='%F{214}'      # Orange
GRAY='%F{250}'    # Light gray
GREEN='%F{green}'    # Green
CYAN='%F{cyan}'      # Cyan
R='%f'                # Reset

CUSTOM_PROMPT=

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

# use system `which` and not built in since system supports -s arg
alias which="$(which -a which | tail -n 1)"
alias isinstalled="which -s"

# vim/nvim editor setup
if isinstalled nvim; then
    export SUDO_EDITOR="nvim -u $HOME/.config/nvim/init.lua"
    export MANPAGER='nvim +Man!'
    export PAGER="$DOTDOTFILES/bin/nvim-pager"
    export MANWIDTH=999
    alias vim="$(command -v nvim)"
elif isinstalled vim; then
    export MANPAGER="vim -M +MANPAGER --not-a-term -"
    export PAGER=$MANPAGER
    export SUDO_EDITOR=vim
    alias nvim="$(command -v vim)"
else
    export SUDO_EDITOR=vi
    alias vim="vi"
    alias nvim="vi"
fi

alias edit="nvim"
alias nano="nvim"
alias emacs="nvim"
export EDITOR="nvim"

# sudo
alias please="sudo"
sudoedit() {
    SUDO_EDITOR="nvim -u $HOME/.config/nvim/init.lua" sudo -e "$@"
}

# clear screen
alias c="clear"

# ls
LS_ARGS="-lah --color=auto -G --group-directories-first"
if isinstalled -s gls; then
    alias ll="$(which gls) $LS_ARGS"
else
    alias ll="$(which ls) $LS_ARGS"
fi
alias ls=ll

# npm
alias npm="pnpm"

# ssh
alias sshrm="ssh-keygen -R" # remove ssh host from known hosts

# Show profiling results if module was loaded
if [[ "${SHOULD_PROFILE:-false}" == "true" ]]; then
    do_profile
fi