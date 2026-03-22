# Source as the very first line of any bash entry point.
# Normalizes PATH on macOS so all child processes (#!/usr/bin/env bash shebangs)
# resolve modern tools. Then re-execs into bash 4+ if still on macOS's 3.2.
# $0 and $@ are the calling script's, so exec correctly re-runs that script.
if [[ "$(uname)" == "Darwin" ]]; then
    export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"
fi
if [[ "${BASH_VERSINFO[0]}" -lt 4 ]]; then
    for _bash in /opt/homebrew/bin/bash /usr/local/bin/bash; do
        if [[ -x "$_bash" ]]; then
            exec "$_bash" "$0" "$@"
        fi
    done
    echo "ERROR: bash 4+ is required but could not be found" >&2
    exit 1
fi
