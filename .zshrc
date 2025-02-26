####################################
# DO NOT EDIT ######################
# Keep this before common_plugins ##
export DOTDOTFILES="$(dirname "$(readlink -f $HOME/.zshrc)")"
source $DOTDOTFILES/lib/head.zsh   #
####################################

####################################
# Ok to edit #######################
ZSH_THEME="powerlevel10k/powerlevel10k"
common_plugins=(colored-man-pages zsh-navigation-tools git fast-syntax-highlighting zsh-autosuggestions zsh-autocomplete)
####################################

####################################
# DO NOT EDIT ######################
# Keep this after common_plugins ###
source $DOTDOTFILES/lib/body.zsh   #
source $DOTDOTFILES/.zshrc.local   #
####################################

################################################
# Ok to edit ###################################
# Add platform-indepedent custom configs below #
################################################

# enables color in ls
export CLICOLOR=1

# aliases
alias nano=vim
alias ll=ls -la --color=auto
alias config="git --git-dir=$DOTDOTFILES/.git --work-tree=$DOTDOTFILES"
alias reload="source $HOME/.zshrc"
alias c="clear"
add_plugin() {
    PLUGIN_GIT_URL="$1"

    if [ -z $2 ] ; then
        PLUGIN_FOLDER_NAME="${$(basename "${PLUGIN_GIT_URL##*:}")%%.git}"
    else
        PLUGIN_FOLDER_NAME="$2"
    fi

    git submodule add $PLUGIN_GIT_URL ./lib/omz-custom/plugins/$PLUGIN_FOLDER_NAME
}
################################################