# [AGENTS.md](http://AGENTS.md)

Instructions for AI agents (Cursor, Claude Code, Codex) working in this repository.

## Overview

Cross-platform dotfiles repository deployed on macOS and Ubuntu. Everything is
symlinked from `~/.dotfiles` into `$HOME` by `sync.sh`. The shell is **zsh**;
bootstrap entry points are thin **bash** wrappers, and background maintenance
is handled by the Go-based `dots` binary.

## Startup Flow

1. `**.zshenv`** — Earliest user file. Starts timing, sets up the perf tree,
  bypasses `/etc/zshrc` locale fork and `/etc/zprofile` path_helper by
   caching their output.
2. `**.zshrc**` — Sets `DOTDOTFILES`, sources `zshrc/incl.zsh`, then applies
  theme, PATH, and aliases.
3. `**zshrc/incl.zsh**` — Sources perf, plugins (zinit turbo), utils, commands,
  integrations, then launches `dots dispatch` asynchronously via `_async`.
  Displays queued notifications from the previous session.

### Performance Constraints

- **zshrc load**: target < 20ms
- **Time-to-interactive-prompt**: target < 40ms
- The perf tree records timing for every `_source` / `_async` call. Use
`zsh_perf` or `dots perf` to inspect the latest startup log.
- **3-tier plugin loading**: zinit turbo → zsh-defer → direct source, chosen
by latency impact.
- File caching (PATH, prefer aliases, dircolors) keeps startup fast.

## Background Dispatch

`dots dispatch` is launched by `_async` from `incl.zsh` on every
interactive shell login. It acquires a `flock` on
`~/.cache/dotfiles_dispatch.flock` so
that only one dispatch runs at a time — opening many terminals simultaneously
is safe because all but the first will exit immediately. The lock is released
automatically by the kernel when the process exits for any reason, including
SIGKILL. `~/.cache/dotfiles_dispatch.lock/` is a status-only directory created
after the flock is acquired; `incl.zsh` checks for its existence to show the
"running in background" banner and reads the optional `status` file inside it.
It spawns workers in parallel and waits for all of them:


| Worker                  | Purpose                                     |
| ----------------------- | ------------------------------------------- |
| `updater`               | Pull dotfiles, sync submodules, weekly jobs |
| `prefer-cache-rebuild`  | Rebuild prefer-alias cache if stale         |
| `path-cache-rebuild`    | Rebuild path_helper cache (macOS)           |
| `zwc-recompile`         | Recompile .zwc files for stale .zsh files   |
| `ssh-key-load-mac`      | Load ed25519 key into macOS keychain        |

`dots dispatch` logs process lifecycle and worker output to
`~/.cache/dotfiles_dispatch.log`.

## Updater Flow

The `updater` worker runs in the background on every login (concurrency is
prevented by the dispatch lock — see Background Dispatch above):

1. Check internet connectivity.
2. Update the repo:
  - Check git health (detached HEAD, rebase in progress, etc.).
  - Fetch from origin.
  - Compare HEAD to `origin/main` to determine if behind.
  - Stash local changes if needed, pull `--ff`, pop stash.
  - Sync submodules.
3. If new commits were pulled, run `sync.sh --quick --skip-git`.
4. If no new commits, check if a weekly full update is due (7-day interval).
  Weekly update runs `sync.sh --repair --skip-git`, zinit update, and
   brew/apt upgrade.

## Submodule Sync

Submodules live under `lib/` and track upstream branches (not pinned SHAs).
Run `git submodule status` for the current list.

The sync logic in the Go sync implementation:

1. `git submodule update --init` to ensure all submodules are checked out.
2. For each submodule:
  - Detect tracking branch (`main` or `master`) from `.gitmodules` or remote.
  - `git fetch` (full output, not quiet).
  - For submodules with local work: stash (including untracked) before pull.
  - `git checkout <branch>`, then `git pull --rebase origin <branch>`.
  - On failure: abort rebase, notify, preserve local state.
3. Auto-commit submodule pointer updates when the parent index is clean.

## Notification System

**File**: `~/.cache/dotfiles/notifications`

**Format**: `level|logfile|message` (one notification per line)

- `level`: `success`, `info`, `warn`, `error`
- `logfile`: path to the log file that produced this notification (may be empty)
- `message`: human-readable text (may contain `|` characters)

`dotfiles_notify` writes to this file. At next interactive login, `incl.zsh`
reads and displays each notification with color coding and shows the log
file path if available, then deletes the file.

## Logging System

- Background dispatch logs to `~/.cache/dotfiles_dispatch.log`.
- `dots` command logs default to `~/.cache/dotfiles/dots.log`
unless `DOTFILES_LOG` is set.
- Startup performance logs are written to `~/.cache/zsh_startup/*.json` and
inspected via `zsh_perf` / `dots perf`.

### Manual `sync.sh` Runs

`sync.sh` is a thin wrapper around `dots sync`, so interactive sync
output comes from the Go command directly rather than shell-side tee helpers.

## Work vs. Personal Separation

