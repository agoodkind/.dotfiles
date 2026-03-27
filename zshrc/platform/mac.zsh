# shellcheck shell=bash

########################
# iterm customizations #
# ITERM2_SQUELCH_MARK=1
test -e "${HOME}/.iterm2_shell_integration.zsh" && _source "${HOME}/.iterm2_shell_integration.zsh"

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

# Set Homebrew environment directly (avoids slow path_helper subprocess from brew shellenv)
if [[ -n "$HOMEBREW_PREFIX" ]]; then
    export HOMEBREW_CELLAR="$HOMEBREW_PREFIX/Cellar"
    export HOMEBREW_REPOSITORY="$HOMEBREW_PREFIX"
    fpath[1,0]="$HOMEBREW_PREFIX/share/zsh/site-functions"
    export PATH="$HOMEBREW_PREFIX/bin:$HOMEBREW_PREFIX/sbin:$PATH"
    [[ -z "${MANPATH-}" ]] || export MANPATH=":${MANPATH#:}"
    export INFOPATH="$HOMEBREW_PREFIX/share/info:${INFOPATH:-}"
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

function rbenv() {
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
function nvm() {
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

function flush_dns() {
    echo "Flushing DNS..."
    sudo dscacheutil -flushcache
    sudo killall -HUP mDNSResponder
}

function set-hostname() {
    change_hostname "$@"
}

function change_hostname() {
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
