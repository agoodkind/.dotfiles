#!/usr/bin/env zsh
# Runs zinit update and compile, printing exit code markers for each command.
# Invoked by update_zinit_plugins in sync.bash; output is routed to sync.log.

source "${DOTDOTFILES:-$HOME/.dotfiles}/lib/zinit/zinit.zsh"

zinit update --all --quiet 2>&1
update_rc=$?
printf "[zinit-update-exit: %d]\n" "$update_rc"

zinit compile --all 2>&1
compile_rc=$?
printf "[zinit-compile-exit: %d]\n" "$compile_rc"

(( update_rc == 0 && compile_rc == 0 ))
