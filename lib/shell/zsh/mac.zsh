# shellcheck shell=bash

########################
# SSH key persistence  #
# Ensure SSH key is loaded in agent after reboot (uses Apple keychain)
# Run in background to not block shell startup
_mac_load_ssh_key() {
    if ! /usr/bin/ssh-add -l 2>/dev/null | grep -q "id_ed25519"; then
        /usr/bin/ssh-add --apple-use-keychain ~/.ssh/id_ed25519
    fi
} >> ~/.cache/ssh-add.log 2>&1
async_run _mac_load_ssh_key
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
# nvm (lazy-loaded, no completions)
export NVM_DIR="$HOME/.nvm"
nvm() {
  unset -f nvm
  [ -s "/opt/homebrew/opt/nvm/nvm.sh" ] && \. "/opt/homebrew/opt/nvm/nvm.sh"
  nvm "$@"
}
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

flush_dns() {
    echo "Flushing DNS..."
    sudo dscacheutil -flushcache
    sudo killall -HUP mDNSResponder
}

change_hostname() {
    if [[ -z "$1" ]]; then
        echo "Usage: change_hostname <new_name>"
        return 1
    fi

    local new_name="$1"
    local sanitized_name="$new_name"
    
    # Capture old names
    local old_computer=$(sudo scutil --get ComputerName 2>/dev/null)
    local old_local=$(sudo scutil --get LocalHostName 2>/dev/null)
    local old_host=$(sudo scutil --get HostName 2>/dev/null)

    # Sanitize for LocalHostName/HostName if needed
    if [[ "$new_name" =~ [^a-zA-Z0-9-] ]]; then
        # Remove apostrophes and other special chars, replace spaces with -
        sanitized_name=$(echo "$new_name" | tr -d "'" | tr ' ' '-' | \
            tr -cd 'a-zA-Z0-9-')
        echo "ℹ️  Sanitized hostname: '$sanitized_name'"
    fi

    # ComputerName accepts spaces and special chars
    sudo scutil --set ComputerName "$new_name"
    
    # LocalHostName and HostName need sanitized input
    sudo scutil --set LocalHostName "$sanitized_name"
    sudo scutil --set HostName "$sanitized_name"

    # Flush DNS cache
    flush_dns

    echo "✅ ComputerName: '$old_computer' -> '$new_name'"
    echo "✅ LocalHostName: '$old_local' -> '$sanitized_name'"
    echo "✅ HostName: '$old_host' -> '$sanitized_name'" 
}
