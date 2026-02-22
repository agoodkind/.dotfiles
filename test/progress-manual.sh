#!/usr/bin/env bash
set -euo pipefail

source "${HOME}/.dotfiles/bash/core/progress.bash"

START_TEST="${1:-1}"
_CURRENT_TEST=0

PASS=0
FAIL=0
TOTAL=0

function header() {
    local title="$1"
    local num="${title%%.*}"
    _CURRENT_TEST="${num// /}"
    if (( _CURRENT_TEST >= START_TEST )); then
        printf "\n\033[1;36m━━━ %s ━━━\033[0m\n" "$title"
    else
        printf "\n\033[2m━━━ %s (skipped) ━━━\033[0m\n" "$title"
    fi
}

function should_run() {
    (( _CURRENT_TEST >= START_TEST ))
}

function wait_key() {
    printf "\n  \033[2mPress ENTER to continue...\033[0m"
    read -r
}

function check() {
    local desc="$1"
    ((TOTAL++)) || true
    printf "  \033[33m?\033[0m %s [y/n] " "$desc"
    local response
    read -rsn1 response
    echo
    if [[ "$response" == "y" || "$response" == "Y" ]]; then
        ((PASS++)) || true
        printf "  \033[32m✓\033[0m %s\n" "$desc"
    else
        ((FAIL++)) || true
        printf "  \033[31m✗\033[0m %s\n" "$desc"
    fi
}

# ──────────────────────────────────────────────────────────
header "1. Vertex: sequential start → complete"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    progress_begin "/tmp/progress-manual-1.log"
    vid=$(progress_vertex_start "Installing deps")
    sleep 1.5
    progress_vertex_complete "$vid"
    vid2=$(progress_vertex_start "Compiling")
    sleep 1
    progress_vertex_complete "$vid2"
    progress_end
)
check "Saw [-] Installing deps, then [+] Installing deps, then [-] Compiling, then [+] Compiling?"
fi

# ──────────────────────────────────────────────────────────
header "2. Vertex: start → error"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    progress_begin "/tmp/progress-manual-2.log"
    vid=$(progress_vertex_start "Downloading artifact")
    sleep 1.5
    progress_vertex_error "$vid"
    progress_end
)
check "Saw [-] Downloading artifact, then [✗] Downloading artifact (red)?"
fi

# ──────────────────────────────────────────────────────────
header "3. Vertex: mixed complete + error + cached"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    progress_begin "/tmp/progress-manual-3.log"
    v1=$(progress_vertex_start "Step A")
    sleep 0.8
    progress_vertex_complete "$v1"
    v2=$(progress_vertex_start "Step B")
    sleep 0.8
    progress_vertex_error "$v2"
    v3=$(progress_vertex_start "Step C")
    sleep 0.5
    progress_vertex_cached "$v3"
    sleep 0.5
    progress_end
)
check "Saw [+] Step A, [✗] Step B (red), [◆] Step C (cached)?"
fi

# ──────────────────────────────────────────────────────────
header "4. Grid: static workers (no status change)"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    tmp_dir=$(mktemp -d)
    progress_begin "/tmp/progress-manual-4.log"
    progress_grid_start "$tmp_dir" 3
    echo "pending|Worker-1|waiting" > "$tmp_dir/1.status"
    echo "pending|Worker-2|waiting" > "$tmp_dir/2.status"
    echo "pending|Worker-3|waiting" > "$tmp_dir/3.status"
    sleep 2
    progress_grid_done
    progress_end
    rm -rf "$tmp_dir"
)
check "Saw 3 grid lines (· Worker-1/2/3 waiting) stable for 2s, no flicker?"
fi

