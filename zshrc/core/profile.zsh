SHOULD_PROFILE=false
if [[ -f ~/.cache/zsh_profile_next ]]; then
    SHOULD_PROFILE=true
    zmodload zsh/zprof
    rm ~/.cache/zsh_profile_next
    typeset -gA _PROFILE_TIMES
fi
export SHOULD_PROFILE

_source() {
    if [[ "$SHOULD_PROFILE" == "true" ]]; then
        local label=${1:t:r}
        local t0=$EPOCHREALTIME
        source "$1"
        _PROFILE_TIMES[$label]=$(( (EPOCHREALTIME - t0) * 1000 ))
    else
        source "$1"
    fi
}

_async() {
    if [[ "$SHOULD_PROFILE" == "true" ]]; then
        local label=${1:t:r}
        local t0=$EPOCHREALTIME
        ("$@" >/dev/null 2>&1 &)
        _PROFILE_TIMES["${label}(async)"]=$(( (EPOCHREALTIME - t0) * 1000 ))
    else
        ("$@" >/dev/null 2>&1 &)
    fi
}

zsh_profile() {
    mkdir -p ~/.cache
    touch ~/.cache/zsh_profile_next
    echo "Performance profiling enabled for next shell session"
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
    printf "\nZsh initialization time: %.0f ms\n" "$(( (EPOCHREALTIME - START_TIME) * 1000 ))"
}
