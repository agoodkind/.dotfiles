#!/usr/bin/env bats

setup() {
    source "${BATS_TEST_DIRNAME}/../bash/core/progress.bash"
    export STATE_DIR="${BATS_TMPDIR}/progress-state"
    export LOG_FILE="${BATS_TMPDIR}/progress.log"
    mkdir -p "$STATE_DIR"
    _PROGRESS_STATE_DIR="$STATE_DIR"
}

teardown() {
    rm -rf "$STATE_DIR"
    rm -f "$LOG_FILE"
}

@test "progress_vertex_exec works without calling progress_begin first" {
    local progress_lib="${BATS_TEST_DIRNAME}/../bash/core/progress.bash"
    run bash --norc --noprofile -c "
        set -euo pipefail
        export CI=true
        source '${progress_lib}'
        progress_vertex_exec 'Auto Init' bash -c 'echo hello'
    "
    [ "$status" -eq 0 ]
    [[ "$output" == *"hello"* ]]
    [[ "$output" == *"+ Completed"* ]]
}

@test "sequential vertex starts get unique incrementing IDs" {
    local progress_lib="${BATS_TEST_DIRNAME}/../bash/core/progress.bash"
    run bash --norc --noprofile -c "
        set -euo pipefail
        source '${progress_lib}'
        progress_begin
        v1=\$(progress_vertex_start 'Step one')
        v2=\$(progress_vertex_start 'Step two')
        v3=\$(progress_vertex_start 'Step three')
        echo \"ids: \$v1 \$v2 \$v3\"
        progress_end
    "
    [ "$status" -eq 0 ]
    [[ "$output" == *"ids: 1 2 3"* ]]
}

@test "vertex lifecycle functions (start, complete, error, cached)" {
    local vid
    vid=$(progress_vertex_start "Test Start")
    [ -f "${STATE_DIR}/${vid}.vertex" ]

    local content
    content=$(cat "${STATE_DIR}/${vid}.vertex")
    [[ "$content" == started\|Test\ Start\|* ]]

    progress_vertex_complete "$vid" "Test Complete"
    content=$(cat "${STATE_DIR}/${vid}.vertex")
    [[ "$content" == completed\|Test\ Complete\|* ]]

    vid=$(progress_vertex_start "Test Error")
    progress_vertex_error "$vid" "Test Error"
    content=$(cat "${STATE_DIR}/${vid}.vertex")
    [[ "$content" == error\|Test\ Error\|* ]]

    vid=$(progress_vertex_start "Test Cached")
    progress_vertex_cached "$vid" "Test Cached"
    content=$(cat "${STATE_DIR}/${vid}.vertex")
    [[ "$content" == cached\|Test\ Cached\|* ]]
}

@test "crash safety (EXIT trap marks started as error)" {
    local vid
    vid=$(progress_vertex_start "Incomplete Vertex")

    local content
    content=$(cat "${STATE_DIR}/${vid}.vertex")
    [[ "$content" == started* ]]

    # Simulate the marking portion of _progress_exit_trap (without the exit/cleanup)
    local i=1
    while [[ -f "${STATE_DIR}/${i}.vertex" ]]; do
        content=$(cat "${STATE_DIR}/${i}.vertex" 2>/dev/null)
        [[ "$content" == started* ]] && _progress_vertex_write "$i" "error" "${content#*|}" "${content##*|}"
        ((i++)) || true
    done

    content=$(cat "${STATE_DIR}/${vid}.vertex")
    [[ "$content" == error* ]]
}

@test "non-TTY mode linear output (command output shown inline)" {
    export CI=true
    local progress_lib="${BATS_TEST_DIRNAME}/../bash/core/progress.bash"
    local dummy_script="${BATS_TMPDIR}/dummy.sh"
    cat > "$dummy_script" << EOF
#!/usr/bin/env bash
set -euo pipefail
source "${progress_lib}"
progress_begin
progress_vertex_exec "Test Non-TTY" bash -c "echo 'hello world'"
progress_end
EOF
    chmod +x "$dummy_script"

    run "$dummy_script"
    [ "$status" -eq 0 ]
    [[ "$output" == *"hello world"* ]]
    [[ "$output" == *"+ Completed"* ]]
}

@test "grid mode (parallel workers)" {
    export CI=true
    progress_grid_start "$STATE_DIR" 2

    [ -f "${STATE_DIR}/.grid" ]
    [ -f "${STATE_DIR}/.grid_tmp" ]
    [ -f "${STATE_DIR}/.grid_total" ]

    echo "ok|Worker 1|done" > "${STATE_DIR}/1.status"
    echo "error|Worker 2|failed" > "${STATE_DIR}/2.status"

    progress_grid_done

    [ ! -f "${STATE_DIR}/.grid" ]
}

@test "nested progress_begin/end preserves parent session" {
    local progress_lib="${BATS_TEST_DIRNAME}/../bash/core/progress.bash"
    run bash --norc --noprofile -c "
        set -euo pipefail
        source '${progress_lib}'
        progress_begin
        echo \"depth after outer begin: \$_PROGRESS_SESSION_DEPTH\"
        progress_begin
        echo \"depth after inner begin: \$_PROGRESS_SESSION_DEPTH\"
        progress_end
        echo \"depth after inner end: \$_PROGRESS_SESSION_DEPTH\"
        progress_end
        echo \"depth after outer end: \$_PROGRESS_SESSION_DEPTH\"
    "
    [ "$status" -eq 0 ]
    [[ "$output" == *"depth after outer begin: 1"* ]]
    [[ "$output" == *"depth after inner begin: 2"* ]]
    [[ "$output" == *"depth after inner end: 1"* ]]
    [[ "$output" == *"depth after outer end: 0"* ]]
}

@test "progress_vertex_detail sets detail on vertex file" {
    local vid
    vid=$(progress_vertex_start "Build check")
    progress_vertex_detail "$vid" "no cached build found"

    local content
    content=$(cat "${STATE_DIR}/${vid}.vertex")
    local detail="${content##*|}"
    [[ "$detail" == "no cached build found" ]]
}

@test "detail persists through vertex_complete" {
    local vid
    vid=$(progress_vertex_start "Build check")
    progress_vertex_detail "$vid" "no cached build found"
    progress_vertex_complete "$vid"

    local content
    content=$(cat "${STATE_DIR}/${vid}.vertex")
    [[ "$content" == completed* ]]
    [[ "${content##*|}" == "no cached build found" ]]
}

@test "detail persists through vertex_error" {
    local vid
    vid=$(progress_vertex_start "Build check")
    progress_vertex_detail "$vid" "upstream not reachable"
    progress_vertex_error "$vid"

    local content
    content=$(cat "${STATE_DIR}/${vid}.vertex")
    [[ "$content" == error* ]]
    [[ "${content##*|}" == "upstream not reachable" ]]
}

@test "render includes detail suffix" {
    local output
    output=$(_progress_render_vertex_line "completed" "My Step" "cache hit")
    [[ "$output" == *"My Step"* ]]
    [[ "$output" == *"cache hit"* ]]
}

@test "logging strips ANSI escapes from log file" {
    progress_set_log_file "$LOG_FILE"

    progress_log "Test ${ESC}[31mRedText${ESC}[0m"

    local content
    content=$(cat "$LOG_FILE")
    [[ "$content" == *"Test RedText"* ]]
    [[ "$content" != *$'\033'* ]]
}
