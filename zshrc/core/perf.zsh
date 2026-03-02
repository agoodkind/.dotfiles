# Override non-tty exit behavior by setting ZSH_PERF=1
if [[ -t 1 || "${ZSH_PERF:-}" == "1" ]]; then
    typeset -gA _PROFILE_TIMES
    _PROFILE_TIMES[_pre_zshrc]=$(( (EPOCHREALTIME - START_TIME) * 1000 ))

    if (( ${+_ZSHENV_TIMES} )); then
        local zshenv_self=$(( (_ZSHENV_TIMES[end] - _ZSHENV_TIMES[start]) * 1000 ))
        local system_time=$(( _PROFILE_TIMES[_pre_zshrc] - zshenv_self ))
        _PROFILE_TIMES[_zshenv_self]=$zshenv_self
        _PROFILE_TIMES[_system_zsh]=$system_time
    fi

    if (( ${+_ETC_TIMES} )); then
        local _t
        _t() { echo $(( ($2 - $1) * 1000 )) }

        if (( ${+_ETC_TIMES[zprofile_start]} && ${+_ETC_TIMES[zprofile_end]} )); then
            _PROFILE_TIMES[_etc_zprofile]=$(_t $_ETC_TIMES[zprofile_start] $_ETC_TIMES[zprofile_end])
        fi
        if (( ${+_ETC_TIMES[zshrc_start]} && ${+_ETC_TIMES[zshrc_end]} )); then
            _PROFILE_TIMES[_etc_zshrc]=$(_t $_ETC_TIMES[zshrc_start] $_ETC_TIMES[zshrc_end])
        fi
        if (( ${+_ETC_TIMES[zshrc_start]} && ${+_ETC_TIMES[zshrc_after_locale]} )); then
            _PROFILE_TIMES[_etc_zshrc_locale]=$(_t $_ETC_TIMES[zshrc_start] $_ETC_TIMES[zshrc_after_locale])
        fi
        if (( ${+_ETC_TIMES[zshrc_after_locale]} && ${+_ETC_TIMES[zshrc_after_hist]} )); then
            _PROFILE_TIMES[_etc_zshrc_hist]=$(_t $_ETC_TIMES[zshrc_after_locale] $_ETC_TIMES[zshrc_after_hist])
        fi
        if (( ${+_ETC_TIMES[zshrc_after_hist]} && ${+_ETC_TIMES[zshrc_after_terminfo]} )); then
            _PROFILE_TIMES[_etc_zshrc_terminfo]=$(_t $_ETC_TIMES[zshrc_after_hist] $_ETC_TIMES[zshrc_after_terminfo])
        fi
        if (( ${+_ETC_TIMES[zshrc_after_terminfo]} && ${+_ETC_TIMES[zshrc_after_bindkey]} )); then
            _PROFILE_TIMES[_etc_zshrc_bindkey]=$(_t $_ETC_TIMES[zshrc_after_terminfo] $_ETC_TIMES[zshrc_after_bindkey])
        fi
        if (( ${+_ETC_TIMES[zshrc_before_term]} && ${+_ETC_TIMES[zshrc_end]} )); then
            _PROFILE_TIMES[_etc_zshrc_term]=$(_t $_ETC_TIMES[zshrc_before_term] $_ETC_TIMES[zshrc_end])
        fi

        unfunction _t
    fi

    SHOULD_PROFILE=false
    if [[ -f ~/.cache/zsh_profile_next ]]; then
        SHOULD_PROFILE=true
        zmodload zsh/zprof
        rm ~/.cache/zsh_profile_next
    fi
    export SHOULD_PROFILE
