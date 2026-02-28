typeset -gA _PROFILE_TIMES
_PROFILE_TIMES[_pre_zshrc]=$(( (EPOCHREALTIME - START_TIME) * 1000 ))

if (( ${+_ZSHENV_TIMES} )); then
    local zshenv_self=$(( (_ZSHENV_TIMES[end] - _ZSHENV_TIMES[start]) * 1000 ))
    local system_time=$(( _PROFILE_TIMES[_pre_zshrc] - zshenv_self ))
    _PROFILE_TIMES[_zshenv_self]=$zshenv_self
    _PROFILE_TIMES[_system_zsh]=$system_time
fi

SHOULD_PROFILE=false
if [[ -f ~/.cache/zsh_profile_next ]]; then
    SHOULD_PROFILE=true
    zmodload zsh/zprof
    rm ~/.cache/zsh_profile_next
fi
export SHOULD_PROFILE

function _source() {
    local label=${1:t:r}
    local t0=$EPOCHREALTIME
    source "$1"
    _PROFILE_TIMES[$label]=$(( (EPOCHREALTIME - t0) * 1000 ))
}

function _async() {
    local label=${1:t:r}
    local t0=$EPOCHREALTIME
    ("$@" >/dev/null 2>&1 &)
    _PROFILE_TIMES[${label}:async]=$(( (EPOCHREALTIME - t0) * 1000 ))
}

function zsh_profile() {
    mkdir -p ~/.cache
    touch ~/.cache/zsh_profile_next
    echo "Performance profiling enabled for next shell session"
}

function _perf_tty_id() {
    local tty=${TTY:t}
    echo "${tty//\//-}"
}

