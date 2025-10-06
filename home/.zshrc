# shellcheck shell=bash

################################################
# DO NOT EDIT ##################################
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

# Use turbo mode for all plugins
zinit wait lucid for \
    OMZP::git \
    OMZP::dotenv

# Additional completions
zinit wait lucid blockf atinit'fpath=("$DOTDOTFILES/lib/completions" $fpath)' for \
    zsh-users/zsh-completions

# Local completion snippets
zinit wait lucid as"completion" id-as"_curl" for \
    "$DOTDOTFILES/lib/_curl"

# fzf-tab with slight delay
zinit wait'1' lucid for \
    Aloxaf/fzf-tab

# Defer zstyle configuration
zinit wait'1' lucid atload'
    zstyle ":fzf-tab:complete:cd:*" fzf-preview "eza -1 --color=always \$realpath"
    zstyle ":fzf-tab:complete:git-(add|diff|restore):*" fzf-preview "git diff \$word | delta"
    zstyle ":completion:*" list-colors ${(s.:.)LS_COLORS}
    zstyle ":completion:*" menu no
' for \
    Freed-Wu/fzf-tab-source

# Syntax highlighting with compinit
zinit wait lucid atinit"ZINIT[COMPINIT_OPTS]=-C; zicompinit; zicdreplay" for \
    zdharma-continuum/fast-syntax-highlighting

# Auto-suggestions
zinit wait lucid atload"_zsh_autosuggest_start" for \
    zsh-users/zsh-autosuggestions

########################################
# Prompt ###############################
setopt PROMPT_SUBST
export NEWLINE=$'\n'
PROMPT='%F{cyan}%~%f %F{red}${vcs_info_msg_0_}%f ${NEWLINE}❯ '
RPROMPT="%D{%L:%M:%S}"
######################################

########################################
# zsh Configs 
########################################
export HISTFILE=~/.zsh_history
export HISTSIZE=100000
export SAVEHIST=100000

########################################
# Aliases ##############################
########################################
# use system which and not built in since system supports -s arg
alias which="$(which -a which | tail -n 1)"
alias isinstalled="which -s"

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
alias ll="$(which ls) -lah --color=auto"
alias ls=ll

# npm
alias npm="pnpm"

# dotfile management
alias config="git --git-dir=$DOTDOTFILES/.git --work-tree=$DOTDOTFILES"
alias reload="echo 'Reloading zshrc...' && source $HOME/.zshrc"
alias repair="(config pull; cd $DOTDOTFILES && $DOTDOTFILES/repair.sh) && reload"

# ssh
alias sshrm="ssh-keygen -R" # remove ssh host from known hosts

### Added by Zinit's installer
if [[ ! -f $HOME/.local/share/zinit/zinit.git/zinit.zsh ]]; then
    print -P "%F{33} %F{220}Installing %F{33}ZDHARMA-CONTINUUM%F{220} Initiative Plugin Manager (%F{33}zdharma-continuum/zinit%F{220})…%f"
    command mkdir -p "$HOME/.local/share/zinit" && command chmod g-rwX "$HOME/.local/share/zinit"
    command git clone https://github.com/zdharma-continuum/zinit "$HOME/.local/share/zinit/zinit.git" && \
        print -P "%F{33} %F{34}Installation successful.%f%b" || \
        print -P "%F{160} The clone has failed.%f%b"
fi

source "$HOME/.local/share/zinit/zinit.git/zinit.zsh"
autoload -Uz _zinit
(( ${+_comps} )) && _comps[zinit]=_zinit
### End of Zinit's installer chunk

# Show profiling results if module was loaded
$SHOULD_PROFILE && do_profile