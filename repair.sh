#!/usr/bin/env bash

DOTDOTFILES="$(dirname "$(readlink -f "$0")")"
export DOTDOTFILES

# call update.sh
"$DOTDOTFILES/update.sh"