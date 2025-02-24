
#########################################################
# Determine OS and load platform-specific configuration #
if [[ $(uname) == "Darwin" ]]; then
    source $DOTDOTFILES/os/mac.zsh
# elif command -v freebsd-version > /dev/null; then
#     source "$ZSH_CUSTOM"/os/freebsd.zsh
elif command -v apt > /dev/null; then
    source $DOTDOTFILES/os/debian.zsh
else
    echo 'Unknown OS!'
fi
# Do we have systemd on board?
# if command -v systemctl > /dev/null; then
#     source "$ZSH_CUSTOM"/os/systemd.zsh
# fi
# # Ditto Kubernetes?
# if command -v kubectl > /dev/null; then
#     source "$ZSH_CUSTOM"/os/kubernetes.zsh
# fi

plugins+=( "${common_plugins[@]}" )
source $ZSH/oh-my-zsh.sh
[[ ! -f "$DOTDOTFILES/lib/.p10k.zsh" ]] || source "$DOTDOTFILES/lib/.p10k.zsh"
