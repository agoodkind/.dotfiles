#!/usr/bin/env bash
set -euo pipefail

echo "Test 1: Direct /dev/tty write"
printf "  Writing directly to /dev/tty: " 
printf "HELLO\n" >/dev/tty
echo "  (did you see HELLO above?)"
echo ""

echo "Test 2: Background process writing to /dev/tty"
(
    sleep 0.5
    printf "BACKGROUND\n" >/dev/tty
) &
pid=$!
echo "  Started background process $pid, waiting 1s..."
sleep 1
wait $pid 2>/dev/null || true
echo "  (did you see BACKGROUND above?)"
echo ""

echo "Test 3: Check if display loop is entering grid code"
source "${HOME}/.dotfiles/bash/core/progress.bash"

tmp_dir=$(mktemp -d)
progress_begin "/tmp/progress-minimal.log"

# Add debug marker
printf "DEBUG: About to start grid\n" >/dev/tty

progress_grid_start "$tmp_dir" 2
echo "pending|Test1|waiting" > "$tmp_dir/1.status"
echo "pending|Test2|waiting" > "$tmp_dir/2.status"

printf "DEBUG: Grid started, sleeping 2s\n" >/dev/tty
sleep 2

progress_grid_done
progress_end

rm -rf "$tmp_dir"
echo ""
echo "Minimal test complete"
