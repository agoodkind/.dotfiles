#!/usr/bin/env bash
set -euo pipefail

source "${HOME}/.dotfiles/bash/core/progress.bash"

echo "=== Progress Display Manual Test ==="
echo ""

# Test 1: TTY detection
echo "Test 1: TTY Detection"
echo "  [[ -t 1 ]] = $([[ -t 1 ]] && echo "true" || echo "false")"
echo "  /dev/tty exists = $([[ -c /dev/tty ]] && echo "true" || echo "false")"
echo "  _progress_is_tty returns: $(_progress_is_tty && echo "0 (TTY)" || echo "1 (not TTY)")"
echo ""

# Test 2: State directory creation
echo "Test 2: State Directory"
_progress_state_dir_create
echo "  _PROGRESS_STATE_DIR = ${_PROGRESS_STATE_DIR:-EMPTY}"
echo "  Directory exists = $([[ -d "${_PROGRESS_STATE_DIR:-}" ]] && echo "true" || echo "false")"
_progress_state_dir_cleanup
echo ""

# Test 3: Vertex file creation and counter
echo "Test 3: Vertex Files and Counter"
_progress_state_dir_create
vid=$(progress_vertex_start "Test vertex 1")
echo "  Vertex ID returned: $vid"
echo "  Vertex file exists: $([[ -f "${_PROGRESS_STATE_DIR}/${vid}.vertex" ]] && echo "true" || echo "false")"
[[ -f "${_PROGRESS_STATE_DIR}/${vid}.vertex" ]] && echo "  Content: $(cat "${_PROGRESS_STATE_DIR}/${vid}.vertex")"
counter_val=$(cat "${_PROGRESS_STATE_DIR}/.counter" 2>/dev/null || echo "MISSING")
echo "  Counter file value: $counter_val"

vid2=$(progress_vertex_start "Test vertex 2")
echo "  Second vertex ID: $vid2"
counter_val=$(cat "${_PROGRESS_STATE_DIR}/.counter" 2>/dev/null || echo "MISSING")
echo "  Counter file value after second: $counter_val"
_progress_state_dir_cleanup
echo ""

# Test 4: Display loop (visual test)
echo "Test 4: Display Loop (3 second visual test)"
echo "  Starting progress_begin..."
progress_begin "/tmp/progress-test.log"
echo "  _PROGRESS_DISPLAY_PID = ${_PROGRESS_DISPLAY_PID:-EMPTY}"

echo ""
echo "  Creating vertex and sleeping 2s - you should see [-] marker:"
test_vid=$(progress_vertex_start "Simulated work")

sleep 2
echo "  (2s elapsed, completing vertex...)"
progress_vertex_complete "$test_vid" "Simulated work done"

sleep 0.5
echo "  (calling progress_end...)"
progress_end

echo ""
echo "=== Test Complete ==="
echo "Log file: /tmp/progress-test.log"
if [[ -f /tmp/progress-test.log ]]; then
    echo "--- log contents ---"
    cat /tmp/progress-test.log
fi
