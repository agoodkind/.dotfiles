# Fallback: if .zshenv armed zprof but failed to load it, load now.
if (( _ZPROF_ARMED && ! _ZPROF_LOADED )); then
    zmodload zsh/zprof 2>/dev/null && _ZPROF_LOADED=1
fi

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

    # Convert _SYSTEM_ORDER from raw timestamps to ms deltas.
    # Input entries:  depth:label:epoch[:tag]
    # Output entries: depth:label:ms[:tag]
    if (( ${#_SYSTEM_ORDER} >= 2 )); then
        local -a _sys_resolved=()
        local _sys_idx _sys_prev_epoch _sys_entry _sys_depth _sys_label _sys_epoch _sys_tag _sys_ms _sys_rest

        _sys_prev_epoch=""
        for (( _sys_idx = 1; _sys_idx <= ${#_SYSTEM_ORDER}; _sys_idx++ )); do
            _sys_entry=${_SYSTEM_ORDER[$_sys_idx]}
            _sys_depth=${_sys_entry%%:*}; _sys_rest=${_sys_entry#*:}
            _sys_label=${_sys_rest%%:*}; _sys_rest=${_sys_rest#*:}
            _sys_epoch=${_sys_rest%%:*}; _sys_tag=${_sys_rest#*:}
            [[ "$_sys_tag" == "$_sys_epoch" ]] && _sys_tag=""

            if [[ -n "$_sys_prev_epoch" ]]; then
                _sys_ms=$(( (_sys_epoch - _sys_prev_epoch) * 1000 ))
                _sys_resolved+=("${_sys_depth}:${_sys_label}:${_sys_ms}${_sys_tag:+:${_sys_tag}}")
            fi
            _sys_prev_epoch=$_sys_epoch
        done
        _SYSTEM_ORDER=("${_sys_resolved[@]}")
    fi
fi

typeset -gi _SOURCE_DEPTH=0
typeset -ga _SOURCE_ORDER=()

function _source() {
    local label=${1:t:r}
    local t0=$EPOCHREALTIME
    (( _SOURCE_DEPTH++ ))
    source "$1"
    (( _SOURCE_DEPTH-- ))
    local ms=$(( (EPOCHREALTIME - t0) * 1000 ))
    _PROFILE_TIMES[$label]=$ms
    _SOURCE_ORDER+=("${_SOURCE_DEPTH}:${label}:${ms}")
}

function _async() {
    local label=${1:t:r}
    local t0=$EPOCHREALTIME
    ("$@" >/dev/null 2>&1 &)
    local ms=$(( (EPOCHREALTIME - t0) * 1000 ))
    _PROFILE_TIMES[${label}:async]=$ms
    _SOURCE_ORDER+=("${_SOURCE_DEPTH}:${label}:${ms}:background")
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
        printf "│   ├── .zshenv: %.0f ms\n" "${_PROFILE_TIMES[_zshenv_self]}"
        printf "│   └── system:  %.0f ms\n" "${_PROFILE_TIMES[_system_zsh]}"
        _perf_print_system_tree
    fi
    printf "└── .zshrc: %.0f ms\n" "$zshrc_time"

    _perf_print_source_tree
    local has_zprof=$(( ${#_ZPROF_OUTPUT} > 0 ))
    (( has_zprof )) && _perf_print_zprof_section "$_ZPROF_OUTPUT"

    printf "\nfirst-precmd:  %.0f ms  (prompt visible)\n" "$precmd"
    printf "time-to-ready: %.0f ms  (shell interactive)\n" "$ready"

    if (( ${#_READY_ORDER} > 0 )); then
        printf "└── deferred (zle-line-init):\n"
        _perf_print_flat_tree _READY_ORDER "    "
    fi
}

# Generic tree renderer for ordered arrays of depth:label:ms[:tag] entries.
# $1 = name of the array variable (not the value)
# $2 = prefix string prepended to every line (for nesting under a parent)
function _perf_print_generic_tree() {
    local _tree_name=$1
    local prefix=$2
    local -a _tree_data=("${(@P)_tree_name}")
    local tree_total=${#_tree_data}
    (( tree_total )) || return 0

    local tree_idx look_idx ancestor_depth
    local node_depth node_label node_ms node_tag node_entry look_depth

    for (( tree_idx = 1; tree_idx <= tree_total; tree_idx++ )); do
        node_entry=${_tree_data[$tree_idx]}
        node_depth=${node_entry%%:*}; node_entry=${node_entry#*:}
        node_label=${node_entry%%:*}; node_entry=${node_entry#*:}
        node_ms=${node_entry%%:*}; node_tag=${node_entry#*:}
        [[ "$node_tag" == "$node_ms" ]] && node_tag=""

        local is_last_sibling=1
        for (( look_idx = tree_idx + 1; look_idx <= tree_total; look_idx++ )); do
            look_depth=${_tree_data[$look_idx]%%:*}
            if (( look_depth == node_depth )); then
                is_last_sibling=0
                break
            fi
            if (( look_depth < node_depth )); then
                break
            fi
        done

        local indent=""
        for (( ancestor_depth = 0; ancestor_depth < node_depth; ancestor_depth++ )); do
            local ancestor_has_more=0
            for (( look_idx = tree_idx + 1; look_idx <= tree_total; look_idx++ )); do
                look_depth=${_tree_data[$look_idx]%%:*}
                if (( look_depth == ancestor_depth )); then ancestor_has_more=1; break; fi
                if (( look_depth < ancestor_depth )); then break; fi
            done
            if (( ancestor_has_more )); then
                indent="${indent}│   "
            else
                indent="${indent}    "
            fi
        done

        local branch="├──"
        (( is_last_sibling )) && branch="└──"

        local suffix=""
        [[ -n "$node_tag" ]] && suffix="  ($node_tag)"

        printf "%s%s%s %-20s %5.1f ms%s\n" "$prefix" "$indent" "$branch" "$node_label" "$node_ms" "$suffix"
    done
}

# Flat tree renderer for ordered arrays of label:ms entries (no depth nesting).
# $1 = name of the array variable
# $2 = prefix string prepended to every line
function _perf_print_flat_tree() {
    local _flat_name=$1
    local prefix=$2
    local -a _flat_data=("${(@P)_flat_name}")
    local flat_total=${#_flat_data}
    (( flat_total )) || return 0

    local flat_idx flat_entry flat_label flat_ms flat_branch
    for (( flat_idx = 1; flat_idx <= flat_total; flat_idx++ )); do
        flat_entry=${_flat_data[$flat_idx]}
        flat_label=${flat_entry%%:*}
        flat_ms=${flat_entry#*:}
        if (( flat_idx == flat_total )); then
            flat_branch="└──"
        else
            flat_branch="├──"
        fi
        printf "%s%s %-20s %5.1f ms\n" "$prefix" "$flat_branch" "$flat_label" "$flat_ms"
    done
}

function _perf_print_source_tree() {
    (( ${#_SOURCE_ORDER} )) || return 0
    _perf_print_generic_tree _SOURCE_ORDER "    "
}

# Renders the system timeline from _SYSTEM_ORDER (already resolved to ms deltas).
# Appends a remainder entry if the sum of probes doesn't account for total system time.
function _perf_print_system_tree() {
    (( ${#_SYSTEM_ORDER} )) || return 0
    local sys_prefix="│       "

    # Sanity check: sum all resolved entries and flag unaccounted time
    local sys_total=${_PROFILE_TIMES[_system_zsh]:-0}
    local sys_accounted=0 sys_entry sys_rest sys_ms
    for sys_entry in "${_SYSTEM_ORDER[@]}"; do
        sys_rest=${sys_entry#*:}; sys_rest=${sys_rest#*:}
        sys_ms=${sys_rest%%:*}
        sys_accounted=$(( sys_accounted + sys_ms ))
    done
    local sys_remainder=$(( sys_total - sys_accounted ))

    local -a sys_display=("${_SYSTEM_ORDER[@]}")
    if (( ${sys_remainder#-} > 1 )); then
        sys_display+=("0:[/etc/zshrc→.zshrc]:${sys_remainder}")
    fi

    _perf_print_generic_tree sys_display "$sys_prefix"
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
    # Run in a detached subshell so jq forks don't block precmd or
    # trigger iTerm2's prompt redecoration (which causes a visible flash).
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

    # Build system_timeline JSON array from _SYSTEM_ORDER
    local sys_timeline_json="[]"
    local sys_entry sys_rest sys_depth sys_label sys_ms sys_tag
    for sys_entry in "${_SYSTEM_ORDER[@]}"; do
        sys_depth=${sys_entry%%:*}; sys_rest=${sys_entry#*:}
        sys_label=${sys_rest%%:*}; sys_rest=${sys_rest#*:}
        sys_ms=${sys_rest%%:*}; sys_tag=${sys_rest#*:}
        [[ "$sys_tag" == "$sys_ms" ]] && sys_tag=""
        sys_timeline_json=$(jq -n \
            --argjson arr "$sys_timeline_json" \
            --argjson depth "$sys_depth" \
            --arg     label "$sys_label" \
            --argjson ms "$sys_ms" \
            --arg     tag "$sys_tag" \
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
        --argjson system_timeline     "$sys_timeline_json" \
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
            system_timeline:  $system_timeline,
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
        ((.system_timeline // []) | map(select(.label == "path_helper_fork")) | .[0].ms // .ms.etc_path_helper // 0) as $ph |
        ((.system_timeline // []) | map(select(.label == "combining/locale")) | .[0].ms // .ms.etc_zshrc_locale // 0) as $loc |
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

# Regenerate the path_helper cache. Run this after installing software that
# modifies /etc/paths.d (e.g. Homebrew, Xcode CLT, new system tools).
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
