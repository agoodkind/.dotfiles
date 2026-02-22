#!/usr/bin/env bash
# Reactive BuildKit-style progress engine: state files + background display loop + EXIT trap.

if [[ -z "${ESC:-}" ]]; then
    readonly ESC=$'\033'
    readonly CURSOR_UP="${ESC}[A"
    readonly CURSOR_DOWN="${ESC}[B"
    readonly CURSOR_HIDE="${ESC}[?25l"
    readonly CURSOR_SHOW="${ESC}[?25h"
    readonly ERASE_LINE="${ESC}[2K"
    readonly CURSOR_SAVE="${ESC}[s"
    readonly CURSOR_RESTORE="${ESC}[u"
    readonly TEXT_DIM="${ESC}[2m"
    readonly TEXT_RESET="${ESC}[0m"
    readonly COLOR_GREEN="${ESC}[32m"
    readonly COLOR_RED="${ESC}[31m"
    readonly COLOR_YELLOW="${ESC}[33m"
fi

_PROGRESS_LOG_FILE=""
_PROGRESS_STATE_DIR=""
_PROGRESS_DISPLAY_PID=""
_PROGRESS_SESSION_DEPTH=0
_PROGRESS_GRID_MODE=false
_PROGRESS_GRID_TMP_DIR=""
_PROGRESS_GRID_TOTAL=0
_PROGRESS_GRID_FORMAT_FN=""
_PROGRESS_GRID_STATE_DIR=""
_PROGRESS_GRID_DISPLAY_PID=""
_PROGRESS_TERM_ROWS=24
_PROGRESS_TERM_COLS=80

function _progress_term_size() {
    local size
    size=$(stty size < /dev/tty 2>/dev/null) || size="24 80"
    _PROGRESS_TERM_ROWS="${size%% *}"
    _PROGRESS_TERM_COLS="${size##* }"
}

function _progress_is_tty() {
    [[ "${PROGRESS_NO_TTY:-}" == "1" ]] && return 1
    [[ "${GITHUB_ACTIONS:-}" == "true" ]] && return 1
    [[ "${CI:-}" == "true" ]] && return 1
    [[ -c /dev/tty ]] && return 0
    return 1
}

function _progress_state_dir_create() {
    _PROGRESS_STATE_DIR=$(mktemp -d 2>/dev/null || echo "/tmp/progress-$$-$RANDOM")
    mkdir -p "$_PROGRESS_STATE_DIR"
}

function _progress_state_dir_cleanup() {
    [[ -n "${_PROGRESS_STATE_DIR:-}" ]] && [[ -d "$_PROGRESS_STATE_DIR" ]] && rm -rf "$_PROGRESS_STATE_DIR"
    _PROGRESS_STATE_DIR=""
}

function _progress_vertex_write() {
    local n="$1"
    local status="$2"
    local label="$3"
    local ts="${4:-$(date +%s)}"
    local detail="${5:-}"
    printf '%s|%s|%s|%s\n' "$status" "$label" "$ts" "$detail" > "${_PROGRESS_STATE_DIR}/${n}.vertex"
}

function progress_vertex_start() {
    local label="$1"
    [[ -z "${_PROGRESS_STATE_DIR:-}" ]] && _progress_state_dir_create
    local counter_file="${_PROGRESS_STATE_DIR}/.counter"
    local n=1
    if [[ -f "$counter_file" ]]; then
        n=$(cat "$counter_file")
        ((n++)) || true
    fi
    echo "$n" > "$counter_file"
    _progress_vertex_write "$n" "started" "$label"
    if [[ -n "${_PROGRESS_LOG_FILE:-}" ]]; then
        echo "$(date +%Y-%m-%dT%H:%M:%S) [vertex $n] started: $label" >> "$_PROGRESS_LOG_FILE"
    fi
    echo $n
}

