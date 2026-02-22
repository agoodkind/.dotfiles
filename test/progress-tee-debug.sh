#!/usr/bin/env bash
set -euo pipefail

source "${HOME}/.dotfiles/bash/core/progress.bash"

echo "=== Progress with tee redirect (like wkm.sh) ==="
echo ""

# Simulate rt_init_log's tee redirect
log_file="/tmp/progress-tee-debug.log"
exec > >(tee -a "$log_file") 2>&1

echo "1. After tee redirect:"
echo "   stdout is now piped through tee"
echo "   _progress_is_tty: $(_progress_is_tty && echo "YES (TTY mode)" || echo "NO (non-TTY mode)")"
echo ""

echo "2. Starting progress session..."
progress_begin "$log_file"
echo "   _PROGRESS_STATE_DIR: ${_PROGRESS_STATE_DIR:-EMPTY}"
echo "   _PROGRESS_DISPLAY_PID: ${_PROGRESS_DISPLAY_PID:-EMPTY}"
echo ""

echo "3. Creating vertex (you should see [-] appear)..."
sleep 0.5
vid=$(progress_vertex_start "Test with tee")

echo "4. Sleeping 3 seconds..."
sleep 3

echo ""
echo "5. Completing vertex..."
progress_vertex_complete "$vid" "Test complete"
sleep 0.5

echo ""
echo "6. Ending progress session..."
progress_end

echo ""
echo "=== Debug complete ==="
echo "Log: $log_file"
