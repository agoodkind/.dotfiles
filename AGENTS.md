# AGENTS.md

Instructions for AI agents (Cursor, Claude Code, Codex) working in this repository.

## Overview

Cross-platform dotfiles repository deployed on macOS and Ubuntu. Everything is
symlinked from `~/.dotfiles` into `$HOME` by `sync.sh`. The shell is **zsh**;
helper scripts and background workers are **bash**.

## Directory Layout

```
.dotfiles/
├── home/           # Symlinked into ~/ (e.g. home/.zshrc → ~/.zshrc)
├── zshrc/          # Zsh configuration (sourced from home/.zshrc → incl.zsh)
│   ├── core/       # perf.zsh, plugins.zsh, utils.zsh
│   ├── commands/   # aliases.zsh, git.zsh, prefer.zsh, editors.zsh, remote.zsh
│   ├── completions/
│   ├── integrations/ # zoxide.zsh, motd.zsh
│   └── platform/   # OS-specific zsh config
├── bash/
│   ├── core/       # tools.bash (logging, git helpers, submodule sync, update)
│   │               # colors.bash, defaults.bash, packages.bash
│   ├── background/ # dispatch.bash, updater.bash, and worker scripts
│   ├── setup/      # OS installers (platform/mac.bash, platform/debian.bash)
│   └── sync.bash   # Main sync logic (sync.sh delegates here)
├── bin/            # Standalone scripts added to $PATH
├── lib/            # Git submodules (zinit, scripts, zsh-defer, motd, ssh)
├── .githooks/      # Repository-local git hooks (pre-commit)
├── git-global-hooks/ # Global hooks applied via core.hooksPath
├── .cursor/        # AI agent config (rules, skills, commands)
└── sync.sh         # Entry point: exec bash/sync.bash "$@"
```

## Startup Flow

1. **`.zshenv`** — Earliest user file. Loads `zsh/datetime`, starts
   `$START_TIME`, sets up `_PERF_TREE`, bypasses `/etc/zshrc` locale fork
   and `/etc/zprofile` path_helper by caching their output.
2. **`.zshrc` (home/.zshrc)** — Sets `DOTDOTFILES`, sources `zshrc/incl.zsh`,
   then applies theme, PATH, and aliases.
3. **`zshrc/incl.zsh`** — Sources perf, plugins (zinit turbo), utils, commands,
   integrations, then launches `dispatch.bash` asynchronously via `_async`.
   Displays queued notifications from the previous session.

### Performance Constraints

- **zshrc load**: target < 20ms
- **Time-to-prompt**: target < 40ms
- `_PERF_TREE` records timing for every `_source` / `_async` call. Use
  `zsh_perf` to dump the tree.
- **3-tier plugin loading**: zinit turbo → zsh-defer → direct source, chosen
  by latency impact.
- File caching (path_cache.zsh, prefer_aliases.zsh, dircolors.cache) keeps
  startup fast.

## Background Dispatch

`dispatch.bash` is launched by `_async` from `incl.zsh` on every interactive
shell login. It acquires an atomic lock (`~/.cache/dotfiles_dispatch.lock`
directory, created via `mkdir`) so that only one dispatch runs at a time —
opening many terminals simultaneously is safe because all but the first
will exit immediately. It spawns workers in parallel and waits for all of
them:

| Worker                     | Log file                                    | Purpose                                    |
|----------------------------|---------------------------------------------|--------------------------------------------|
| `updater.bash`             | `~/.cache/dotfiles/updater.log`             | Pull dotfiles, sync submodules, weekly jobs |
| `prefer-cache-rebuild.bash`| `~/.cache/dotfiles/prefer-cache-rebuild.log`| Rebuild prefer-alias cache if stale        |
| `path-cache-rebuild.bash`  | `~/.cache/dotfiles/path-cache-rebuild.log`  | Rebuild path_helper cache (macOS)          |
| `zwc-recompile.bash`       | `~/.cache/dotfiles/zwc-recompile.log`       | Recompile .zwc files for stale .zsh files  |
| `ssh-key-load-mac.bash`    | `~/.cache/dotfiles/ssh-key-load-mac.log`    | Load ed25519 key into macOS keychain       |

`dispatch.bash` itself logs process lifecycle to
`~/.cache/dotfiles_dispatch.log` (start/finish of each worker). Each worker
gets its own log via `dotfiles_log_init "<name>"`.

## Updater Flow

`updater.bash` runs in the background on every login (concurrency is
prevented by the dispatch lock — see Background Dispatch above):

