# Fallback: if .zshenv armed zprof but failed to load it, load now.
if ((_ZPROF_ARMED != 0 && _ZPROF_LOADED == 0)); then
    zmodload zsh/zprof 2>/dev/null && _ZPROF_LOADED=1
fi

source "$DOTDOTFILES/zshrc/core/zsh-shims.zsh"

typeset -gA _PROFILE_TIMES
_PROFILE_TIMES[_pre_zshrc]=$(((EPOCHREALTIME - START_TIME) * 1000))

local zshenv_self=${_PROFILE_TIMES[_zshenv_self]:-0}
local system_time=$((_PROFILE_TIMES[_pre_zshrc] - zshenv_self))
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
    if ! ((_sc_depth >= 2)); then
        continue
    fi
    _sc_rest=${_sc_entry#*:}
    _sc_rest=${_sc_rest#*:}
    _sc_ms=${_sc_rest%%:*}
    _sc_accounted=$((_sc_accounted + _sc_ms))
done
local _sc_remainder=$((system_time - _sc_accounted))
_PERF_TREE+=("2:[gap /etc/zshrc→.zshrc]:${_sc_remainder}")

typeset -gi _SOURCE_DEPTH=0

function _source() {
    local _tail=${1##*/}
    local label=${_tail%.zsh}
    local t0=$EPOCHREALTIME
    ((_SOURCE_DEPTH++))
    source "$1"
    ((_SOURCE_DEPTH--))
    local ms=$(((EPOCHREALTIME - t0) * 1000))
    _PROFILE_TIMES[$label]=$ms
    _PERF_TREE+=("$((_SOURCE_DEPTH + 2)):${label}:${ms}")
}

function _async() {
    local _tail=${1##*/}
    local label=${_tail%.zsh}
    local t0=$EPOCHREALTIME
    ("$@" >/dev/null 2>&1 &)
    local ms=$(((EPOCHREALTIME - t0) * 1000))
    local _ak="${label}:async"
    _PROFILE_TIMES[$_ak]=$ms
    _PERF_TREE+=("$((_SOURCE_DEPTH + 2)):${label}:${ms}:background")
}

function _perf_tty_id() {
    local tty=${TTY##*/}
    echo "${tty//\//-}"
}

typeset -g _ZPROF_OUTPUT=""

function _perf_first_precmd() {
    _PROFILE_TIMES[_first_precmd]=$(((EPOCHREALTIME - START_TIME) * 1000))
    if ((_ZPROF_LOADED != 0)); then
        _ZPROF_OUTPUT=$(zprof 2>/dev/null)
    fi
    precmd_functions=(${precmd_functions:#_perf_first_precmd})
}
precmd_functions=(_perf_first_precmd $precmd_functions)

function _write_startup_log() {
    if [[ ! -t 1 && "${ZSH_PERF:-}" != "1" ]]; then
        return 0
    fi
    (__write_startup_log_impl >/dev/null 2>&1) &|
}
function __write_startup_log_impl() {
    local log_dir=~/.cache/zsh_startup
    mkdir -p "$log_dir"

    local tty_id=$(_perf_tty_id)
    local ts=$(date '+%Y%m%d_%H%M%S')
    local log="$log_dir/${ts}_${tty_id}.json"
    local ready_ms=${_PROFILE_TIMES[_time_to_ready]:-$(((EPOCHREALTIME - START_TIME) * 1000))}

    # Flat sections JSON (simple key -> ms mapping)
    local sections_json="{}"
    local k key
    _zassoc_keys_sorted _PROFILE_TIMES
    for k in "${_ZSH_ARR[@]}"; do
        if [[ "$k" == _* ]]; then
            continue
        fi
        key=${k//:/_}
        sections_json=$(jq -n \
            --argjson obj "$sections_json" \
            --arg key "$key" \
            --argjson val "${_PROFILE_TIMES[$k]}" \
            '$obj + {($key): $val}')
    done

    # Serialize a depth:label:ms[:tag] array to JSON
    _perf_tree_to_json() {
        local -a entries=("${@}")
        local json="[]"
        local entry rest depth label ms tag
        for entry in "${entries[@]}"; do
            depth=${entry%%:*}
            rest=${entry#*:}
            label=${rest%%:*}
            rest=${rest#*:}
            ms=${rest%%:*}
            tag=${rest#*:}
            if [[ "$tag" == "$ms" ]]; then
                tag=""
            fi
            json=$(jq -n \
                --argjson arr "$json" \
                --argjson depth "$depth" \
                --arg label "$label" \
                --argjson ms "$ms" \
                --arg tag "$tag" \
                '$arr + [{depth: $depth, label: $label, ms: $ms} + (if $tag != "" then {tag: $tag} else {} end)]')
        done
        echo "$json"
    }

    local tree_json=$(_perf_tree_to_json "${_PERF_TREE[@]}")
    local deferred_json=$(_perf_tree_to_json "${_PERF_TREE_DEFERRED[@]}")

    # Read syssnap if available
    local snap_file="$log_dir/.syssnap_$$"
    local syssnap_json="null"
    if [[ -f "$snap_file" ]]; then
        syssnap_json=$(jq -Rs '.' "$snap_file" 2>/dev/null || echo "null")
        rm -f "$snap_file"
    fi

    local zprof_json="null"
    if [[ -n "${_ZPROF_OUTPUT:-}" ]]; then
        zprof_json=$(jq -Rs '.' <<<"$_ZPROF_OUTPUT" 2>/dev/null || echo "null")
    fi

    jq -n \
        --arg ts "$(date '+%Y-%m-%d %H:%M:%S %Z')" \
        --arg tty "$tty_id" \
        --argjson pid "$$" \
        --arg term "${TERM_PROGRAM:-unset}" \
        --argjson pre_zshrc "${_PROFILE_TIMES[_pre_zshrc]:-0}" \
        --argjson system_zsh "${_PROFILE_TIMES[_system_zsh]:-0}" \
        --argjson path_helper_done "${_PATH_HELPER_DONE:-0}" \
        --argjson locale_done "${_LOCALE_DONE:-0}" \
        --argjson zshenv_self "${_PROFILE_TIMES[_zshenv_self]:-0}" \
        --argjson time_prompt "${_PROFILE_TIMES[_time_to_prompt]:-0}" \
        --argjson first_precmd "${_PROFILE_TIMES[_first_precmd]:-0}" \
        --argjson time_ready "$ready_ms" \
        --argjson tree "$tree_json" \
        --argjson deferred "$deferred_json" \
        --argjson sections "$sections_json" \
        --argjson syssnap "$syssnap_json" \
        --argjson zprof "$zprof_json" \
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
            deferred:         $deferred,
            sections:         $sections,
            syssnap:          $syssnap,
            zprof:            $zprof
        }' >"$log"

    # Prune oldest beyond 500, in background
    (
        _zglobfiles_mtime "$log_dir" "*.json"
        local -a all=("${_ZSH_ARR[@]}")
        local count=${#all}
        if ((count > 500)); then
            rm -f "${all[@]:500}"
        fi
    ) &|
}

function _dots_exec() {
    command bash -lc 'source "$0" && run_dots_go_command "$@"' "$DOTDOTFILES/dots/bootstrap-go.sh" "$@"
}

function zsh_perf() {
    _dots_exec perf "$@"
}

function zsh_perf_log() {
    _dots_exec perf log "$@"
}

function zsh_perf_history() {
    _dots_exec perf history "$@"
}

function path_cache_rebuild() {
    _dots_exec perf rebuild-path-cache "$@"
}

function zsh_profile() {
    _dots_exec perf arm-zprof "$@"
}

function do_profile() {
    :
}
