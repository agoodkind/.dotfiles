# Fallback: if .zshenv armed zprof but failed to load it, load now.
if (( _ZPROF_ARMED && ! _ZPROF_LOADED )); then
    zmodload zsh/zprof 2>/dev/null && _ZPROF_LOADED=1
fi

source "${0:h:h:h}/lib/tree.zsh"

typeset -gA _PROFILE_TIMES
_PROFILE_TIMES[_pre_zshrc]=$(( (EPOCHREALTIME - START_TIME) * 1000 ))

local zshenv_self=${_PROFILE_TIMES[_zshenv_self]:-0}
local system_time=$(( _PROFILE_TIMES[_pre_zshrc] - zshenv_self ))
_PROFILE_TIMES[_system_zsh]=$system_time

# Inject structural nodes at the front of _PERF_TREE.
# .zshenv pushed probes at depth 2+ during /etc/zprofile, ~/.zprofile, /etc/zshrc.
# We prepend the section headers that give those probes their parents.
local -a _sys_header=(
    "1:.zshenv:${zshenv_self}"
    "1:system:${system_time}"
)
_PERF_TREE=("${_sys_header[@]}" "${_PERF_TREE[@]}")

# Sanity check: sum system probe entries and flag unaccounted time
local _sc_accounted=0 _sc_entry _sc_rest _sc_ms
for _sc_entry in "${_PERF_TREE[@]}"; do
    local _sc_depth=${_sc_entry%%:*}
    (( _sc_depth >= 2 )) || continue
    _sc_rest=${_sc_entry#*:}; _sc_rest=${_sc_rest#*:}
    _sc_ms=${_sc_rest%%:*}
    _sc_accounted=$(( _sc_accounted + _sc_ms ))
done
local _sc_remainder=$(( system_time - _sc_accounted ))
_PERF_TREE+=("2:[gap /etc/zshrc→.zshrc]:${_sc_remainder}")

typeset -gi _SOURCE_DEPTH=0

function _source() {
    local _tail=${1:t}
    local label=${_tail%.zsh}
    local t0=$EPOCHREALTIME
    (( _SOURCE_DEPTH++ ))
    source "$1"
    (( _SOURCE_DEPTH-- ))
    local ms=$(( (EPOCHREALTIME - t0) * 1000 ))
    _PROFILE_TIMES[$label]=$ms
    _PERF_TREE+=("$(( _SOURCE_DEPTH + 2 )):${label}:${ms}")
}

function _async() {
    local _tail=${1:t}
    local label=${_tail%.zsh}
    local t0=$EPOCHREALTIME
    ("$@" >/dev/null 2>&1 &)
    local ms=$(( (EPOCHREALTIME - t0) * 1000 ))
    _PROFILE_TIMES[${label}:async]=$ms
    _PERF_TREE+=("$(( _SOURCE_DEPTH + 2 )):${label}:${ms}:background")
}

function _perf_tty_id() {
    local tty=${TTY:t}
    echo "${tty//\//-}"
}

typeset -g _ZPROF_OUTPUT=""

function _perf_first_precmd() {
    _PROFILE_TIMES[_first_precmd]=$(( (EPOCHREALTIME - START_TIME) * 1000 ))
    if (( _ZPROF_LOADED )); then
        _ZPROF_OUTPUT=$(zprof 2>/dev/null)
    fi
    precmd_functions=(${precmd_functions:#_perf_first_precmd})
}
precmd_functions=(_perf_first_precmd $precmd_functions)

function zsh_perf() {
    local prompt=${_PROFILE_TIMES[_time_to_prompt]:-0}
    local precmd=${_PROFILE_TIMES[_first_precmd]:-0}
    local ready
    if (( ${+_PROFILE_TIMES[_time_to_ready]} )); then
        ready=${_PROFILE_TIMES[_time_to_ready]}
    else
        ready=$(( (EPOCHREALTIME - START_TIME) * 1000 ))
    fi

    printf "shell startup: %.0f ms (time-to-prompt)\n" "$prompt"
    tree_print _PERF_TREE ""

    local has_zprof=$(( ${#_ZPROF_OUTPUT} > 0 ))
    (( has_zprof )) && _perf_print_zprof_section "$_ZPROF_OUTPUT"

    printf "\nfirst-precmd:  %.0f ms  (prompt visible)\n" "$precmd"
    printf "time-to-ready: %.0f ms  (shell interactive)\n" "$ready"
}

# zprof summary line format (9 fields after word-split):
#   num)  calls  total_cumul  total_avg  total_%  self_cumul  self_avg  self_%  name
#   [1]   [2]    [3]          [4]        [5]      [6]         [7]       [8]     [9]
function _perf_print_zprof_section() {
    local zprof_out=$1

    local -a entries=()
    local zp_line sep_count=0
    while IFS= read -r zp_line; do
        if [[ "$zp_line" == --* ]]; then
            (( ++sep_count >= 2 )) && break
            continue
        fi
        (( sep_count == 1 )) || continue
        [[ "$zp_line" =~ ^[[:space:]]+[0-9]+\) ]] || continue
        entries+=("$zp_line")
    done <<< "$zprof_out"

    (( ${#entries} == 0 )) && return 0

    local total_entries=${#entries}
    printf "    └── [zprof: %d functions]\n" "$total_entries"

    local zi zp_f zp_name zp_total zp_self zp_calls zp_s zp_branch
    for zi in {1..$total_entries}; do
        zp_f=( ${=entries[$zi]} )
        (( ${#zp_f} >= 9 )) || continue

        zp_name=${zp_f[9]}
        zp_total=${zp_f[3]}
        zp_self=${zp_f[6]}
        zp_calls=${zp_f[2]}
        zp_s="s"
        (( zp_calls == 1 )) && zp_s=""

        if (( zi == total_entries )); then
            zp_branch="└──"
        else
            zp_branch="├──"
        fi

        printf "        %s %-28s %6.2f ms self   %6.2f ms total   (%s call%s)\n" \
            "$zp_branch" "$zp_name" "$zp_self" "$zp_total" "$zp_calls" "$zp_s"
    done
}

function _write_startup_log() {
    [[ -t 1 || "${ZSH_PERF:-}" == "1" ]] || return 0
    ( __write_startup_log_impl >/dev/null 2>&1 ) &!
}
function __write_startup_log_impl() {
    local log_dir=~/.cache/zsh_startup
    mkdir -p "$log_dir"

    local tty_id=$(_perf_tty_id)
    local ts=$(date '+%Y%m%d_%H%M%S')
    local log="$log_dir/${ts}_${tty_id}.json"
    local ready_ms=${_PROFILE_TIMES[_time_to_ready]:-$(( (EPOCHREALTIME - START_TIME) * 1000 ))}

    # Flat sections JSON (simple key -> ms mapping)
    local sections_json="{}"
    local k key
    for k in ${(ok)_PROFILE_TIMES}; do
        [[ "$k" == _* ]] && continue
        key=${k//:/_}
        sections_json=$(jq -n \
            --argjson obj "$sections_json" \
            --arg     key "$key" \
            --argjson val "${_PROFILE_TIMES[$k]}" \
            '$obj + {($key): $val}')
    done

    # Build tree JSON array from _PERF_TREE
    local tree_json="[]"
    local pt_entry pt_rest pt_depth pt_label pt_ms pt_tag
    for pt_entry in "${_PERF_TREE[@]}"; do
        pt_depth=${pt_entry%%:*}; pt_rest=${pt_entry#*:}
        pt_label=${pt_rest%%:*}; pt_rest=${pt_rest#*:}
        pt_ms=${pt_rest%%:*}; pt_tag=${pt_rest#*:}
        [[ "$pt_tag" == "$pt_ms" ]] && pt_tag=""
        tree_json=$(jq -n \
            --argjson arr "$tree_json" \
            --argjson depth "$pt_depth" \
            --arg     label "$pt_label" \
            --argjson ms "$pt_ms" \
            --arg     tag "$pt_tag" \
            '$arr + [{depth: $depth, label: $label, ms: $ms} + (if $tag != "" then {tag: $tag} else {} end)]')
    done

    # Read syssnap if available
    local snap_file="$log_dir/.syssnap_$$"
    local syssnap_json="null"
    if [[ -f "$snap_file" ]]; then
        syssnap_json=$(jq -Rs '.' "$snap_file" 2>/dev/null || echo "null")
        rm -f "$snap_file"
    fi

    local zprof_json="null"
    if [[ -n "${_ZPROF_OUTPUT:-}" ]]; then
        zprof_json=$(jq -Rs '.' <<< "$_ZPROF_OUTPUT" 2>/dev/null || echo "null")
    fi

    jq -n \
        --arg     ts          "$(date '+%Y-%m-%d %H:%M:%S %Z')" \
        --arg     tty         "$tty_id" \
        --argjson pid         "$$" \
        --arg     term        "${TERM_PROGRAM:-unset}" \
        --argjson pre_zshrc           "${_PROFILE_TIMES[_pre_zshrc]:-0}" \
        --argjson system_zsh          "${_PROFILE_TIMES[_system_zsh]:-0}" \
        --argjson path_helper_done    "${_PATH_HELPER_DONE:-0}" \
        --argjson locale_done         "${_LOCALE_DONE:-0}" \
        --argjson zshenv_self         "${_PROFILE_TIMES[_zshenv_self]:-0}" \
        --argjson time_prompt         "${_PROFILE_TIMES[_time_to_prompt]:-0}" \
        --argjson first_precmd        "${_PROFILE_TIMES[_first_precmd]:-0}" \
        --argjson time_ready          "$ready_ms" \
        --argjson tree                "$tree_json" \
        --argjson sections            "$sections_json" \
        --argjson syssnap             "$syssnap_json" \
        --argjson zprof               "$zprof_json" \
        '{
            ts:           $ts,
            tty:          $tty,
            pid:          $pid,
            term_program: $term,
            bypasses: {
                path_helper_cached: ($path_helper_done == 1),
                locale_bypassed:    ($locale_done == 1)
            },
            ms: {
                pre_zshrc:      $pre_zshrc,
                system_zsh:     $system_zsh,
                zshenv_self:    $zshenv_self,
                time_prompt:    $time_prompt,
                first_precmd:   $first_precmd,
                time_ready:     $time_ready
            },
            tree:             $tree,
            sections:         $sections,
            syssnap:          $syssnap,
            zprof:            $zprof
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
        .ms.pre_zshrc    as $pre    |
        .ms.time_prompt  as $prompt |
        .ms.first_precmd as $precmd |
        .ms.time_ready   as $ready  |
        ((.tree // .system_timeline // []) | map(select(.label == "path_helper_fork")) | .[0].ms // 0) as $ph |
        ((.tree // .system_timeline // []) | map(select(.label == "combining/locale")) | .[0].ms // 0) as $loc |
        (if (.bypasses.path_helper_cached // false) then "P" else "-" end) as $ph_flag |
        (if (.bypasses.locale_bypassed // false)    then "L" else "-" end) as $loc_flag |
        (if $pre    > 100 then "SLOW-PRE "    else "" end) +
        (if $prompt > 300 then "SLOW-PROMPT " else "" end)
        as $slow |
        [ .ts[0:16], .tty,
          ($pre    | round | tostring),
          ($prompt | round | tostring),
          ($precmd | round | tostring),
          ($ready  | round | tostring),
          ($ph     | round | tostring),
          ($loc    | round | tostring),
          ($ph_flag + $loc_flag),
          $slow
        ] | @tsv'

    if [[ "$slow_only" == true ]]; then
        jq_filter='select(.ms.pre_zshrc > 100 or .ms.time_prompt > 300) | '"$jq_filter"
    fi

    printf "%-18s %-10s %8s %8s %8s %8s %6s %6s  %s %s\n" \
        "timestamp" "tty" "pre-rc" "prompt" "precmd" "ready" "ph" "locale" "by" "flags"
    printf "%-18s %-10s %8s %8s %8s %8s %6s %6s\n" \
        "---------" "---" "------" "------" "------" "-----" "--" "------"

    jq -r "$jq_filter" "${logs[@]}" 2>/dev/null | \
        awk -F'\t' '{printf "%-18s %-10s %8s %8s %8s %8s %6s %6s  %s %s\n", $1,$2,$3,$4,$5,$6,$7,$8,$9,$10}'

    printf "\n%d logs shown (of %d total).  --slow  --all  --last=N  --json\n" \
        "${#logs}" "$total"
    printf "by: P=path_helper cached  L=locale bypassed\n"
}

function path_cache_rebuild() {
    local cache_dir=~/.cache/zsh_startup
    local cache_file="$cache_dir/path_cache.zsh"
    mkdir -p "$cache_dir"
    if [[ ! -x /usr/libexec/path_helper ]]; then
        echo "path_helper not found at /usr/libexec/path_helper"
        return 1
    fi
    /usr/libexec/path_helper -s > "$cache_file"
    echo "path cache rebuilt: $cache_file"
    echo "contents: $(cat $cache_file)"
}

function zsh_profile() {
    touch ~/.zsh_profile_next
    echo "zprof armed — open a new shell, then run zsh_perf to see function-level detail."
}

function do_profile() {
    :
}
