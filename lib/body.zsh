
#########################################################
# Determine OS and load platform-specific configuration #
if [[ $(uname) == "Darwin" ]]; then
    source $DOTDOTFILES/os/mac.zsh
elif command -v apt > /dev/null; then
    source $DOTDOTFILES/os/debian.zsh
else
    echo 'Unknown OS!'
fi

plugins+=( "${common_plugins[@]}" )
source $ZSH/oh-my-zsh.sh
[[ ! -f "$DOTDOTFILES/lib/.p10k.zsh" ]] || source "$DOTDOTFILES/lib/.p10k.zsh"
