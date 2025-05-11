# shellcheck shell=bash

###########################################
export DOTDOTFILES="$HOME/.dotfiles"
source $DOTDOTFILES/lib/include/.zshrc.head
###########################################

######################################
# Theme ##############################
source $DOTDOTFILES/lib/include/.zshrc.styles

autoload -U compinit
compinit
autoload -Uz vcs_info
precmd() {
    vcs_info
}
#

# Prompt ################################
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
alias c="clear"
alias config="git --git-dir=$DOTDOTFILES/.git --work-tree=$DOTDOTFILES"
alias ll="ls -lah --color=auto"
alias nano="vim"
alias npm="pnpm"
alias please="sudo"
alias reload="echo 'Reloading zshrc...' && source $HOME/.zshrc"
alias repair="(config pull; cd $DOTDOTFILES && $DOTDOTFILES/repair.sh) && reload"
alias sshrm="ssh-keygen -R" # remove ssh host from known hosts
alias sudoedit="sudo -e"

# The following lines were added by compinstall

autoload -Uz compinit
compinit
# End of lines added by compinstall