1. Check internet connectivity.
3. Call `dotfiles_update_repo` (defined in `tools.bash`):
   - Check git health (detached HEAD, rebase in progress, etc.).
   - Fetch from origin.
   - Compare HEAD to `origin/main` to determine if behind.
   - Stash local changes if needed, pull `--ff`, pop stash.
   - Re-source `tools.bash` after pull so submodule sync uses the freshest code.
   - Call `_sync_submodules`.
4. If new commits were pulled, run `sync.sh --quick --skip-git`.
5. If no new commits, check if a weekly full update is due (7-day interval).
   Weekly update runs `sync.sh --repair --skip-git`, zinit update, and
   brew/apt upgrade.

## Submodule Sync

Submodules live under `lib/` and track upstream branches (not pinned SHAs):

- `lib/zinit` — zsh plugin manager
- `lib/scripts` — personal shell scripts (may have local work)
- `lib/zsh-defer` — deferred loading library

`_sync_submodules` (in `tools.bash`):
1. `git submodule update --init` to ensure all submodules are checked out.
2. For each submodule, `_sync_one_submodule`:
   - Detect tracking branch (`main` or `master`) from `.gitmodules` or remote.
   - `git fetch` (full output, not quiet).
   - For `lib/scripts`: stash local work (including untracked) before pull.
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

All background workers use a unified logging design from `tools.bash`:

- **`dotfiles_log_init "<name>"`** — Creates `~/.cache/dotfiles/<name>.log`,
  exports `DOTFILES_LOG` so all child calls inherit the path.
- **`dotfiles_log "<message>"`** — Timestamped entry. Always writes to
  `$DOTFILES_LOG`. Also prints to terminal when stdout is a tty.
- **`dotfiles_run <command...>`** — Runs a command with output routing:
  interactive → tee to log and terminal; background → log only.

### Manual `sync.sh` Runs

When `sync.sh` is run interactively, `dotfiles_log_init "sync"` is called at
the top. All `dotfiles_run` calls tee output to both the terminal and
`~/.cache/dotfiles/sync.log`. Stderr from `dotfiles_update_repo` flows
directly to the terminal (no capture with `2>&1`).

## Work vs. Personal Separation

- `WORK_DIR_PATH` env var (set in `~/.overrides.local`) signals a work laptop.
- Work laptops skip SSH config sync, authorized_keys, and `/opt/scripts`.
- `.githooks/pre-commit` blocks committing proprietary patterns. Override
  patterns live in `.githooks/deny-patterns.local` (gitignored).
- `.zshrc.local` and `~/.overrides.local` hold machine-specific config
  (both gitignored).

## Shell Compatibility

- Target shells: **zsh** (primary, interactive), **bash** (scripts, background).
- All `bash/` scripts use `#!/usr/bin/env bash`.
- Avoid bashisms in code that might be sourced from zsh and vice versa;
  `tools.bash` is sourced from both.
- Use `$EPOCHSECONDS` / `zsh/datetime` for timestamps in zsh, `date +%s` in
  bash. Prefer builtins over `gdate`/`gstat` for cross-platform safety.
- macOS ships BSD `stat`/`date`; Linux ships GNU. The codebase avoids
  platform-specific flags by using `awk strftime` (`epoch_to_date`) and
  portable find/test expressions.

## Common Commands

| Command | Purpose |
|---------|---------|
| `./sync.sh` | Full manual sync (git + link + install + compile) |
| `./sync.sh --quick --skip-git` | Re-link and recompile without git or installs |
| `./sync.sh --repair --skip-git` | Deep repair (clean up stale state, reinstall) |
| `zsh_perf` | Print startup performance tree |

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
3. **Test on both platforms** when touching `tools.bash`, `.zshenv`, or
   submodule logic. Use `is_macos` / `is_linux` guards for platform-specific
   code.
4. **Logging**: Use `dotfiles_log_init`, `dotfiles_log`, `dotfiles_run` for
   any new background work. Do not invent bespoke log/run helpers.
5. **Notifications**: Use `dotfiles_notify` to surface messages at next login.
   Always pass the log file path (or rely on `$DOTFILES_LOG` fallback).
6. **Submodules**: These track upstream branches. Do not pin to specific SHAs.
   `_sync_one_submodule` handles fetch/checkout/rebase.
7. **Bash version on macOS**: macOS ships bash 3.2 and non-login SSH sessions
   lack `/usr/local/bin` in PATH, breaking any bash 4+ feature. The fix is:
   - For entry point scripts: source `bash/core/init.bash` as the first line.
     It normalizes PATH on macOS (propagates to all child processes via export),
     then re-execs into bash 4+ if needed. No per-script guards or `"$BASH"`
     propagation required in downstream scripts.
   - Entry points: `sync.bash`, `updater.bash`, `dispatch.bash`.
   - New entry point pattern:
     ```bash
     DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
     source "$DOTDOTFILES/bash/core/init.bash"
     ```
