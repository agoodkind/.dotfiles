# shellcheck shell=bash

###########################################
export DOTDOTFILES="$HOME/.dotfiles"
###########################################

export PATH="$PATH:$HOME/.local/bin"
export NVM_LAZY_LOAD=true

################################################
# DO NOT EDIT ##################################
source $DOTDOTFILES/lib/include/.zshrc.incl ####
################################################

######################################
# Theme ##############################

# enables color in ls
export CLICOLOR=1
# dircolors just prints LS_COLORS
eval "$(dircolors -b)"

########################################
# plugins ##############################
########################################

zi snippet OMZP::git
zi snippet OMZP::dotenv

# Completions
autoload -U compinit
compinit

# fzf-tab: replace zsh's default completion selection menu with fzf
zinit light Aloxaf/fzf-tab

zstyle ':fzf-tab:complete:cd:*' fzf-preview 'eza -1 --color=always $realpath'
zstyle ':fzf-tab:complete:git-(add|diff|restore):*' fzf-preview 'git diff $word | delta'
zstyle ':fzf-tab:complete:git-log:*' fzf-preview 'git log --color=always $word'
zstyle ':fzf-tab:complete:git-help:*' fzf-preview 'git help $word | bat -plman --color=always'
zstyle ':fzf-tab:complete:git-show:*' fzf-preview 'case "$group" in "commit tag") git show --color=always $word ;; *) git show --color=always $word | delta ;; esac'
zstyle ':fzf-tab:complete:git-checkout:*' fzf-preview 'case "$group" in "modified file") git diff $word | delta ;; "recent commit object name") git show --color=always $word | delta ;; *) git log --color=always $word ;; esac'
zstyle ':completion:*:git-checkout:*' sort false
zstyle ':completion:*:descriptions' format '[%d]'
zstyle ':completion:*' list-colors ${(s.:.)LS_COLORS}
zstyle ':completion:*' menu no
zstyle ':fzf-tab:complete:cd:*' fzf-preview 'eza -1 --color=always $realpath'
zstyle ':fzf-tab:*' switch-group '<' '>'
zstyle ':completion:*' list-max-items 20

zinit light Freed-Wu/fzf-tab-source

# Syntax highlighting with fast initialization
zinit ice wait lucid atinit"ZINIT[COMPINIT_OPTS]=-C; zicompinit; zicdreplay"
zinit light zdharma-continuum/fast-syntax-highlighting

# Additional completions with blockf to prevent clashes
zinit ice wait lucid blockf
zinit light zsh-users/zsh-completions

# Auto-suggestions with proper initialization
zinit ice wait lucid atload"!_zsh_autosuggest_start"
zinit light zsh-users/zsh-autosuggestions

# Prompt ###############################
setopt PROMPT_SUBST
export NEWLINE=$'\n'
PROMPT='%F{cyan}%~%f %F{red}${vcs_info_msg_0_}%f ${NEWLINE}❯ '
RPROMPT="%D{%L:%M:%S}"
######################################

########################################
# Configs 
if command -v nvim &> /dev/null; then
    export SUDO_EDITOR=nvim
    alias vim="$(command -v nvim)"
else
    export SUDO_EDITOR=vim
    alias nvim="$(command -v vim)"
fi

export HISTFILE=~/.zsh_history
export HISTSIZE=100000
export SAVEHIST=100000

########################################
# Aliases ##############################
########################################

# sudo wrapper to run commands with current user
alias please="sudo"
alias sudoedit="sudo -e"

# clear screen
alias c="clear"

# ls
alias ll="ls -lah --color=auto"

# vim
alias nano="nvim"
alias emacs="nvim"

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

