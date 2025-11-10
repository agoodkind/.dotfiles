# shellcheck shell=bash

################################################
# DO NOT EDIT ##################################
# use system `which` and not built in since system supports -s arg
alias which="$(which -a which | tail -n 1)"
alias isinstalled="which -s"
########################################
export DOTDOTFILES="$HOME/.dotfiles"
########################################
export PATH="$PATH:$HOME/.local/bin"
export NVM_LAZY_LOAD=true
################################################
# Include OS specific and common zshrc configs
source $DOTDOTFILES/lib/include/.zshrc.incl ####
################################################

######################################
# Theme ##############################

# enables color in ls
export CLICOLOR=1

# Cache dircolors
if [[ ! -f ~/.cache/dircolors.cache ]] || [[ ~/.dir_colors -nt ~/.cache/dircolors.cache ]]; then
    mkdir -p ~/.cache
    dircolors -b > ~/.cache/dircolors.cache
fi
source ~/.cache/dircolors.cache

########################################
# plugins ##############################
########################################

# Core plugins - load immediately with turbo
zinit wait lucid for \
    OMZP::git

# Completions with custom fpath
zinit wait lucid blockf for \
    zsh-users/zsh-completions

# Auto-suggestions
zinit wait lucid atload"_zsh_autosuggest_start" for \
    zsh-users/zsh-autosuggestions

# Local completion files
fpath=("$DOTDOTFILES/lib/completions" $fpath)

# Syntax highlighting with deferred compinit
zinit wait'1' lucid atinit"ZINIT[COMPINIT_OPTS]=-C; zicompinit; zicdreplay" for \
    zdharma-continuum/fast-syntax-highlighting

# fzf-tab - defer to reduce startup impact
zinit wait'2' lucid for \
    Aloxaf/fzf-tab

# fzf-tab configuration - load last
zinit wait'2' lucid atload'
    zstyle ":fzf-tab:complete:cd:*" fzf-preview "eza -1 --color=always \$realpath"
    zstyle ":fzf-tab:complete:git-(add|diff|restore):*" fzf-preview "git diff \$word | delta"
    zstyle ":completion:*" list-colors ${(s.:.)LS_COLORS}
    zstyle ":completion:*" menu no
' for \
    Freed-Wu/fzf-tab-source

########################################
# Prompt ###############################
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
########################################

########################################
# zsh Configs 
########################################
export HISTFILE=~/.zsh_history
export HISTSIZE=100000
export SAVEHIST=100000
setopt appendhistory
setopt incappendhistory
setopt share_history

########################################
# Aliases ##############################
########################################

# Use bat if installed
# if isinstalled bat; then
#     alias cat="bat"
# fi

# vim
if isinstalled nvim; then
    export SUDO_EDITOR=nvim
    alias vim="$(command -v nvim)"
elif isinstalled vim; then
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

# use most as pager if installed
if isinstalled -s most; then
    export PAGER=most
fi

# sudo wrapper to run commands with current user
alias please="sudo"
alias sudoedit="sudo -e"

# clear screen
alias c="clear"

# ls
if isinstalled -s gls; then
    alias ll="$(which gls) -lah --color=auto --group-directories-first"
else
    alias ll="$(which ls) -lah"
fi
alias ls=ll

# npm
alias npm="pnpm"

# dotfile management
alias config="git --git-dir=$DOTDOTFILES/.git --work-tree=$DOTDOTFILES"
alias reload="echo 'Reloading zshrc...' && source $HOME/.zshrc"
alias repair="(config pull; cd $DOTDOTFILES && $DOTDOTFILES/repair.sh) && git pull && git submodule update --init --recursive --remote && reload"

# ssh
alias sshrm="ssh-keygen -R" # remove ssh host from known hosts

# Show profiling results if module was loaded
$SHOULD_PROFILE && do_profile