function zsh_perf() {
    local pre=${_PROFILE_TIMES[_pre_zshrc]:-0}
    local prompt=${_PROFILE_TIMES[_time_to_prompt]:-0}
    local precmd=${_PROFILE_TIMES[_first_precmd]:-0}
    local ready=$(( (EPOCHREALTIME - START_TIME) * 1000 ))
    local zshrc_time=$(( prompt - pre ))

    printf "shell startup: %.0f ms (time-to-prompt)\n" "$prompt"
    printf "├── pre-zshrc: %.0f ms\n" "$pre"
    if [[ -n "${_PROFILE_TIMES[_system_zsh]:-}" ]]; then
        printf "│   ├── system:  %.0f ms  (/etc/zprofile, /etc/zshrc)\n" \
            "${_PROFILE_TIMES[_system_zsh]}"
        printf "│   └── .zshenv: %.0f ms\n" "${_PROFILE_TIMES[_zshenv_self]}"
    fi
    printf "└── .zshrc: %.0f ms\n" "$zshrc_time"

    local -a sections=() async_sections=()
    local k
    for k in ${(ok)_PROFILE_TIMES}; do
        [[ "$k" == _* ]] && continue
        [[ -z "$k" ]] && continue
        if [[ "$k" == *:async ]]; then
            async_sections+=("$k")
        else
            sections+=("$k")
        fi
    done

    local total=${#sections}
    local has_async=$(( ${#async_sections} > 0 ))
    local i
    for i in {1..$total}; do
        k=${sections[$i]}
        if (( i == total )) && (( ! has_async )); then
            printf "    └── %-14s %5.1f ms\n" "$k" "${_PROFILE_TIMES[$k]}"
        else
            printf "    ├── %-14s %5.1f ms\n" "$k" "${_PROFILE_TIMES[$k]}"
        fi
    done

    for i in {1..${#async_sections}}; do
        k=${async_sections[$i]}
        local display_name=${k%:async}
        [[ -z "$display_name" ]] && continue
        if (( i == ${#async_sections} )); then
            printf "    └── %-14s %5.1f ms (background)\n" "$display_name" "${_PROFILE_TIMES[$k]}"
        else
            printf "    ├── %-14s %5.1f ms (background)\n" "$display_name" "${_PROFILE_TIMES[$k]}"
        fi
    done

    printf "\nfirst-precmd:  %.0f ms  (prompt visible)\n" "$precmd"
    printf "time-to-ready: %.0f ms  (after zinit turbo)\n" "$ready"
}

function zsh_perf_log() {
    local log_dir=~/.cache/zsh_startup
    local tty_id=$(_perf_tty_id)
    local -a matches=("$log_dir"/*_${tty_id}.log(N.om))
    if (( ${#matches} > 0 )); then
        cat "${matches[1]}"
    elif [[ -f "$log_dir/latest.log" ]]; then
        cat "$log_dir/latest.log"
    else
        echo "No startup log found"
    fi
}

function zsh_perf_bench() {
    echo "Benchmarking shell startup (5 runs)..."
    local total=0 i
    for i in {1..5}; do
        local t=$({ time zsh -i -c exit; } 2>&1 | grep total | awk '{print $NF}')
        t=${t%total}
        local ms=$(echo "$t" | awk -F'[ms]' '{print ($1*60 + $2) * 1000}')
        total=$((total + ${ms%.*}))
        printf "  run %d: %s\n" "$i" "$t"
    done
    printf "Average: %.0f ms\n" "$((total / 5))"
}

function zsh_perf_history() {
    local log_dir=~/.cache/zsh_startup
    local -a logs=("$log_dir"/*.log(N.om))
    logs=("${(@)logs:#*latest.log}")
    if (( ${#logs} == 0 )); then
        echo "No startup logs found"
        return 1
    fi
    printf "%-20s %10s %10s %10s %10s\n" \
        "timestamp" "tty" "prompt" "precmd" "ready"
    printf "%-20s %10s %10s %10s %10s\n" \
        "---------" "---" "------" "------" "-----"
    for log in "${logs[@]}"; do
        local name=${${log:t}%.log}
        local ts=${name%%_[pt]*}
        local tty=${name#*_}
        local prompt_time precmd_time ready_time
        prompt_time=$(grep 'time-to-prompt:' "$log" 2>/dev/null | awk '{print $2}')
        precmd_time=$(grep 'first-precmd:' "$log" 2>/dev/null | awk '{print $2}')
        ready_time=$(grep 'time-to-ready:' "$log" 2>/dev/null | awk '{print $2}')
        printf "%-20s %10s %10s %10s %10s\n" \
            "$ts" "$tty" "${prompt_time:-?}" "${precmd_time:-?}" "${ready_time:-?}"
    done
}

function _write_startup_log() {
    local log_dir=~/.cache/zsh_startup
    mkdir -p "$log_dir"

    local tty_id=$(_perf_tty_id)
    local ts=$(date '+%Y%m%d_%H%M%S')
    local log="$log_dir/${ts}_${tty_id}.log"

    {
        printf "# %s  tty=%s\n" "$(date '+%Y-%m-%d %H:%M:%S %Z')" "$tty_id"
        printf "pre-zshrc:      %.0f ms\n" "${_PROFILE_TIMES[_pre_zshrc]:-0}"
        if [[ -n "${_PROFILE_TIMES[_system_zsh]:-}" ]]; then
            printf "  system_zsh:   %.0f ms  (/etc/zprofile, /etc/zshrc)\n" \
                "${_PROFILE_TIMES[_system_zsh]}"
            printf "  zshenv_self:  %.0f ms  (.zshenv user code)\n" \
                "${_PROFILE_TIMES[_zshenv_self]}"
        fi
        printf "time-to-prompt: %.0f ms\n" "${_PROFILE_TIMES[_time_to_prompt]}"
        printf "first-precmd:   %.0f ms  (prompt visible)\n" \
            "${_PROFILE_TIMES[_first_precmd]:-0}"
        printf "time-to-ready:  %.0f ms  (after zinit turbo)\n" \
            "$(( (EPOCHREALTIME - START_TIME) * 1000 ))"
        printf "\nPer-section times:\n"
        local k
        for k in ${(ok)_PROFILE_TIMES}; do
            [[ "$k" == _* ]] && continue
            printf "  %-20s %5.1f ms\n" "$k" "${_PROFILE_TIMES[$k]}"
        done
        if [[ -n "${_ZPROF_OUTPUT:-}" ]]; then
            printf "\nzprof output:\n%s\n" "$_ZPROF_OUTPUT"
        fi
    } > "$log"

    ln -sf "$log" "$log_dir/latest.log"
}

function _perf_first_precmd() {
    _PROFILE_TIMES[_first_precmd]=$(( (EPOCHREALTIME - START_TIME) * 1000 ))
    precmd_functions=(${precmd_functions:#_perf_first_precmd})
}
precmd_functions=(_perf_first_precmd $precmd_functions)

function do_profile() {
    [[ "$SHOULD_PROFILE" == "true" ]] || return 0
    typeset -g _ZPROF_OUTPUT
    _ZPROF_OUTPUT=$(zprof)
    echo "Zsh performance profiling results:"
    echo "$_ZPROF_OUTPUT"
    echo ""
    printf "pre-zshrc: %.0f ms\n" "${_PROFILE_TIMES[_pre_zshrc]:-0}"
    echo ""
    echo "Per-section times:"
    local k
    for k in ${(ok)_PROFILE_TIMES}; do
        [[ "$k" == _* ]] && continue
        printf "  %-20s %5.1f ms\n" "$k" "${_PROFILE_TIMES[$k]}"
    done
    printf "\ntime-to-prompt: %.0f ms\n" "${_PROFILE_TIMES[_time_to_prompt]}"
}
