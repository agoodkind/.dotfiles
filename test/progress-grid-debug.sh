#!/usr/bin/env bash
set -euo pipefail

source "${HOME}/.dotfiles/bash/core/progress.bash"

echo "=== Grid Display Debug ==="
echo ""

# Create temp dir for grid status files
tmp_dir=$(mktemp -d)
echo "Grid temp dir: $tmp_dir"

progress_begin "/tmp/progress-grid-debug.log"
echo "_PROGRESS_STATE_DIR: ${_PROGRESS_STATE_DIR:-EMPTY}"
echo ""

echo "Starting grid with 3 workers..."
progress_grid_start "$tmp_dir" 3
echo "Grid files created:"
ls -la "${_PROGRESS_STATE_DIR}/" 2>/dev/null || echo "  (none)"
echo ""

echo "Simulating workers (you should see grid updating)..."
echo ""

# Worker 1 starts
sleep 0.5
echo "active|Worker-1|processing" > "$tmp_dir/1.status"

sleep 1
# Worker 1 completes, Worker 2 starts
echo "ok|Worker-1|done" > "$tmp_dir/1.status"
echo "active|Worker-2|processing" > "$tmp_dir/2.status"

sleep 1
# Worker 2 completes, Worker 3 starts
echo "ok|Worker-2|done" > "$tmp_dir/2.status"
echo "active|Worker-3|processing" > "$tmp_dir/3.status"

sleep 1
# Worker 3 completes
echo "ok|Worker-3|done" > "$tmp_dir/3.status"

sleep 0.5
echo ""
echo "All workers done, ending grid..."
progress_grid_done

echo ""
echo "Ending progress session..."
progress_end

rm -rf "$tmp_dir"

echo ""
echo "=== Grid debug complete ==="
