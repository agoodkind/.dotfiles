#!/usr/bin/env bash
set -euo pipefail

source "${HOME}/.dotfiles/bash/core/progress.bash"

echo "=== Progress Display Debug ==="
echo ""
echo "1. TTY detection:"
echo "   /dev/tty exists: $([[ -c /dev/tty ]] && echo YES || echo NO)"
echo "   _progress_is_tty: $(_progress_is_tty && echo "YES (TTY mode)" || echo "NO (non-TTY mode)")"
echo ""

echo "2. Starting progress session..."
progress_begin "/tmp/progress-debug.log"
echo "   _PROGRESS_STATE_DIR: ${_PROGRESS_STATE_DIR:-EMPTY}"
echo "   _PROGRESS_DISPLAY_PID: ${_PROGRESS_DISPLAY_PID:-EMPTY}"
echo ""

echo "3. Creating vertex (you should see [-] appear below)..."
sleep 0.5
vid=$(progress_vertex_start "Test task running")
echo "   Vertex ID: $vid"
echo "   Vertex file exists: $([[ -f "${_PROGRESS_STATE_DIR}/${vid}.vertex" ]] && echo YES || echo NO)"
[[ -f "${_PROGRESS_STATE_DIR}/${vid}.vertex" ]] && echo "   Vertex content: $(cat "${_PROGRESS_STATE_DIR}/${vid}.vertex")"
echo ""

echo "4. Sleeping 3 seconds (display loop should be rendering)..."
sleep 3

echo ""
echo "5. Completing vertex (you should see [+] appear)..."
progress_vertex_complete "$vid" "Test task done"
sleep 0.5

echo ""
echo "6. Ending progress session..."
progress_end

echo ""
echo "=== Debug complete ==="
echo "Log file: /tmp/progress-debug.log"
echo ""
echo "Log contents:"
cat /tmp/progress-debug.log