fi

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
    local ready
    if (( ${+_PROFILE_TIMES[_time_to_ready]} )); then
        ready=${_PROFILE_TIMES[_time_to_ready]}
    else
        ready=$(( (EPOCHREALTIME - START_TIME) * 1000 ))
    fi
    local zshrc_time=$(( prompt - pre ))

    printf "shell startup: %.0f ms (time-to-prompt)\n" "$prompt"
    printf "├── pre-zshrc: %.0f ms\n" "$pre"
    if [[ -n "${_PROFILE_TIMES[_system_zsh]:-}" ]]; then
        printf "│   ├── system:  %.0f ms\n" "${_PROFILE_TIMES[_system_zsh]}"
        if [[ -n "${_PROFILE_TIMES[_etc_zprofile]:-}" ]]; then
            printf "│   │   ├── /etc/zprofile:          %.1f ms  (path_helper)\n" \
                "${_PROFILE_TIMES[_etc_zprofile]}"
        fi
        if [[ -n "${_PROFILE_TIMES[_etc_zshrc]:-}" ]]; then
            printf "│   │   └── /etc/zshrc:             %.1f ms\n" \
                "${_PROFILE_TIMES[_etc_zshrc]}"
            [[ -n "${_PROFILE_TIMES[_etc_zshrc_locale]:-}"   ]] && \
                printf "│   │       ├── locale/combining:   %.1f ms\n" "${_PROFILE_TIMES[_etc_zshrc_locale]}"
            [[ -n "${_PROFILE_TIMES[_etc_zshrc_hist]:-}"     ]] && \
                printf "│   │       ├── history/opts:       %.1f ms\n" "${_PROFILE_TIMES[_etc_zshrc_hist]}"
            [[ -n "${_PROFILE_TIMES[_etc_zshrc_terminfo]:-}" ]] && \
                printf "│   │       ├── terminfo:           %.1f ms\n" "${_PROFILE_TIMES[_etc_zshrc_terminfo]}"
            [[ -n "${_PROFILE_TIMES[_etc_zshrc_bindkey]:-}"  ]] && \
                printf "│   │       ├── bindkey:            %.1f ms\n" "${_PROFILE_TIMES[_etc_zshrc_bindkey]}"
            [[ -n "${_PROFILE_TIMES[_etc_zshrc_term]:-}"     ]] && \
                printf "│   │       └── zshrc_%s:  %.1f ms\n" \
                    "${TERM_PROGRAM:-unset}" "${_PROFILE_TIMES[_etc_zshrc_term]}"
        fi
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

