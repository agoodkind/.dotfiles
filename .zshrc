#########################################################
# Update this to point to the location of your dotfiles #
export DOTDOTFILES="$HOME/.dotfiles"
#########################################################

#######################################
source $DOTDOTFILES/lib/shared.zsh #
#######################################

################################################
# Add platform-indepedent custom configs below #
alias nano=vim
alias pbcopy="ssh alexs-mba pbcopy"
alias ls=ls -a
################################################