function progress_vertex_complete() {
    local n="$1"
    local label="${2:-}"
    local detail=""
    if [[ -f "${_PROGRESS_STATE_DIR}/${n}.vertex" ]]; then
        local _c; _c=$(<"${_PROGRESS_STATE_DIR}/${n}.vertex")
        local _r="${_c#*|}"
        [[ -z "$label" ]] && label="${_r%%|*}"
        local _r2="${_r#*|}"; local _r3="${_r2#*|}"
        detail="${_r3%%|*}"
    fi
    _progress_vertex_write "$n" "completed" "$label" "" "$detail"
    if [[ -n "${_PROGRESS_LOG_FILE:-}" ]]; then
        echo "$(date +%Y-%m-%dT%H:%M:%S) [vertex $n] completed: $label" >> "$_PROGRESS_LOG_FILE"
    fi
}

function progress_vertex_error() {
    local n="$1"
    local label="${2:-}"
    local detail=""
    if [[ -f "${_PROGRESS_STATE_DIR}/${n}.vertex" ]]; then
        local _c; _c=$(<"${_PROGRESS_STATE_DIR}/${n}.vertex")
        local _r="${_c#*|}"
        [[ -z "$label" ]] && label="${_r%%|*}"
        local _r2="${_r#*|}"; local _r3="${_r2#*|}"
        detail="${_r3%%|*}"
    fi
    _progress_vertex_write "$n" "error" "$label" "" "$detail"
    if [[ -n "${_PROGRESS_LOG_FILE:-}" ]]; then
        echo "$(date +%Y-%m-%dT%H:%M:%S) [vertex $n] error: $label" >> "$_PROGRESS_LOG_FILE"
    fi
}

function progress_vertex_cached() {
    local n="$1"
    local label="${2:-}"
    local detail=""
    if [[ -f "${_PROGRESS_STATE_DIR}/${n}.vertex" ]]; then
        local _c; _c=$(<"${_PROGRESS_STATE_DIR}/${n}.vertex")
        local _r="${_c#*|}"
        [[ -z "$label" ]] && label="${_r%%|*}"
        local _r2="${_r#*|}"; local _r3="${_r2#*|}"
        detail="${_r3%%|*}"
    fi
    _progress_vertex_write "$n" "cached" "$label" "" "$detail"
    if [[ -n "${_PROGRESS_LOG_FILE:-}" ]]; then
        echo "$(date +%Y-%m-%dT%H:%M:%S) [vertex $n] cached: $label" >> "$_PROGRESS_LOG_FILE"
    fi
}

function progress_vertex_detail() {
    local n="$1"
    local detail="$2"
    local f="${_PROGRESS_STATE_DIR}/${n}.vertex"
    [[ ! -f "$f" ]] && return 0
    local content
    content=$(<"$f")
    local status="${content%%|*}"
    local rest="${content#*|}"
    local label="${rest%%|*}"
    local rest2="${rest#*|}"
    local ts="${rest2%%|*}"
    _progress_vertex_write "$n" "$status" "$label" "$ts" "$detail"
    if [[ -n "${_PROGRESS_LOG_FILE:-}" ]]; then
        echo "$(date +%Y-%m-%dT%H:%M:%S) [vertex $n] detail: $detail" >> "$_PROGRESS_LOG_FILE"
    fi
}

function _progress_render_vertex_line() {
    local status="$1"
    local label="$2"
    local detail="${3:-}"
    local suffix=""
    [[ -n "$detail" ]] && suffix="  ${TEXT_DIM}(${detail})${TEXT_RESET}"
    case "$status" in
        started)   printf '[-] %s%s\n' "$label" "$suffix" ;;
        completed) printf '%s[+] %s%s%s\n' "$COLOR_GREEN" "$label" "$TEXT_RESET" "$suffix" ;;
        error)     printf '%s[x] %s%s%s\n' "$COLOR_RED" "$label" "$TEXT_RESET" "$suffix" ;;
        cached)    printf '%s[+] %s (cached)%s%s\n' "$COLOR_GREEN" "$label" "$TEXT_RESET" "$suffix" ;;
        *)         printf '[-] %s%s\n' "$label" "$suffix" ;;
    esac
}

_PROGRESS_OUTPUT_COUNT=0

