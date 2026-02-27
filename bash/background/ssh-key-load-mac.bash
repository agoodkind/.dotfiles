#!/usr/bin/env bash
[[ "$(uname)" != "Darwin" ]] && exit 0

if ! /usr/bin/ssh-add -l 2>/dev/null | grep -q "id_ed25519"; then
    /usr/bin/ssh-add --apple-use-keychain ~/.ssh/id_ed25519
fi