function _write_startup_log() {
    [[ -t 1 || "${ZSH_PERF:-}" == "1" ]] || return 0
    local log_dir=~/.cache/zsh_startup
    mkdir -p "$log_dir"

    local tty_id=$(_perf_tty_id)
    local ts=$(date '+%Y%m%d_%H%M%S')
    local log="$log_dir/${ts}_${tty_id}.json"
    local ready_ms=$(( (EPOCHREALTIME - START_TIME) * 1000 ))
    # Snapshot once so zsh_perf reads a stable value rather than re-computing EPOCHREALTIME - START_TIME on each call.
    _PROFILE_TIMES[_time_to_ready]=$ready_ms

    # Build sections object
    local sections_json="{}"
    local k
    for k in ${(ok)_PROFILE_TIMES}; do
        [[ "$k" == _* ]] && continue
        local key=${k//:/_}
        sections_json=$(jq -n \
            --argjson obj "$sections_json" \
            --arg     key "$key" \
            --argjson val "${_PROFILE_TIMES[$k]}" \
            '$obj + {($key): $val}')
    done

    # Read syssnap if available
    local snap_file="$log_dir/.syssnap_$$"
    local syssnap_json="null"
    if [[ -f "$snap_file" ]]; then
        syssnap_json=$(jq -Rs '.' "$snap_file" 2>/dev/null || echo "null")
        rm -f "$snap_file"
    fi

    jq -n \
        --arg     ts          "$(date '+%Y-%m-%d %H:%M:%S %Z')" \
        --arg     tty         "$tty_id" \
        --argjson pid         "$$" \
        --arg     term        "${TERM_PROGRAM:-unset}" \
        --argjson pre_zshrc      "${_PROFILE_TIMES[_pre_zshrc]:-0}" \
        --argjson system_zsh     "${_PROFILE_TIMES[_system_zsh]:-0}" \
        --argjson etc_zprofile        "${_PROFILE_TIMES[_etc_zprofile]:-null}" \
        --argjson etc_zshrc           "${_PROFILE_TIMES[_etc_zshrc]:-null}" \
        --argjson etc_zshrc_locale    "${_PROFILE_TIMES[_etc_zshrc_locale]:-null}" \
        --argjson etc_zshrc_hist      "${_PROFILE_TIMES[_etc_zshrc_hist]:-null}" \
        --argjson etc_zshrc_terminfo  "${_PROFILE_TIMES[_etc_zshrc_terminfo]:-null}" \
        --argjson etc_zshrc_bindkey   "${_PROFILE_TIMES[_etc_zshrc_bindkey]:-null}" \
        --argjson etc_zshrc_term      "${_PROFILE_TIMES[_etc_zshrc_term]:-null}" \
        --argjson zshenv_self    "${_PROFILE_TIMES[_zshenv_self]:-0}" \
        --argjson cargo_ms       "${_ZSHENV_TIMES[cargo]:-0}" \
        --argjson time_prompt    "${_PROFILE_TIMES[_time_to_prompt]:-0}" \
        --argjson first_precmd   "${_PROFILE_TIMES[_first_precmd]:-0}" \
        --argjson time_ready     "$ready_ms" \
        --argjson sections       "$sections_json" \
        --argjson syssnap        "$syssnap_json" \
        --argjson zprof          "${_ZPROF_OUTPUT:+$(jq -Rs '.' <<< "$_ZPROF_OUTPUT")}null" \
        '{
            ts:           $ts,
            tty:          $tty,
            pid:          $pid,
            term_program: $term,
            ms: {
                pre_zshrc:      $pre_zshrc,
                system_zsh:          $system_zsh,
                etc_zprofile:        $etc_zprofile,
                etc_zshrc:           $etc_zshrc,
                etc_zshrc_locale:    $etc_zshrc_locale,
                etc_zshrc_hist:      $etc_zshrc_hist,
                etc_zshrc_terminfo:  $etc_zshrc_terminfo,
                etc_zshrc_bindkey:   $etc_zshrc_bindkey,
                etc_zshrc_term:      $etc_zshrc_term,
                zshenv_self:    $zshenv_self,
                cargo:          $cargo_ms,
                time_prompt:    $time_prompt,
                first_precmd:   $first_precmd,
                time_ready:     $time_ready
            },
            sections:     $sections,
            syssnap:      $syssnap,
            zprof:        $zprof
        }' > "$log"

    ln -sf "$log" "$log_dir/latest.json"

    # Prune oldest beyond 500, in background
    (
        local -a all=("$log_dir"/*.json(N.om))
        all=("${(@)all:#*latest.json}")
        local count=${#all}
        if (( count > 500 )); then
            rm -f "${all[-$(( count - 500 )),-1]}"
        fi
    ) &!
}

function zsh_perf_log() {
    local log_dir=~/.cache/zsh_startup
    local tty_id=$(_perf_tty_id)
    local -a matches=("$log_dir"/*_${tty_id}.json(N.om))
    local log
    if (( ${#matches} > 0 )); then
        log="${matches[1]}"
    elif [[ -f "$log_dir/latest.json" ]]; then
        log="$log_dir/latest.json"
    else
        echo "No startup log found"
        return 1
    fi
    jq '.' "$log"
}

function zsh_perf_history() {
    local log_dir=~/.cache/zsh_startup
    local limit=50
    local slow_only=false
    local json_out=false

    for arg in "$@"; do
        case "$arg" in
            --slow)   slow_only=true ;;
            --all)    limit=9999 ;;
            --json)   json_out=true ;;
            --last=*) limit=${arg#--last=} ;;
        esac
    done

    local -a logs=("$log_dir"/*.json(N.om))
    logs=("${(@)logs:#*latest.json}")
    if (( ${#logs} == 0 )); then
        echo "No startup logs found"
        return 1
    fi

    local total=${#logs}
    (( total > limit )) && logs=("${logs[@]:0:$limit}")

    if [[ "$json_out" == true ]]; then
        jq -s '.' "${logs[@]}"
        return 0
    fi

    local jq_filter='
        .ms.pre_zshrc   as $pre    |
        .ms.time_prompt as $prompt |
        .ms.first_precmd as $precmd |
        .ms.time_ready  as $ready  |
        (if $pre    > 100 then "SLOW-PRE "    else "" end) +
        (if $prompt > 300 then "SLOW-PROMPT " else "" end)
        as $flags |
        [ .ts[0:16], .tty,
          ($pre    | round | tostring),
          ($prompt | round | tostring),
          ($precmd | round | tostring),
          ($ready  | round | tostring),
          $flags
        ] | @tsv'

    if [[ "$slow_only" == true ]]; then
        jq_filter='select(.ms.pre_zshrc > 100 or .ms.time_prompt > 300) | '"$jq_filter"
    fi

    printf "%-18s %-10s %8s %8s %8s %8s  %s\n" \
        "timestamp" "tty" "pre-rc" "prompt" "precmd" "ready" "flags"
    printf "%-18s %-10s %8s %8s %8s %8s\n" \
        "---------" "---" "------" "------" "------" "-----"

    jq -r "$jq_filter" "${logs[@]}" 2>/dev/null | \
        awk -F'\t' '{printf "%-18s %-10s %8s %8s %8s %8s  %s\n", $1,$2,$3,$4,$5,$6,$7}'

    printf "\n%d logs shown (of %d total).  --slow  --all  --last=N  --json\n" \
        "${#logs}" "$total"
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
