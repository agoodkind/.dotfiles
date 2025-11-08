# shellcheck shell=bash

########################
# iterm customizations #
# ITERM2_SQUELCH_MARK=1
test -e "${HOME}/.iterm2_shell_integration.zsh" && source "${HOME}/.iterm2_shell_integration.zsh"

########################

############
# homebrew #
# Cache brew shellenv to avoid slow eval on every startup
if [[ ! -f ~/.cache/brew-shellenv.cache ]] || [[ /opt/homebrew/bin/brew -nt ~/.cache/brew-shellenv.cache ]]; then
    mkdir -p ~/.cache
    /opt/homebrew/bin/brew shellenv > ~/.cache/brew-shellenv.cache
fi
source "$HOME/.cache/brew-shellenv.cache"
############

# export PATH="/opt/homebrew/bin:$PATH"
# Commands also provided by macOS and the commands dir, dircolors, vdir have been installed with the prefix "g".
# If you need to use these commands with their normal names, you can add a "gnubin" directory to your PATH with:
export PATH="/opt/homebrew/opt/coreutils/libexec/gnubin:$PATH"

########
# Ruby
# fallback to homebrew
export PATH="/opt/homebrew/opt/ruby/bin:$PATH"
# use rbenv if present
export PATH="${HOME}/.rbenv/shims:${PATH}"
export RBENV_SHELL=zsh

rbenv() {
  local command
  command="${1:-}"
  if [ "$#" -gt 0 ]; then
    shift
  fi

  case "$command" in
  rehash|shell)
    eval "$(rbenv "sh-$command" "$@")";;
  *)
    command rbenv "$command" "$@";;
  esac
}


#######
# nvm #
export NVM_DIR=~/.nvm
#######

########
# pnpm #
export PNPM_HOME="$HOME/Library/pnpm"
case ":$PATH:" in
*":$PNPM_HOME:"*) ;;
*) export PATH="$PNPM_HOME:$PATH" ;;
esac
#######

alias dircolors="gdircolors"