function _progress_render_output_lines() {
    local out_file="$1"
    local max_lines="$2"
    local max_width=$(( ${_PROGRESS_TERM_COLS:-80} - 6 ))
    [[ $max_width -lt 10 ]] && max_width=10
    _PROGRESS_OUTPUT_COUNT=0
    [[ ! -f "$out_file" ]] && return 0
    local line
    while IFS= read -r line; do
        line="${line//$'\r'/}"
        [[ ${#line} -gt $max_width ]] && line="${line:0:$max_width}"
        printf '%s    %s%s%s\n' "$ERASE_LINE" "$TEXT_DIM" "$line" "$TEXT_RESET" >/dev/tty
        ((_PROGRESS_OUTPUT_COUNT++)) || true
    done < <(tail -n "$max_lines" "$out_file" 2>/dev/null)
}

function _progress_render_grid_line() {
    local status="$1"
    local name="$2"
    local detail="$3"
    local cols="${_PROGRESS_TERM_COLS:-80}"
    local prefix_len=30
    local max_detail=$(( cols - prefix_len - 1 ))
    [[ $max_detail -lt 4 ]] && max_detail=4
    [[ ${#detail} -gt $max_detail ]] && detail="${detail:0:$max_detail}"
    case "$status" in
        pending) printf "  ${TEXT_DIM}%s${TEXT_RESET} %-25s ${TEXT_DIM}%s${TEXT_RESET}\n" $'\u00b7' "$name" "$detail" ;;
        active)  printf "  ${COLOR_YELLOW}%s${TEXT_RESET} %-25s %s\n" $'\u25cb' "$name" "$detail" ;;
        ok)      printf "  ${COLOR_GREEN}%s${TEXT_RESET} %-25s %s\n" $'\u2713' "$name" "$detail" ;;
        error)   printf "  ${COLOR_RED}%s${TEXT_RESET} %-25s %s\n" $'\u2717' "$name" "$detail" ;;
        *)       printf "  ${TEXT_DIM}%s${TEXT_RESET} %-25s %s\n" $'\u00b7' "$name" "$detail" ;;
    esac
}

function _progress_render_grid() {
    local grid_tmp="$1"
    local grid_total="$2"
    local i=1
    for (( i=1; i <= grid_total; i++ )); do
        local content=""
        [[ -f "$grid_tmp/${i}.status" ]] && content=$(cat "$grid_tmp/${i}.status" 2>/dev/null) || true
        local status="${content%%|*}"
        local rest="${content#*|}"
        local name="${rest%%|*}"
        local detail="${rest#*|}"
        if [[ "$status" == "ok" ]] && [[ "$detail" == *"|"* ]]; then
            detail="${detail#*|}"
            [[ "$detail" == *"|"* ]] && detail="${detail#*|}"
        fi
        [[ -z "$status" ]] && status="pending"
        [[ -z "$name" ]] && name="..."
        [[ -z "$detail" ]] && detail=""
        _progress_render_grid_line "$status" "$name" "$detail"
    done
}

function _progress_grid_loop() {
    local state_dir="$1"
    local grid_tmp grid_total
    grid_tmp=$(cat "${state_dir}/.grid_tmp" 2>/dev/null) || return 0
    grid_total=$(cat "${state_dir}/.grid_total" 2>/dev/null) || return 0
    printf '%s' "$CURSOR_HIDE" >/dev/tty
    local prev_content="" prev_lines=0
    while [[ -f "${state_dir}/.grid" ]]; do
        local content=""
        local i=1
        for (( i=1; i <= grid_total; i++ )); do
            content+=$(cat "$grid_tmp/${i}.status" 2>/dev/null) || true
            content+="|"
        done
        if [[ "$content" != "$prev_content" ]]; then
            local j=0
            for (( j=0; j<prev_lines; j++ )); do
                printf '\r%s%s' "$ERASE_LINE" "$CURSOR_UP" >/dev/tty
            done
            [[ $prev_lines -gt 0 ]] && printf '\r%s' "$ERASE_LINE" >/dev/tty
            _progress_render_grid "$grid_tmp" "$grid_total" >/dev/tty
            prev_lines=$grid_total
            prev_content="$content"
        fi
        sleep 0.1
    done
    local j=0
    for (( j=0; j<prev_lines; j++ )); do
        printf '\r%s%s' "$ERASE_LINE" "$CURSOR_UP" >/dev/tty
    done
    [[ $prev_lines -gt 0 ]] && printf '\r%s' "$ERASE_LINE" >/dev/tty
    printf '%s' "$CURSOR_SHOW" >/dev/tty
}

function _progress_display_loop() {
    local state_dir="$1"
    local grid_only="${2:-false}"
    local prev_snapshot=""
    local prev_lines=0
    local prev_rows=0
    local prev_cols=0
    printf '%s' "$CURSOR_HIDE" >/dev/tty

    if [[ "$grid_only" == "true" ]]; then
        _progress_grid_loop "$state_dir"
        return 0
    fi

    local grid_content=""

    while [[ ! -f "${state_dir}/.done" ]]; do
        _progress_term_size
        if [[ "$_PROGRESS_TERM_ROWS" != "$prev_rows" || "$_PROGRESS_TERM_COLS" != "$prev_cols" ]]; then
            # Terminal resized: clear conservatively then force full redraw.
            local j=0
            for (( j=0; j<prev_lines; j++ )); do
                printf '\r%s%s' "$ERASE_LINE" "$CURSOR_UP" >/dev/tty
            done
            [[ $prev_lines -gt 0 ]] && printf '\r%s' "$ERASE_LINE" >/dev/tty
            prev_lines=0
            prev_snapshot=""
            prev_rows="$_PROGRESS_TERM_ROWS"
            prev_cols="$_PROGRESS_TERM_COLS"
        fi

        if [[ -f "${state_dir}/.grid" ]]; then
            local grid_tmp grid_total
            grid_tmp=$(cat "${state_dir}/.grid_tmp" 2>/dev/null) || grid_tmp=""
            grid_total=$(cat "${state_dir}/.grid_total" 2>/dev/null) || grid_total=0
            if [[ -n "$grid_tmp" && "$grid_total" -gt 0 ]]; then
                local content=""
                local gi=1
                for (( gi=1; gi <= grid_total; gi++ )); do
                    content+=$(cat "$grid_tmp/${gi}.status" 2>/dev/null) || true
                    content+="|"
                done
                if [[ "$content" != "$grid_content" ]]; then
                    local j=0
                    for (( j=0; j<prev_lines; j++ )); do
                        printf '\r%s%s' "$ERASE_LINE" "$CURSOR_UP" >/dev/tty
                    done
                    [[ $prev_lines -gt 0 ]] && printf '\r%s' "$ERASE_LINE" >/dev/tty
                    _progress_render_grid "$grid_tmp" "$grid_total" >/dev/tty
                    grid_content="$content"
                    prev_lines=$grid_total
                fi
                sleep 0.1
                continue
            fi
        fi

        grid_content=""

        local rendered=()
        local vertex_statuses=()
        local snapshot=""
        local i=1
        while [[ -f "${state_dir}/${i}.vertex" ]]; do
            local content
            content=$(cat "${state_dir}/${i}.vertex" 2>/dev/null) || content=""
            local status="${content%%|*}"
            local rest="${content#*|}"
            local label="${rest%%|*}"
            local rest2="${rest#*|}"; local rest3="${rest2#*|}"
            local detail="${rest3%%|*}"
            snapshot+="${content}|"
            if [[ "$status" == "started" && -f "${state_dir}/${i}.out" ]]; then
                snapshot+="$(tail -c 512 "${state_dir}/${i}.out" 2>/dev/null)"
            fi
            rendered+=("$(_progress_render_vertex_line "$status" "$label" "$detail")")
            vertex_statuses+=("$status")
            ((i++)) || true
        done

        local num=${#rendered[@]}
        if [[ "$snapshot" != "$prev_snapshot" && $num -gt 0 ]]; then
            local j=0
            for (( j=0; j<prev_lines; j++ )); do
                printf '\r%s%s' "$ERASE_LINE" "$CURSOR_UP" >/dev/tty
            done
            [[ $prev_lines -gt 0 ]] && printf '\r%s' "$ERASE_LINE" >/dev/tty

            local total_rendered=0
            local output_max=$(( _PROGRESS_TERM_ROWS - num - 2 ))
            [[ $output_max -lt 3 ]] && output_max=3
            [[ $output_max -gt 15 ]] && output_max=15

            for ((j=0; j<num; j++)); do
                printf '%s\n' "${rendered[$j]}" >/dev/tty
                ((total_rendered++)) || true
                local vid=$(( j + 1 ))
                if [[ "${vertex_statuses[$j]}" == "started" && -f "${state_dir}/${vid}.out" ]]; then
                    _progress_render_output_lines "${state_dir}/${vid}.out" "$output_max"
                    ((total_rendered += _PROGRESS_OUTPUT_COUNT)) || true
                fi
            done
            prev_lines=$total_rendered
            prev_snapshot="$snapshot"
        fi
        sleep 0.1
    done

    local rendered=()
    local i=1
    while [[ -f "${state_dir}/${i}.vertex" ]]; do
        local content
        content=$(cat "${state_dir}/${i}.vertex" 2>/dev/null) || content=""
        local status="${content%%|*}"
        local rest="${content#*|}"
        local label="${rest%%|*}"
        local rest2="${rest#*|}"; local rest3="${rest2#*|}"
        local detail="${rest3%%|*}"
        rendered+=("$(_progress_render_vertex_line "$status" "$label" "$detail")")
        ((i++)) || true
    done
    local num=${#rendered[@]}
    if [[ $num -gt 0 ]]; then
        local j=0
        for (( j=0; j<prev_lines; j++ )); do
            printf '\r%s%s' "$ERASE_LINE" "$CURSOR_UP" >/dev/tty
        done
        [[ $prev_lines -gt 0 ]] && printf '\r%s' "$ERASE_LINE" >/dev/tty
        for ((j=0; j<num; j++)); do
            printf '%s\n' "${rendered[$j]}" >/dev/tty
        done
    fi

    printf '%s' "$CURSOR_SHOW" >/dev/tty
}

function _progress_exit_trap() {
    set +e
    local ec=$?
    if [[ -n "${_PROGRESS_STATE_DIR:-}" ]] && [[ -d "$_PROGRESS_STATE_DIR" ]]; then
        local i=1
        while [[ -f "${_PROGRESS_STATE_DIR}/${i}.vertex" ]]; do
            local content
            content=$(cat "${_PROGRESS_STATE_DIR}/${i}.vertex" 2>/dev/null)
            if [[ "$content" == started* ]]; then
                local _r="${content#*|}"
                local _label="${_r%%|*}"
                local _r2="${_r#*|}"; local _ts="${_r2%%|*}"
                local _r3="${_r2#*|}"; local _detail="${_r3%%|*}"
                _progress_vertex_write "$i" "error" "$_label" "$_ts" "$_detail"
            fi
            ((i++)) || true
        done
        touch "${_PROGRESS_STATE_DIR}/.done"
        [[ -n "${_PROGRESS_DISPLAY_PID:-}" ]] && wait "$_PROGRESS_DISPLAY_PID" 2>/dev/null || true
    fi
    (printf '%s' "$CURSOR_SHOW" >/dev/tty) 2>/dev/null || true
    _progress_state_dir_cleanup
    _PROGRESS_DISPLAY_PID=""
    if [[ $ec -ne 0 ]]; then
        if [[ -n "${_PROGRESS_LOG_FILE:-}" ]] && [[ -f "${_PROGRESS_LOG_FILE:-}" ]]; then
            echo -e "\n${COLOR_RED}Full log: ${_PROGRESS_LOG_FILE}${TEXT_RESET}" >&2
        else
            echo -e "\n${COLOR_RED}Failed with exit code $ec (no log file)${TEXT_RESET}" >&2
        fi
    fi
    trap - EXIT
    exit $ec
}

function progress_begin() {
    ((_PROGRESS_SESSION_DEPTH++)) || true
    (( _PROGRESS_SESSION_DEPTH > 1 )) && return 0
    local log_file="${1:-}"
    if [[ -n "$log_file" ]]; then
        _PROGRESS_LOG_FILE="$log_file"
        mkdir -p "$(dirname "$log_file")"
        echo "=== Progress Log Started: $(date) ===" > "$_PROGRESS_LOG_FILE"
    fi
    _progress_state_dir_create
    if ! _progress_is_tty; then
        trap - EXIT
        return 0
    fi
    _progress_display_loop "$_PROGRESS_STATE_DIR" &
    _PROGRESS_DISPLAY_PID=$!
    trap _progress_exit_trap EXIT
}

function progress_end() {
    set +e
    (( _PROGRESS_SESSION_DEPTH > 0 )) && ((_PROGRESS_SESSION_DEPTH--))
    (( _PROGRESS_SESSION_DEPTH > 0 )) && return 0
    if [[ -n "${_PROGRESS_CURRENT_VERTEX:-}" ]] && [[ -n "${_PROGRESS_STATE_DIR:-}" ]]; then
        progress_vertex_complete "$_PROGRESS_CURRENT_VERTEX" ""
    fi
    if [[ -z "${_PROGRESS_STATE_DIR:-}" ]]; then
        trap - EXIT
        return 0
    fi
    trap - EXIT
    touch "${_PROGRESS_STATE_DIR}/.done"
    [[ -n "${_PROGRESS_DISPLAY_PID:-}" ]] && wait "$_PROGRESS_DISPLAY_PID" 2>/dev/null || true
    (printf '%s' "$CURSOR_SHOW" >/dev/tty) 2>/dev/null || true
    _progress_state_dir_cleanup
    _PROGRESS_DISPLAY_PID=""
}

function progress_set_log_file() {
    local log_file="$1"
    if [[ -n "$log_file" ]]; then
        _PROGRESS_LOG_FILE="$log_file"
        mkdir -p "$(dirname "$log_file")"
        echo "=== Progress Log Started: $(date) ===" > "$_PROGRESS_LOG_FILE"
    fi
}

function progress_log() {
    if [[ -n "${_PROGRESS_LOG_FILE:-}" ]]; then
        # Use printf to handle ANSI escapes correctly without expanding them
        printf "%b\n" "$*" | sed 's/\x1b\[[0-9;]*m//g' >> "$_PROGRESS_LOG_FILE" 2>/dev/null || true
    fi
}

function progress_on_exit_trap() {
    _progress_exit_trap
}

function _progress_term_cols() {
    local cols="${COLUMNS:-0}"
    [[ "$cols" -lt 10 ]] && cols=$(tput cols 2>/dev/null) || cols=80
    echo $((cols > 4 ? cols - 4 : 76))
}

function progress_clear_lines() {
    local count=$1
    local i
    for ((i=0; i<count; i++)); do
        echo -ne "${CURSOR_UP}${ERASE_LINE}"
    done
}

# --- progress_vertex_exec: run command, stream output, update vertex ---
function progress_vertex_exec() {
    local label="$1"
    shift
    [[ -z "${_PROGRESS_STATE_DIR:-}" ]] && _progress_state_dir_create
    local n
    n=$(progress_vertex_start "$label")
    local out_file="${_PROGRESS_STATE_DIR}/${n}.out"
    : > "$out_file"
    local exit_code=0
    local exit_token="PROGRESS_EXIT_$(date +%s)_$RANDOM"
    if ! _progress_is_tty; then
        set +e
        while IFS= read -r line || [[ -n "$line" ]]; do
            if [[ "$line" == "$exit_token:"* ]]; then
                exit_code="${line#$exit_token:}"
                continue
            fi
            progress_log "$line"
            line="${line##*$'\r'}"
            line="${line//$'\r'/}"
            echo "  $line"
        done < <("$@" 2>&1; printf "\n%s:%d\n" "$exit_token" $?)
        [[ "$exit_code" == "-1" ]] && exit_code=1
        if [[ $exit_code -eq 0 ]]; then
            progress_vertex_complete "$n" "$label"
            echo -e "${COLOR_GREEN}  + Completed${TEXT_RESET}"
            progress_log "  + Completed"
        else
            progress_vertex_error "$n" "$label"
            echo -e "${COLOR_RED}  x Failed (exit code: $exit_code)${TEXT_RESET}"
            echo -e "${COLOR_RED}  Command: $*${TEXT_RESET}"
            progress_log "  x Failed (exit code: $exit_code)"
            progress_log "  Command: $*"
        fi
        return $exit_code
    fi
    set +e
    ("$@" 2>&1; printf "\n%s:%d\n" "$exit_token" $?) | while IFS= read -r line || [[ -n "$line" ]]; do
        if [[ "$line" == "$exit_token:"* ]]; then
            echo "$line" >> "$out_file"
            continue
        fi
        progress_log "$line"
        echo "$line" >> "$out_file"
    done
    exit_code=1
    if [[ -f "$out_file" ]]; then
        while IFS= read -r line; do
            [[ "$line" == "$exit_token:"* ]] && exit_code="${line#$exit_token:}"
        done < "$out_file"
    fi
    [[ "$exit_code" == "-1" ]] && exit_code=1
    if [[ $exit_code -eq 0 ]]; then
        progress_vertex_complete "$n" "$label"
        progress_log "  + Completed"
    else
        progress_vertex_error "$n" "$label"
        progress_log "  x Failed (exit code: $exit_code)"
        progress_log "  Command: $*"
    fi
    return $exit_code
}

function progress_grid_start() {
    local tmp_dir="$1"
    local total="$2"
    _PROGRESS_GRID_MODE=true
    _PROGRESS_GRID_TMP_DIR="$tmp_dir"
    _PROGRESS_GRID_TOTAL="$total"

    if [[ -n "${_PROGRESS_STATE_DIR:-}" ]] && [[ -d "$_PROGRESS_STATE_DIR" ]]; then
        printf '%s\n' "$tmp_dir" > "${_PROGRESS_STATE_DIR}/.grid_tmp"
        printf '%s\n' "$total" > "${_PROGRESS_STATE_DIR}/.grid_total"
        touch "${_PROGRESS_STATE_DIR}/.grid"
        return 0
    fi

    _PROGRESS_GRID_STATE_DIR=$(mktemp -d 2>/dev/null || echo "/tmp/progress-grid-$$-$RANDOM")
    mkdir -p "$_PROGRESS_GRID_STATE_DIR"
    printf '%s\n' "$tmp_dir" > "${_PROGRESS_GRID_STATE_DIR}/.grid_tmp"
    printf '%s\n' "$total" > "${_PROGRESS_GRID_STATE_DIR}/.grid_total"
    touch "${_PROGRESS_GRID_STATE_DIR}/.grid"
    if _progress_is_tty; then
        _progress_grid_loop "$_PROGRESS_GRID_STATE_DIR" &
        _PROGRESS_GRID_DISPLAY_PID=$!
    fi
}

function progress_grid_done() {
    local state_dir="${_PROGRESS_STATE_DIR:-${_PROGRESS_GRID_STATE_DIR:-}}"
    if [[ -n "$state_dir" ]] && [[ -f "${state_dir}/.grid" ]]; then
        rm -f "${state_dir}/.grid"
    fi
    if [[ -n "${_PROGRESS_GRID_DISPLAY_PID:-}" ]]; then
        wait "$_PROGRESS_GRID_DISPLAY_PID" 2>/dev/null || true
        _PROGRESS_GRID_DISPLAY_PID=""
    fi
    if [[ -n "${_PROGRESS_GRID_STATE_DIR:-}" ]] && [[ -d "$_PROGRESS_GRID_STATE_DIR" ]]; then
        rm -rf "$_PROGRESS_GRID_STATE_DIR"
        _PROGRESS_GRID_STATE_DIR=""
    fi
    _PROGRESS_GRID_MODE=false
    _PROGRESS_GRID_TMP_DIR=""
    _PROGRESS_GRID_TOTAL=0
    _PROGRESS_GRID_FORMAT_FN=""
}