# ──────────────────────────────────────────────────────────
header "5. Grid: workers progress through states"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    tmp_dir=$(mktemp -d)
    progress_begin "/tmp/progress-manual-5.log"
    progress_grid_start "$tmp_dir" 4
    echo "pending|alpha|queued" > "$tmp_dir/1.status"
    echo "pending|bravo|queued" > "$tmp_dir/2.status"
    echo "pending|charlie|queued" > "$tmp_dir/3.status"
    echo "pending|delta|queued" > "$tmp_dir/4.status"
    sleep 0.8
    echo "active|alpha|cloning..." > "$tmp_dir/1.status"
    echo "active|bravo|cloning..." > "$tmp_dir/2.status"
    sleep 0.8
    echo "ok|alpha|done 0.8s" > "$tmp_dir/1.status"
    echo "active|charlie|cloning..." > "$tmp_dir/3.status"
    sleep 0.8
    echo "ok|bravo|done 1.6s" > "$tmp_dir/2.status"
    echo "error|charlie|failed" > "$tmp_dir/3.status"
    echo "active|delta|cloning..." > "$tmp_dir/4.status"
    sleep 0.8
    echo "ok|delta|done 2.4s" > "$tmp_dir/4.status"
    sleep 0.5
    progress_grid_done
    progress_end
    rm -rf "$tmp_dir"
)
check "Saw workers transition: pending→active→ok/error? alpha/bravo/delta green, charlie red?"
fi

# ──────────────────────────────────────────────────────────
header "6. Grid: large grid (12 workers)"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    tmp_dir=$(mktemp -d)
    progress_begin "/tmp/progress-manual-6.log"
    progress_grid_start "$tmp_dir" 12
    for i in {1..12}; do
        echo "pending|repo-${i}|waiting" > "$tmp_dir/${i}.status"
    done
    sleep 0.5
    for i in {1..4}; do
        echo "active|repo-${i}|cloning..." > "$tmp_dir/${i}.status"
    done
    sleep 0.5
    for i in {1..4}; do
        echo "ok|repo-${i}|done" > "$tmp_dir/${i}.status"
    done
    for i in {5..8}; do
        echo "active|repo-${i}|cloning..." > "$tmp_dir/${i}.status"
    done
    sleep 0.5
    for i in {5..8}; do
        echo "ok|repo-${i}|done" > "$tmp_dir/${i}.status"
    done
    for i in {9..12}; do
        echo "active|repo-${i}|cloning..." > "$tmp_dir/${i}.status"
    done
    sleep 0.5
    for i in {9..12}; do
        echo "ok|repo-${i}|done" > "$tmp_dir/${i}.status"
    done
    sleep 0.3
    progress_grid_done
    progress_end
    rm -rf "$tmp_dir"
)
check "Saw 12-row grid scrolling through waves of active→ok?"
fi

# ──────────────────────────────────────────────────────────
header "7. Vertex → Grid → Vertex (mixed session)"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    tmp_dir=$(mktemp -d)
    progress_begin "/tmp/progress-manual-7.log"

    v1=$(progress_vertex_start "Preparing workspace")
    sleep 1
    progress_vertex_complete "$v1"

    progress_grid_start "$tmp_dir" 3
    echo "pending|svc-a|waiting" > "$tmp_dir/1.status"
    echo "pending|svc-b|waiting" > "$tmp_dir/2.status"
    echo "pending|svc-c|waiting" > "$tmp_dir/3.status"
    sleep 0.5
    echo "active|svc-a|building" > "$tmp_dir/1.status"
    echo "active|svc-b|building" > "$tmp_dir/2.status"
    sleep 0.8
    echo "ok|svc-a|done" > "$tmp_dir/1.status"
    echo "ok|svc-b|done" > "$tmp_dir/2.status"
    echo "active|svc-c|building" > "$tmp_dir/3.status"
    sleep 0.5
    echo "ok|svc-c|done" > "$tmp_dir/3.status"
    sleep 0.3
    progress_grid_done

    v2=$(progress_vertex_start "Running tests")
    sleep 1
    progress_vertex_complete "$v2"

    progress_end
    rm -rf "$tmp_dir"
)
check "Saw vertex (Preparing), then grid (3 workers), then vertex (Running tests)?"
fi

