#!/usr/bin/env bash
# Step 1.2: State dir + vertex file I/O. Run from repo root: bash bash/core/test_progress_state.sh
set -e
source "$(dirname "$0")/progress.bash"
_progress_state_dir_create
n=$(progress_vertex_start "Test vertex")
[[ -f "${_PROGRESS_STATE_DIR}/${n}.vertex" ]] || { echo "FAIL: vertex file missing"; exit 1; }
content=$(cat "${_PROGRESS_STATE_DIR}/${n}.vertex")
[[ "$content" == started* ]] || { echo "FAIL: expected started|... got $content"; exit 1; }
progress_vertex_complete "$n" "Test vertex"
content=$(cat "${_PROGRESS_STATE_DIR}/${n}.vertex")
[[ "$content" == completed* ]] || { echo "FAIL: expected completed|... got $content"; exit 1; }
_progress_state_dir_cleanup
echo "PASS: state + vertex I/O"
