typeset -gA _PROFILE_TIMES

SHOULD_PROFILE=false
if [[ -f ~/.cache/zsh_profile_next ]]; then
    SHOULD_PROFILE=true
    zmodload zsh/zprof
    rm ~/.cache/zsh_profile_next
fi
export SHOULD_PROFILE

_source() {
    local label=${1:t:r}
    local t0=$EPOCHREALTIME
    source "$1"
    _PROFILE_TIMES[$label]=$(( (EPOCHREALTIME - t0) * 1000 ))
}

_async() {
    local label=${1:t:r}
    local t0=$EPOCHREALTIME
    ("$@" >/dev/null 2>&1 &)
    _PROFILE_TIMES["${label}(async)"]=$(( (EPOCHREALTIME - t0) * 1000 ))
}

zsh_profile() {
    mkdir -p ~/.cache
    touch ~/.cache/zsh_profile_next
    echo "Performance profiling enabled for next shell session"
}

_write_startup_log() {
    local log=~/.cache/zsh_startup_last.log
    mkdir -p ~/.cache
    {
        printf "# %s\n" "$(date '+%Y-%m-%d %H:%M:%S %Z')"
        printf ".zshrc time:   %.0f ms\n" "$(( (EPOCHREALTIME - START_TIME) * 1000 ))"
        if [[ -n "${_PROFILE_TIMES[zinit_turbo]:-}" ]]; then
            printf "time-to-ready: %.0f ms  (includes zinit turbo)\n" \
                "${_PROFILE_TIMES[zinit_turbo]}"
        fi
        printf "\nPer-file source times:\n"
        for k in ${(ok)_PROFILE_TIMES}; do
            printf "  %-20s %5.1f ms\n" "$k" "${_PROFILE_TIMES[$k]}"
        done
    } > "$log"
}

do_profile() {
    [[ "$SHOULD_PROFILE" == "true" ]] || return 0
    echo "Zsh performance profiling results:"
    zprof
    echo ""
    echo "Per-file source times:"
    for k in ${(ok)_PROFILE_TIMES}; do
        printf "  %-20s %5.1f ms\n" "$k" "${_PROFILE_TIMES[$k]}"
    done
    printf "\n.zshrc time:   %.0f ms\n" "$(( (EPOCHREALTIME - START_TIME) * 1000 ))"
    if [[ -n "${_PROFILE_TIMES[zinit_turbo]:-}" ]]; then
        printf "time-to-ready: %.0f ms  (includes zinit turbo)\n" \
            "${_PROFILE_TIMES[zinit_turbo]}"
    fi
}
