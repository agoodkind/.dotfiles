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
        echo "=== Progress Log Started: $(date) ===" > "$_PROGRESS_LOG_FILE"
    fi
    echo -ne "${CURSOR_HIDE}"
}

# Log to file (strips ANSI codes)
function progress_log() {
    if [[ -n "$_PROGRESS_LOG_FILE" ]]; then
        # Strip ANSI escape codes before logging
        echo "$@" | sed 's/\x1b\[[0-9;]*m//g' >> "$_PROGRESS_LOG_FILE"
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
    local exit_code=0
    local exit_file="/tmp/progress_exit_$$"
    
    # In CI/Non-interactive, just stream output linearly without cursor magic
    if [[ "${GITHUB_ACTIONS:-}" == "true" ]] || [[ "${CI:-}" == "true" ]]; then
        "$@" 2>&1 | while IFS= read -r line; do
            echo "  $line"
            progress_log "$line"
        done
        exit_code=${PIPESTATUS[0]}
        
        if [[ $exit_code -eq 0 ]]; then
            echo -e "${COLOR_GREEN}  ✓ Completed${TEXT_RESET}"
            progress_log "  ✓ Completed"
        else
            echo -e "${COLOR_RED}  ✗ Failed (exit code: $exit_code)${TEXT_RESET}"
            progress_log "  ✗ Failed (exit code: $exit_code)"
        fi
        return $exit_code
    fi
    
    # We use a temp file to capture the exit code because capturing it
    # from process substitution is unreliable across bash versions.
    
    # Disable set -e in the subshell so we can capture the exit code
    while IFS= read -r line; do
        echo -e "${TEXT_DIM}  ${line}${TEXT_RESET}"
        progress_log "$line"
        buffer+=("$line")
        ((line_count++))
        
        # Keep buffer size manageable
        if [[ ${#buffer[@]} -gt 100 ]]; then
            buffer=("${buffer[@]:1}")
        fi
    done < <(set +e; "$@" 2>&1; echo $? > "$exit_file")

    if [[ -f "$exit_file" ]]; then
        exit_code=$(cat "$exit_file")
        rm -f "$exit_file"
    else
        exit_code=1 # Fallback error if something went wrong
    fi

    # Clear verbose output
    if [[ $line_count -gt 0 ]]; then
        progress_clear_lines $line_count
    fi

    # Show result
    local duration=$(($(date +%s) - _PROGRESS_STEP_START))
    local summary
    if [[ $exit_code -eq 0 ]]; then
        summary="  ✓ Completed in ${duration}s"
        echo -e "${COLOR_GREEN}${summary}${TEXT_RESET}"
    else
        summary="  ✗ Failed in ${duration}s"
        echo -e "${COLOR_RED}${summary}${TEXT_RESET}"
        # Show last 10 lines on error so the user can see WHY it failed
        echo -e "${TEXT_DIM}"
        for line in "${buffer[@]: -10}"; do
            echo "  $line"
        done
        echo -e "${TEXT_RESET}"
    fi
    progress_log "$summary"

    return $exit_code
}
