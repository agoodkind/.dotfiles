#!/usr/bin/env bash

# Terminal progress display utilities extracted from docker/buildkit
# Provides docker-style collapsible verbose output for build steps

# ANSI escape codes
readonly ESC=$'\033'
readonly CURSOR_UP="${ESC}[A"
readonly CURSOR_DOWN="${ESC}[B"
readonly CURSOR_HIDE="${ESC}[?25l"
readonly CURSOR_SHOW="${ESC}[?25h"
readonly ERASE_LINE="${ESC}[2K"
readonly TEXT_DIM="${ESC}[2m"
readonly TEXT_RESET="${ESC}[0m"
readonly COLOR_GREEN="${ESC}[32m"
readonly COLOR_RED="${ESC}[31m"
readonly COLOR_YELLOW="${ESC}[33m"

# Track line count for cleanup
_PROGRESS_LINE_COUNT=0
_PROGRESS_STEP_START=0
_PROGRESS_LOG_FILE=""

# Initialize display (hide cursor)
function progress_init() {
    local log_file="${1:-}"
    if [[ -n "$log_file" ]]; then
        _PROGRESS_LOG_FILE="$log_file"
        mkdir -p "$(dirname "$log_file")"
        echo "=== Progress Log Started: $(date) ===" > "$_PROGRESS_LOG_FILE"
    fi
    echo -ne "${CURSOR_HIDE}"
}

# EXIT trap: restore cursor, print log path on error.
# Usage: progress_set_log_file /path/to/log; trap progress_on_exit_trap EXIT
function progress_on_exit_trap() {
    local ec=$?
    # Temporarily disable set -e to ensure we finish the trap
    set +e
    progress_done
    if [[ $ec -ne 0 ]]; then
        if [[ -n "${_PROGRESS_LOG_FILE:-}" ]] && [[ -f "${_PROGRESS_LOG_FILE:-}" ]]; then
            echo -e "\n${COLOR_RED}Full log: ${_PROGRESS_LOG_FILE}${TEXT_RESET}" >&2
        else
            echo -e "\n${COLOR_RED}Sync failed with exit code $ec (no log file found)${TEXT_RESET}" >&2
        fi
    fi
    trap - EXIT
    exit $ec
}

# Set log file only (no cursor change). Use when you want file logging without init.
function progress_set_log_file() {
    local log_file="$1"
    if [[ -n "$log_file" ]]; then
        _PROGRESS_LOG_FILE="$log_file"
        mkdir -p "$(dirname "$log_file")"
        echo "=== Progress Log Started: $(date) ===" > "$_PROGRESS_LOG_FILE"
    fi
}

# Log to file (strips ANSI codes)
function progress_log() {
    if [[ -n "${_PROGRESS_LOG_FILE:-}" ]]; then
        # Use a more portable way to strip ANSI codes that doesn't depend on BSD vs GNU sed differences
        # or just log as is if sed is being problematic, but we'll try to keep it simple
        echo "$*" | sed 's/\[[0-9;]*m//g' >> "$_PROGRESS_LOG_FILE" 2>/dev/null || true
    fi
}

# Cleanup display (show cursor, clear tracking)
function progress_done() {
    echo -ne "${CURSOR_SHOW}"
    _PROGRESS_LINE_COUNT=0
}

# Clear N lines above current position
function progress_clear_lines() {
    local count=$1
    local i
    for ((i=0; i<count; i++)); do
        echo -ne "${CURSOR_UP}${ERASE_LINE}"
    done
}

# Print step header
function progress_step() {
    local step_name="$1"
    local step_num="${2:-}"

    _PROGRESS_STEP_START=$(date +%s)

    local header
    if [[ -n "$step_num" ]]; then
        header="[+] Step ${step_num}: ${step_name}"
    else
        header="[+] ${step_name}"
    fi

    echo "$header"
    progress_log "$header"
}

