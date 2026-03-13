# Reactive progress engine

BuildKit-style progress display: state files plus a background display loop that polls and redraws. Callers write state, the loop renders to `/dev/tty`.

## API

- **progress_begin [log_file]**
  Create state dir, start display loop (TTY only), set EXIT trap. Supports nesting via `_PROGRESS_SESSION_DEPTH`: nested calls increment the counter and return early, so inner callers (e.g. `cleanup_worktrees` inside `wkm.sh`) share the parent session.

- **progress_end**
  Decrement session depth. When depth reaches 0: touch `.done`, wait for display process, restore cursor, remove state dir. Nested calls decrement and return early.

- **progress_vertex_start \<label\>**
  Write `started|label|ts` to next `N.vertex`. Vertex IDs are assigned from a file-based counter (`.counter`) that persists across subshells. Returns vertex id on stdout.

- **progress_vertex_complete \<id\> [label] [suffix]**
  Overwrite vertex file with `completed|...`. The optional `suffix` renders as a dimmed parenthetical after the label: `[+] Syncing SSH config  (work laptop)`.

- **progress_vertex_error \<id\> [label]**
  Overwrite vertex file with `error|...`.

- **progress_vertex_warning \<id\> [label]**
  Overwrite vertex file with `warning|...`.


- **progress_vertex_detail \<id\> \<detail\>**
  Attach a short annotation to a vertex. Can be called at any point while the vertex is `started`, or before calling complete/error/warning. The detail survives through status transitions and renders as a dimmed suffix after the label: `[-] Checking build  (no cached build found)` while running, `[+] Checking build  (no cached build found)` when complete.

- **progress_vertex_exec \<label\> \<command\> [args...]**
  Start a vertex, run command, stream stdout/stderr to `N.out` (TTY mode) or inline (non-TTY), then complete or error the vertex based on exit code. In TTY mode, the display loop picks up `N.out` and renders a scrolling output window below the active vertex.

- **progress_grid_start \<tmp_dir\> \<total\> [format_fn]**
  Switch display loop to grid mode. Workers write `status|name|detail` to `tmp_dir/N.status`. The display loop polls those files and renders a grid of workers with status icons (pending/active/ok/error). If called within a `progress_begin` session, the grid integrates into the main display loop. If called standalone, it spawns its own grid loop.

- **progress_grid_done**
  Leave grid mode. Remove `.grid` control file, wait for standalone grid display process if any, clean up.

- **progress_log \<message\>**
  Append to log file with ANSI escapes stripped.

- **progress_set_log_file \<path\>**
  Set log file path and write header. Can be used independently of `progress_begin`.

## Scrolling output window

When a vertex is `started`, `warning`, or `error` and has a `.out` file, the display loop renders the last N lines of output below the vertex status line. Lines are dimmed, indented, and truncated to terminal width. The window height adapts to terminal size: `output_max = terminal_rows - vertex_count - 2`, clamped between 3 and 15 lines. On completion, the output window collapses (final render omits output lines), while warning/error retain output in the final render. Terminal dimensions are re-read each loop iteration via `stty size`, so resizing the terminal adjusts the layout live.

## State protocol

- **State dir**: created by `progress_begin`, removed by `progress_end` or EXIT trap.
- **`N.vertex`**: one file per vertex. Content: `status|label|timestamp|detail`. The `detail` field is optional (empty string when unset). Status values: `started`, `completed`, `warning`, `error`.
- **`N.out`**: command output from `progress_vertex_exec`. Written by the exec function, read by the display loop for the scrolling output window.
- **`.counter`**: integer file tracking the next vertex ID. Incremented by `progress_vertex_start`. File-based (not a shell variable) so it persists across `$(...)` subshells.
- **`.done`**: created when the session ends. Display loop exits when it sees this file.
- **`.grid`**: presence signals grid mode is active.
- **`.grid_tmp`**: path to the grid worker status directory.
- **`.grid_total`**: number of grid workers.

## TTY detection

`_progress_is_tty` returns false when:
- `PROGRESS_NO_TTY=1` (used by the background updater)
- `GITHUB_ACTIONS=true` or `CI=true`
- `/dev/tty` is not a character device
- `/dev/tty` exists but is not writable (no controlling TTY)

When not a TTY, `progress_begin` still creates the state dir and sets the log file, but skips the display loop and EXIT trap. `progress_vertex_exec` falls back to linear inline output with log-only streaming.

## Crash safety

EXIT trap runs on any exit (including SIGINT). It marks all `started` vertices as `error`, touches `.done`, waits for the display process to render the final state, restores the cursor, cleans up the state dir, and exits with the original code. If a log file was set, the trap prints its path to stderr on non-zero exit.

## Logging

- `progress_log` and `progress_set_log_file` work independently of the display engine.
- `progress_begin` can set the log file. Vertex transitions and exec output are appended with ANSI escapes stripped.
- Display output (ANSI cursor control, grid rendering) never reaches the log file.
