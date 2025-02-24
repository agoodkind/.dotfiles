# import variable for dotfiles path
source ./dotfiles_path.zsh

##############################################
# Platform-independent omz/zsh configuration #
# do not modify this section                 #
if [[ -r "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh" ]]; then
  source "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh"
fi
export ZSH="$DOTDOTFILES/lib/.oh-my-zsh"
ZSH_CUSTOM="$DOTDOTFILES/lib/omz-custom"

##############################################

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
