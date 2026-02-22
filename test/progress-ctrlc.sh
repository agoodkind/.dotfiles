#!/usr/bin/env bash
set -euo pipefail

source "${HOME}/.dotfiles/bash/core/progress.bash"

printf "\033[1;36m━━━ Ctrl+C Cleanup Test ━━━\033[0m\n"
printf "This script runs a long progress session.\n"
printf "Your job: press Ctrl+C while it's running.\n"
printf "Then check:\n"
printf "  1. Cursor is visible (not hidden)\n"
printf "  2. No ghost processes: ps aux | grep progress_display\n"
printf "  3. No leftover temp dirs: ls /tmp/progress-* 2>/dev/null\n"
printf "  4. Terminal not corrupted (type normally)\n"
printf "\n"

function run_test() {
    local test_name="$1"
    shift
    printf "\033[33m▶ %s\033[0m\n" "$test_name"
    printf "  (Ctrl+C NOW, or wait 30s for auto-finish)\n\n"

    local pids_before
    pids_before=$(ps -eo pid,comm 2>/dev/null | grep -c "[_]progress_display" || echo 0)

    "$@"

    local pids_after
    pids_after=$(ps -eo pid,comm 2>/dev/null | grep -c "[_]progress_display" || echo 0)

    printf "\n  Display PIDs before: %s, after: %s\n" "$pids_before" "$pids_after"
    if [[ "$pids_after" -gt "$pids_before" ]]; then
        printf "  \033[31m✗ GHOST PROCESS DETECTED\033[0m\n"
    else
        printf "  \033[32m✓ No ghost processes\033[0m\n"
    fi
    printf "\n"
}

function test_vertex_ctrlc() {
    local progress_lib="${HOME}/.dotfiles/bash/core/progress.bash"
    bash --norc --noprofile -c "
        set -euo pipefail
        source '${progress_lib}'
        progress_begin '/tmp/progress-ctrlc-vertex.log'
        vid=\$(progress_vertex_start 'Long running task')
        sleep 30
        progress_vertex_complete \"\$vid\"
        progress_end
    " || true
}

function test_grid_ctrlc() {
    local progress_lib="${HOME}/.dotfiles/bash/core/progress.bash"
    bash --norc --noprofile -c "
        set -euo pipefail
        source '${progress_lib}'
        tmp_dir=\$(mktemp -d)
        progress_begin '/tmp/progress-ctrlc-grid.log'
        progress_grid_start \"\$tmp_dir\" 5
        for i in 1 2 3 4 5; do
            echo \"active|worker-\${i}|processing...\" > \"\$tmp_dir/\${i}.status\"
        done
        sleep 30
        progress_grid_done
        progress_end
        rm -rf \"\$tmp_dir\"
    " || true
}

function test_mixed_ctrlc() {
    local progress_lib="${HOME}/.dotfiles/bash/core/progress.bash"
    bash --norc --noprofile -c "
        set -euo pipefail
        source '${progress_lib}'
        tmp_dir=\$(mktemp -d)
        progress_begin '/tmp/progress-ctrlc-mixed.log'
        vid=\$(progress_vertex_start 'Setup')
        sleep 1
        progress_vertex_complete \"\$vid\"
        progress_grid_start \"\$tmp_dir\" 3
        echo 'active|alpha|running' > \"\$tmp_dir/1.status\"
        echo 'active|bravo|running' > \"\$tmp_dir/2.status\"
        echo 'pending|charlie|queued' > \"\$tmp_dir/3.status\"
        sleep 30
        progress_grid_done
        progress_end
        rm -rf \"\$tmp_dir\"
    " || true
}

PS3="Pick a test (or q to quit): "
select choice in \
    "Vertex (Ctrl+C during single vertex)" \
    "Grid (Ctrl+C during grid display)" \
    "Mixed (vertex then Ctrl+C during grid)" \
    "Run all three sequentially" \
    "Quit"; do
    case $REPLY in
        1) run_test "Ctrl+C during vertex" test_vertex_ctrlc ;;
        2) run_test "Ctrl+C during grid" test_grid_ctrlc ;;
        3) run_test "Ctrl+C during mixed vertex+grid" test_mixed_ctrlc ;;
        4)
            run_test "Ctrl+C during vertex" test_vertex_ctrlc
            run_test "Ctrl+C during grid" test_grid_ctrlc
            run_test "Ctrl+C during mixed vertex+grid" test_mixed_ctrlc
            ;;
        5) break ;;
        *) printf "Invalid choice\n" ;;
    esac
done

printf "\n\033[1;36m━━━ Final Check ━━━\033[0m\n"
printf "Cursor visible? (can you see it blinking?)\n"
printf "Type something to confirm terminal works: "
read -r response
printf "You typed: %s\n" "$response"

local_ghosts=$(ps -eo pid,args 2>/dev/null | grep "[_]progress_display_loop" || true)
if [[ -n "$local_ghosts" ]]; then
    printf "\033[31mGhost display loops found:\033[0m\n%s\n" "$local_ghosts"
else
    printf "\033[32mNo ghost display loops.\033[0m\n"
fi

local_temps=$(ls -d /tmp/progress-ctrlc-*.log 2>/dev/null || true)
if [[ -n "$local_temps" ]]; then
    printf "Leftover temp files: %s\n" "$local_temps"
    rm -f /tmp/progress-ctrlc-*.log
    printf "(cleaned up)\n"
else
    printf "No leftover temp files.\n"
fi