# ──────────────────────────────────────────────────────────
header "8. Crash safety: error mid-vertex (EXIT trap)"
# ──────────────────────────────────────────────────────────
if should_run; then
printf "  (this one will show an error, that's expected)\n"
(
    bash --norc --noprofile -c "
        set -euo pipefail
        source '${HOME}/.dotfiles/bash/core/progress.bash'
        progress_begin '/tmp/progress-manual-8.log'
        vid=\$(progress_vertex_start 'Risky operation')
        sleep 1.5
        false
    "
) || true
sleep 0.3
check "Saw [-] Risky operation, then [✗] Risky operation (red, from EXIT trap)?"
fi

# ──────────────────────────────────────────────────────────
header "9. Crash safety: error mid-grid (EXIT trap)"
# ──────────────────────────────────────────────────────────
if should_run; then
printf "  (this one will show an error, that's expected)\n"
(
    bash --norc --noprofile -c "
        set -euo pipefail
        source '${HOME}/.dotfiles/bash/core/progress.bash'
        tmp_dir=\$(mktemp -d)
        progress_begin '/tmp/progress-manual-9.log'
        progress_grid_start \"\$tmp_dir\" 2
        echo 'active|task-1|running' > \"\$tmp_dir/1.status\"
        echo 'pending|task-2|queued' > \"\$tmp_dir/2.status\"
        sleep 1.5
        false
    "
) || true
sleep 0.3
check "Saw grid with task-1 active, task-2 pending, then clean exit (cursor restored)?"
fi

# ──────────────────────────────────────────────────────────
header "10. progress_vertex_exec (wrapper)"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    progress_begin "/tmp/progress-manual-10.log"
    progress_vertex_exec "Quick task" bash -c "sleep 1 && echo 'task output here'"
    progress_end
)
check "Saw [-] Quick task, then [+] Quick task?"
fi

# ──────────────────────────────────────────────────────────
header "11. progress_vertex_exec with command failure"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    progress_begin "/tmp/progress-manual-11.log"
    progress_vertex_exec "Failing task" bash -c "sleep 0.5 && exit 1" || true
    progress_end
)
check "Saw [-] Failing task, then [✗] Failing task (red)?"
fi

# ──────────────────────────────────────────────────────────
header "12. Long label text (truncation / overflow)"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    progress_begin "/tmp/progress-manual-12.log"
    vid=$(progress_vertex_start "This is a very long vertex label that should test how the display handles overflow and wrapping in the terminal")
    sleep 1.5
    progress_vertex_complete "$vid"
    progress_end
)
check "Long label displayed without corrupting the terminal layout?"
fi

# ──────────────────────────────────────────────────────────
header "13. Grid with long detail text"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    tmp_dir=$(mktemp -d)
    progress_begin "/tmp/progress-manual-13.log"
    progress_grid_start "$tmp_dir" 2
    echo "active|worker-1|processing /very/long/path/to/some/deeply/nested/directory/structure/that/exceeds/terminal/width" > "$tmp_dir/1.status"
    echo "active|worker-2|short" > "$tmp_dir/2.status"
    sleep 2
    progress_grid_done
    progress_end
    rm -rf "$tmp_dir"
)
check "Long detail truncated cleanly, no terminal corruption?"
fi

# ──────────────────────────────────────────────────────────
header "14. Nested progress_begin/end"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    progress_begin "/tmp/progress-manual-14.log"
    v1=$(progress_vertex_start "Outer step")
    sleep 0.8
    progress_vertex_complete "$v1"

    progress_begin "/tmp/progress-manual-14-inner.log"
    v2=$(progress_vertex_start "Inner step")
    sleep 0.8
    progress_vertex_complete "$v2"
    progress_end

    v3=$(progress_vertex_start "Back to outer")
    sleep 0.8
    progress_vertex_complete "$v3"
    progress_end
)
check "Saw Outer step, Inner step, Back to outer, all displayed without resetting?"
fi

