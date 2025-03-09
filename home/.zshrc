# shellcheck shell=bash

###########################################
# DO NOT EDIT ############################
# Keep this before common_plugins ########
export DOTDOTFILES="$HOME/.dotfiles"
source $DOTDOTFILES/lib/include/.zshrc.head 
###########################################

######################################
# Ok to edit #########################  
ZSH_THEME="powerlevel10k/powerlevel10k"
common_plugins=(
    colored-man-pages
    zsh-navigation-tools
    git
    fast-syntax-highlighting
    zsh-autosuggestions 
    zsh-autocomplete
)
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

# set editor to vim
export EDITOR=vim
# enables color in ls
export CLICOLOR=1
eval "$(dircolors -b)"
zstyle ':completion:*:default' list-colors ${(s.:.)LS_COLORS}

# aliases
alias nano="vim"
alias ll="ls -lah --color=auto"
alias config="git --git-dir=$DOTDOTFILES/.git --work-tree=$DOTDOTFILES"
alias reload="echo 'Reloading zshrc...' && source $HOME/.zshrc"
alias c="clear"
alias repair="(config pull && cd $DOTDOTFILES && $DOTDOTFILES/repair.sh) && reload"
alias sudoedit="sudo -e"