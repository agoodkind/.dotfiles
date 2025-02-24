##############################################
# Platform-independent omz/zsh configuration #
# do not modify this section                 #
if [[ -r "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh" ]]; then
  source "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh"
fi
export ZSH="$DOTDOTFILES/lib/.oh-my-zsh"
ZSH_CUSTOM="$DOTDOTFILES/lib/omz-custom"