# ──────────────────────────────────────────────────────────
header "15. Rapid vertex succession (stress test)"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    progress_begin "/tmp/progress-manual-15.log"
    for i in {1..8}; do
        vid=$(progress_vertex_start "Step ${i} of 8")
        sleep 0.3
        progress_vertex_complete "$vid"
    done
    progress_end
)
check "Saw 8 steps flash by quickly, each showing [-] then [+]?"
fi

# ──────────────────────────────────────────────────────────
header "16. Scrolling output window (progress_vertex_exec)"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    progress_begin "/tmp/progress-manual-16.log"
    progress_vertex_exec "Building project" bash -c '
        for i in $(seq 1 50); do
            echo "[step $i/50] Compiling module_${i}.o"
            sleep 0.06
        done
    '
    progress_end
)
check "Saw [-] Building project with dimmed scrolling output below it, then [+] Building project (output collapsed)?"
fi

# ──────────────────────────────────────────────────────────
header "17. Rapid output (100 lines in 2s)"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    progress_begin "/tmp/progress-manual-17.log"
    progress_vertex_exec "Stress test" bash -c '
        for i in $(seq 1 100); do
            echo "line $i: $(head -c 60 /dev/urandom | base64 | head -c 60)"
            sleep 0.02
        done
    '
    progress_end
)
check "Saw rapid scrolling dimmed output below [-] Stress test, display kept up without corruption?"
fi

# ──────────────────────────────────────────────────────────
header "18. Command failure with output visible"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    progress_begin "/tmp/progress-manual-18.log"
    progress_vertex_exec "Compile with errors" bash -c '
        echo "gcc -c main.c"
        sleep 0.3
        echo "main.c:42: error: expected semicolon"
        sleep 0.3
        echo "main.c:58: error: undeclared identifier foo"
        sleep 0.3
        exit 1
    ' || true
    progress_end
)
check "Saw dimmed output lines, then [✗] Compile with errors (red), output collapsed?"
fi

# ──────────────────────────────────────────────────────────
header "19. Resize during output (manual resize test)"
# ──────────────────────────────────────────────────────────
if should_run; then
printf "  \033[2mResize your terminal window during this test.\033[0m\n"
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    progress_begin "/tmp/progress-manual-19.log"
    progress_vertex_exec "Slow build (resize now)" bash -c '
        for i in $(seq 1 30); do
            echo "[${i}/30] Processing chunk_${i} ..."
            sleep 0.3
        done
    '
    progress_end
)
check "Resized terminal during output, layout adjusted without corruption?"
fi

# ──────────────────────────────────────────────────────────
header "20. Vertex detail annotation"
# ──────────────────────────────────────────────────────────
if should_run; then
(
    set -euo pipefail
    source "${HOME}/.dotfiles/bash/core/progress.bash"
    progress_begin "/tmp/progress-manual-20.log"
    vid=$(progress_vertex_start "Checking golden build")
    sleep 0.4
    progress_vertex_detail "$vid" "no cached build found"
    sleep 0.4
    progress_vertex_complete "$vid"
    progress_end
)
check "Saw [-] Checking golden build, then [+] Checking golden build  (no cached build found) dimmed?"
fi

# ──────────────────────────────────────────────────────────
# Summary
# ──────────────────────────────────────────────────────────
printf "\n\033[1;36m━━━ Results ━━━\033[0m\n"
printf "  Passed: %d / %d\n" "$PASS" "$TOTAL"
if [[ $FAIL -gt 0 ]]; then
    printf "  \033[31mFailed: %d\033[0m\n" "$FAIL"
else
    printf "  \033[32mAll passed!\033[0m\n"
fi

rm -f /tmp/progress-manual-*.log
