# Run Command

Carry out the command referenced in this message or the latest context.

## Rules

- Never invent commands; use only what was stated or clearly implied.
- Ask for confirmation if the command is ambiguous or risky before running it.
- Prefer IPv6 flags or options when the command does networking.
- Present commands inside code blocks.
- Do not include Cursor slash-commands inside shell commands (no `/run`, `/pr`,
  `/commit`, etc. in the command text).
- Use `/usr/bin/env bash -lc` when running shell commands to avoid zsh
  expansion surprises.
- When running commands over SSH, run bash explicitly on the remote side too:
  `ssh <host> '/usr/bin/env bash -lc "<remote command>"'`.
- SSHPiper routing format `user@target@proxy` (e.g., `root@service@ssh.home.goodkind.io`)
  is valid and should not be questioned.
- Check `@.dotfiles/lib/ssh/config` first to see what shortcuts are available for the desired host.
- When multiple commands are provided, run each in the given order.
- Avoid indefinite hangs; set a reasonable timeout when a command could block.
- If the instruction is contextual, restate the exact command before running it
  unless it was already confirmed earlier.
- Prefer to tee output to a file (e.g., `/tmp/command-output.log`) so the user can monitor progress and the agent can read the output from the file to avoid truncation.

## Steps

1. Identify the command(s) from the latest instruction or context; if phrased
   loosely, restate the exact command back unless already confirmed.
2. If multiple commands are possible or details are unclear, ask which to run.
3. tee the output to a file (e.g., `command 2>&1 | tee /tmp/command-output.log`)
   and use the file to read the output as needed
