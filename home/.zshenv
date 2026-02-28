zmodload zsh/datetime
START_TIME=$EPOCHREALTIME

typeset -gA _ZSHENV_TIMES
_ZSHENV_TIMES[start]=$EPOCHREALTIME

[[ -f "$HOME/.cargo/env" ]] && . "$HOME/.cargo/env"

_ZSHENV_TIMES[cargo]=$(( (EPOCHREALTIME - _ZSHENV_TIMES[start]) * 1000 ))
_ZSHENV_TIMES[end]=$EPOCHREALTIME
