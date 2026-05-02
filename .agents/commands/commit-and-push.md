# Commit and Push

Shortcut that runs `/commit` then `/push` in sequence.

## Steps

1. Before doing anything else, run `git rev-parse --show-toplevel` and `git remote get-url origin` to confirm you are operating in the correct repository and that `origin` points to the expected remote URL. In multi-repo workspaces, the shell's working directory may point to a different repo than the one being discussed.
2. Follow all remaining rules and steps from `/commit`.
3. After a successful commit, follow all rules and steps from `/push`. The remote URL check in step 1 satisfies the equivalent check in `/push`; you do not need to repeat it.
