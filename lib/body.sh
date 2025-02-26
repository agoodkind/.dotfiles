#!/usr/bin/env bash

#########################################################
# Determine OS and load platform-specific configuration #
if [[ $(uname) == "Darwin" ]]; then
    source "$DOTDOTFILES/os/mac.zsh"
elif command -v apt > /dev/null; then
    source "$DOTDOTFILES/os/debian.zsh"
else
    echo 'Unknown OS!'
fi

plugins+=( "${common_plugins[@]}" )

source "$ZSH"/oh-my-zsh.sh

add_plugin() {
    PLUGIN_GIT_URL="$1"

    if [ -z "$2" ] ; then
        TEMP_NAME="$(basename "${PLUGIN_GIT_URL##*:}")"
        PLUGIN_FOLDER_NAME="${TEMP_NAME%%.git}"
    else
        PLUGIN_FOLDER_NAME="$2"
    fi

    git submodule add "$PLUGIN_GIT_URL" "lib/omz-custom/plugins/$PLUGIN_FOLDER_NAME"
}

add_theme() {
    THEME_GIT_URL="$1"
    TEMP_NAME="$(basename "${THEME_GIT_URL##*:}")"
    THEME_FOLDER_NAME="${TEMP_NAME%%.git}"
    git submodule add "$THEME_GIT_URL" "lib/omz-custom/themes/$THEME_FOLDER_NAME"
}

[[ ! -f "$DOTDOTFILES/.zshrc.local" ]] ||  source "$DOTDOTFILES/.zshrc.local" ###

# To customize prompt, run `p10k configure` or edit ~/.p10k.zsh.
[[ ! -f ~/.p10k.zsh ]] || source ~/.p10k.zsh
