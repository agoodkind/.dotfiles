ZSH_THEME="powerlevel10k/powerlevel10k"
# git status sits and polls your git config for changes, which can be slow
POWERLEVEL9K_DISABLE_GITSTATUS=true
# this is the list of plugins that are common to all platforms
common_plugins=(colored-man-pages zsh-navigation-tools git zsh-syntax-highlighting zsh-autosuggestions)

###########DO NOT EDIT##############
source $DOTDOTFILES/lib/shared.zsh #
####################################

################################################
# Add platform-indepedent custom configs below #
alias nano=vim
alias pbcopy="ssh alexs-mba pbcopy"
alias ls=ll
################################################