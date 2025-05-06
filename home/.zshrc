# shellcheck shell=bash

###########################################
# DO NOT EDIT ############################
# Keep this before common_plugins ########
export DOTDOTFILES="$HOME/.dotfiles"
source $DOTDOTFILES/lib/include/.zshrc.head 
###########################################

######################################
# Theme ############################## 
autoload -U compinit; compinit
autoload -Uz vcs_info
precmd() { vcs_info } 

zstyle ':vcs_info:git:*' formats '%b '
zstyle ':completion:*:default' list-colors ${(s.:.)LS_COLORS}

setopt PROMPT_SUBST
export NEWLINE=$'\n'
PROMPT='%F{cyan}%~%f %F{red}${vcs_info_msg_0_}%f ${NEWLINE}❯ '
RPROMPT="%D{%L:%M:%S}"
######################################  

################################################
# DO NOT EDIT ##################################
# Keep this after common_plugins ###############
source $DOTDOTFILES/lib/include/.zshrc.body ####
################################################

################################################
# Ok to edit ###################################
# Add platform-indepedent custom configs below #
################################################

# autosuggestions bindkey
bindkey '\t' end-of-line

# set editor to vim
export SUDO_EDITOR=vim
export VISUAL=vim
export EDITOR=vim
# enables color in ls
export CLICOLOR=1
eval "$(dircolors -b)"

# aliases
alias nano="vim"
alias ll="ls -lah --color=auto"
alias config="git --git-dir=$DOTDOTFILES/.git --work-tree=$DOTDOTFILES"
alias c="clear"
alias reload="echo 'Reloading zshrc...' && source $HOME/.zshrc && omz reload"  
alias repair="(config pull; cd $DOTDOTFILES && $DOTDOTFILES/repair.sh) && reload"
alias sudoedit="sudo -e"
alias npm="pnpm"
alias sshrm="ssh-keygen -R" # remove ssh host from known hosts