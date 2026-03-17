#!/usr/bin/env bash
[[ "$(uname)" != "Darwin" ]] && exit 0

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "$DOTDOTFILES/bash/core/tools.bash"
dotfiles_log_init "ssh-key-load-mac"

if ! /usr/bin/ssh-add -l 2>/dev/null | grep -q "id_ed25519"; then
    dotfiles_log "loading ed25519 key into keychain"
    dotfiles_run /usr/bin/ssh-add --apple-use-keychain ~/.ssh/id_ed25519
else
    dotfiles_log "key already loaded"
fi
