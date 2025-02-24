####################################
# DO NOT EDIT ######################
# Keep this before common_plugins ##
export DOTDOTFILES="/home/agoodkind/.dotfiles"
source $DOTDOTFILES/lib/head.zsh   #
####################################

ZSH_THEME="powerlevel10k/powerlevel10k"
common_plugins=(colored-man-pages zsh-navigation-tools git fast-syntax-highlighting zsh-autosuggestions zsh-autocomplete)

####################################
# DO NOT EDIT ######################
# Keep this after common_plugins ###
source $DOTDOTFILES/lib/body.zsh   #
####################################

################################################
# Add platform-indepedent custom configs below #
alias nano=vim
alias ls=ll

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