# Live streaming version (docker-style)
function progress_exec_stream() {
    local buffer=()
    local line_count=0
    local exit_code=-1
    local exit_token="PROGRESS_EXIT_CODE_$(date +%s)_$RANDOM"

    # In CI/Non-interactive/Non-TTY, just stream output linearly without cursor magic
    if [[ "${GITHUB_ACTIONS:-}" == "true" ]] || [[ "${CI:-}" == "true" ]] || [[ ! -t 1 ]]; then
        while IFS= read -r line || [[ -n "$line" ]]; do
            if [[ "$line" == "$exit_token:"* ]]; then
                exit_code="${line#$exit_token:}"
                continue
            fi
            echo "  $line"
            progress_log "$line"
        done < <(set +e; "$@" 2>&1; printf "\n%s:%d\n" "$exit_token" $?)

        # Fallback if for some reason we didn't get the exit code
        [[ "$exit_code" == "-1" ]] && exit_code=1

        if [[ $exit_code -eq 0 ]]; then
            echo -e "${COLOR_GREEN}  ✓ Completed${TEXT_RESET}"
            progress_log "  ✓ Completed"
        else
            echo -e "${COLOR_RED}  ✗ Failed (exit code: $exit_code)${TEXT_RESET}"
            echo -e "${COLOR_RED}  Command: $*${TEXT_RESET}"
            progress_log "  ✗ Failed (exit code: $exit_code)"
            progress_log "  Command: $*"
        fi
        return $exit_code
    fi

    # TTY Mode: Fixed height scrolling window
    local max_height=10  # Maximum lines to show at once
    local visible_lines=0

    # Disable set -e in the subshell so we can capture the exit code
    # We append the exit code to the stream to avoid race conditions with temp files
    while IFS= read -r line || [[ -n "$line" ]]; do
        if [[ "$line" == "$exit_token:"* ]]; then
            exit_code="${line#$exit_token:}"
            continue
        fi

        # Log full output to file
        progress_log "$line"
        ((line_count++)) || true

        # Buffer logic for display
        buffer+=("$line")
        # Keep buffer size manageable (but larger than max_height)
        if [[ ${#buffer[@]} -gt 50 ]]; then
            buffer=("${buffer[@]:1}")
        fi

        # --- Display Update (Fixed Height Scrolling) ---

        # 1. Determine lines to show (last N lines)
        local display_start=0
        if [[ ${#buffer[@]} -gt $max_height ]]; then
            display_start=$(( ${#buffer[@]} - max_height ))
        fi
        local lines_to_print=("${buffer[@]:$display_start}")
        local new_visible_count=${#lines_to_print[@]}

        # 2. Move cursor to top of the visible block
        if [[ $visible_lines -gt 0 ]]; then
            echo -ne "${ESC}[${visible_lines}A"
        fi

        # 3. Print the window
        for l in "${lines_to_print[@]}"; do
            echo -ne "${ERASE_LINE}"
            echo -e "${TEXT_DIM}  ${l}${TEXT_RESET}"
        done

        # 4. Update visible count
        visible_lines=$new_visible_count

    done < <(set +e; "$@" 2>&1; printf "\n%s:%d\n" "$exit_token" $?)

    # Fallback if for some reason we didn't get the exit code
    [[ "$exit_code" == "-1" ]] && exit_code=1

    # Clear verbose output only on success; keep it visible on failure
    if [[ $exit_code -eq 0 ]] && [[ $visible_lines -gt 0 ]]; then
        progress_clear_lines $visible_lines
    fi

    # Show result
    local duration=$(($(date +%s) - _PROGRESS_STEP_START))
    local summary
    if [[ $exit_code -eq 0 ]]; then
        summary="  ✓ Completed in ${duration}s"
        echo -e "${COLOR_GREEN}${summary}${TEXT_RESET}"
    else
        summary="  ✗ Failed in ${duration}s (exit code: $exit_code)"
        echo -e "${COLOR_RED}${summary}${TEXT_RESET}"
        echo -e "${COLOR_RED}  Command: $*${TEXT_RESET}"
        progress_log "  Command: $*"
    fi
    progress_log "$summary"

    return $exit_code
}
