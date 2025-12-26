# shellcheck shell=bash

########################
# SSH key persistence  #
# Ensure SSH key is loaded in agent after reboot (uses Apple keychain)
# Run in background to not block shell startup
{
    if ! /usr/bin/ssh-add -l 2>/dev/null | grep -q "id_ed25519"; then
        /usr/bin/ssh-add --apple-use-keychain ~/.ssh/id_ed25519
    fi
} >> ~/.cache/ssh-add.log 2>&1 &!
########################

########################
# iterm customizations #
# ITERM2_SQUELCH_MARK=1
test -e "${HOME}/.iterm2_shell_integration.zsh" && source "${HOME}/.iterm2_shell_integration.zsh"

########################

############
# homebrew #
# Detect Homebrew path (Apple Silicon vs Intel)
if [[ -x /opt/homebrew/bin/brew ]]; then
    HOMEBREW_PREFIX="/opt/homebrew"
elif [[ -x /usr/local/bin/brew ]]; then
    HOMEBREW_PREFIX="/usr/local"
else
    HOMEBREW_PREFIX=""
fi

# Cache brew shellenv to avoid slow eval on every startup
if [[ -n "$HOMEBREW_PREFIX" ]]; then
    BREW_BIN="$HOMEBREW_PREFIX/bin/brew"
    if [[ ! -f ~/.cache/brew-shellenv.cache ]] || [[ "$BREW_BIN" -nt ~/.cache/brew-shellenv.cache ]]; then
        mkdir -p ~/.cache
        "$BREW_BIN" shellenv > ~/.cache/brew-shellenv.cache
    fi
    source "$HOME/.cache/brew-shellenv.cache"
fi
############

# Commands also provided by macOS and the commands dir, dircolors, vdir have been installed with the prefix "g".
# If you need to use these commands with their normal names, you can add a "gnubin" directory to your PATH with:
if [[ -n "$HOMEBREW_PREFIX" ]]; then
    export PATH="$HOMEBREW_PREFIX/opt/coreutils/libexec/gnubin:$PATH"
fi

########
# Ruby
# fallback to homebrew
if [[ -n "$HOMEBREW_PREFIX" ]]; then
    export PATH="$HOMEBREW_PREFIX/opt/ruby/bin:$PATH"
fi
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
