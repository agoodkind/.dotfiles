#!/usr/bin/env bash
# Dispatch background jobs to run in the background
# used by zshrc/incl.zsh to run background jobs
DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

bash "$DOTDOTFILES/bash/background/updater.bash" &
bash "$DOTDOTFILES/bash/background/prefer-cache-check.bash" &
bash "$DOTDOTFILES/bash/background/zwc-recompile.bash" &
bash "$DOTDOTFILES/bash/background/ssh-key-load-mac.bash" &

wait