- `WORK_DIR_PATH` env var (set in `~/.overrides.local`) signals a work laptop.
- Work laptops skip SSH config sync, authorized_keys, and `/opt/scripts`.
- `.githooks/pre-commit` blocks committing proprietary patterns. Override
patterns live in `.githooks/deny-patterns.local` (gitignored).
- `.zshrc.local` and `~/.overrides.local` hold machine-specific config
(both gitignored).

## Shell Compatibility

- Target shells: **zsh** (primary, interactive), **bash** (bootstrap wrappers).
- All `bash/` scripts use `#!/usr/bin/env bash`.
- Avoid bashisms in code that might be sourced from zsh and vice versa.
- Use `$EPOCHSECONDS` for timestamps in zsh, `date +%s` in bash. Prefer
builtins over `gdate`/`gstat` for cross-platform safety.
- macOS ships BSD `stat`/`date`; Linux ships GNU. The codebase avoids
platform-specific flags by using `awk strftime` for date formatting and
portable find/test expressions.

## Common Commands


| Command                         | Purpose                                                                             |
| ------------------------------- | ----------------------------------------------------------------------------------- |
| `./sync.sh`                     | Full manual sync (Go `dots sync`)                                            |
| `./sync.sh --quick --skip-git`  | Re-link and recompile; skips package installs, zinit/nvim updates, and custom tools |
| `./sync.sh --repair --skip-git` | Deep repair (clean up stale state, reinstall)                                       |
| `zsh_perf`                      | Inspect the latest startup performance log                                          |
| `dots perf history`      | Show startup performance history                                                    |


## Git Operations on This Repository

**Do not use `git push` to push changes in this repository.** The dotfiles repo
uses a bare-repo / worktree setup where the working tree is `~/.dotfiles` but
the git directory is stored elsewhere. The `config` shell alias wraps the
correct `git --git-dir` invocation.

Use `config push` instead of `git push` for any push operation on `.dotfiles`.
All other git read operations (`git status`, `git diff`, `git log`, etc.) work
normally when run from inside `~/.dotfiles`; only push (and any operation that
writes to the remote) requires `config`.

## Smoke Testing zsh Startup

To accurately test that zsh loads without errors on a remote host, allocate a real PTY,
force interactive and login mode, and unset Cursor/VSCode env vars that affect MOTD and
terminal detection logic:

```bash
ssh -t <host> 'env -u TERM_PROGRAM -u VSCODE_INJECTION -u VSCODE_SHELL_INTEGRATION \
  zsh -i -l -c "echo ok" 2>&1'
```

- `-t`: forces PTY allocation so zsh behaves as it would in a real terminal session.
- `-i`: interactive mode (loads `.zshrc`).
- `-l`: login mode (also sources `.zprofile` and `.zshenv`).
- `env -u ...`: strips Cursor/VSCode injected variables that alter MOTD and terminal
detection (e.g. `TERM_PROGRAM=vscode` suppresses MOTD on some paths).
- Running just `zsh -c` without `-i` skips `.zshrc` entirely; running without `-t` means
no TTY, which can suppress prompts and alter plugin behavior.

## Smoke Testing Shell Changes

After modifying zsh startup files, test with a real interactive login shell. The
correct invocation is `ssh -t <host> 'TERM_PROGRAM= zsh -i -l -c "echo ok"'`:

- `-t` allocates a PTY (required; without it zsh may skip interactive paths).
- `-i -l` makes zsh interactive and login, sourcing `.zshenv`, `.zprofile`, and
`.zshrc` in the same order a real terminal session would.
- `TERM_PROGRAM=` unsets the VSCode/Cursor env var so IDE-specific branches
(MOTD suppression, editor detection) do not skew the result.

This only works when run from a real terminal (iTerm2, Terminal.app). When run
from Cursor's shell, stdin is not a TTY so `-t` has no effect and ssh falls back
to no-PTY mode. In that case the test is still useful for catching errors but
does not fully replicate a login session.

## Rules for Changes

1. **Performance**: Keep `.zshrc` load under 20ms. Profile with `zsh_perf`
  after changes. Use `_async` or `zsh-defer` for anything that can wait.
2. **No secrets**: Never commit keys, tokens, or work-specific paths.
  Use `~/.overrides.local` or env vars.
3. **Test on both platforms** when touching `.zshenv`, Go sync/dispatch logic,
  or submodule logic. Use platform guards for macOS/Linux-specific code.
4. **Logging**: Route new background work through the existing Go telemetry
  logger and notification flow. Do not reintroduce bespoke shell log helpers.
5. **Notifications**: Use `dotfiles_notify` to surface messages at next login.
  Always pass the log file path (or rely on `$DOTFILES_LOG` fallback).
6. **Submodules**: These track upstream branches. Do not pin to specific SHAs.
  The Go sync logic handles fetch/checkout/rebase.
7. **Bash version on macOS**: macOS ships bash 3.2 and non-login SSH sessions
  lack `/usr/local/bin` in PATH, breaking any bash 4+ feature. The fix is:
  - For any new top-level entry point script: source `bash/core/init.bash` as
  the first line. It normalizes PATH on macOS (propagates to all child
  processes via export), then re-execs into bash 4+ if needed. No per-script
  guards or `"$BASH"` propagation required in downstream scripts.
  - New entry point pattern:
    ```bash
    DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
    source "$DOTDOTFILES/bash/core/init.bash"
    ```
