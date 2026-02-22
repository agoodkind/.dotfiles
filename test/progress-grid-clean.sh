#!/usr/bin/env bash
set -euo pipefail

source "${HOME}/.dotfiles/bash/core/progress.bash"

# Create temp dir for grid status files
tmp_dir=$(mktemp -d)

# Start progress session (this starts the display loop)
progress_begin "/tmp/progress-grid-clean.log"

# Start grid mode - from here, display loop should show grid
progress_grid_start "$tmp_dir" 3

# Simulate workers (NO echo/printf during this phase!)
sleep 0.5
echo "active|Worker-1|processing" > "$tmp_dir/1.status"
sleep 1
echo "ok|Worker-1|done" > "$tmp_dir/1.status"
echo "active|Worker-2|processing" > "$tmp_dir/2.status"
sleep 1
echo "ok|Worker-2|done" > "$tmp_dir/2.status"
echo "active|Worker-3|processing" > "$tmp_dir/3.status"
sleep 1
echo "ok|Worker-3|done" > "$tmp_dir/3.status"
sleep 0.5

# End grid mode
progress_grid_done

# End progress session
progress_end

rm -rf "$tmp_dir"

echo ""
echo "Grid test complete"
