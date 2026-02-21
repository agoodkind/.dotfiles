# Reactive progress engine

BuildKit-style progress display: state files plus a background display loop that polls and redraws. No cursor math in the main script; the loop writes to `/dev/tty`.

## API

- **progress_begin [log_file]**  
  Create state dir, start display loop (TTY only), set EXIT trap. Optionally set log file and write header.

- **progress_end**  
  Touch `.done`, wait for display process, restore cursor, remove state dir.

- **progress_vertex_start \<label\>**  
  Write `started|label|ts` to next `N.vertex`. Returns vertex id.

- **progress_vertex_complete \<id\> [label]**  
  Overwrite vertex with `completed|...`.

- **progress_vertex_error \<id\> [label]**  
  Overwrite vertex with `error|...`.

- **progress_vertex_cached \<id\> [label]**  
  Overwrite vertex with `cached|...`.

- **progress_vertex_exec \<label\> -- \<command\> [args...]**  
  Start a vertex, run command, stream stdout/stderr to log (and in TTY to state), then complete or error the vertex.

- **progress_grid_start \<tmp_dir\> \<total\> [format_fn]**  
  (Stub) Switch display loop to grid mode; poll `tmp_dir/1` … `tmp_dir/total` for `tag|rest` lines. Not yet wired into the display loop.

- **progress_grid_done**  
  (Stub) Leave grid mode.

## Backward compatibility

- **progress_init [log_file]** → progress_begin
- **progress_done** → progress_end
- **progress_step \<name\> [num]** → progress_vertex_start (and store as current vertex for next exec).
- **progress_exec_stream \<cmd\> [args...]** → If there is a current vertex (from progress_step), run command and complete/error that vertex; else progress_vertex_exec.

Scripts that only use progress_set_log_file + trap progress_on_exit_trap and never call progress_begin keep the legacy behaviour: progress_step echoes and logs, progress_exec_stream uses the old inline TTY streaming. Call progress_begin at the start of main to use the reactive engine.

## State protocol

- State dir: created by progress_begin, removed by progress_end or EXIT trap.
- `N.vertex`: one file per vertex; content `status|label|timestamp` with status in `started`, `completed`, `error`, `cached`.
- `.done`: created when the session is ending; display loop exits when it sees this.

## Crash safety

EXIT trap runs on any exit. It marks all `started` vertices as `error`, touches `.done`, waits for the display process to render and exit, restores cursor, then cleans up and exits with the original code.

## Non-TTY / CI

When not a TTY (or CI), progress_begin does nothing (no state dir, no loop). progress_vertex_* and progress_vertex_exec fall back to linear output and logging.

## Logging

- progress_set_log_file, progress_log, progress_on_exit_trap unchanged.
- progress_begin can set the log file; vertex transitions and exec output are appended (ANSI stripped).
