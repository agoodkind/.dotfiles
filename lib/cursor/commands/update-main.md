# Safely Update Branch with Latest Main

Fetch the latest `origin/main` and rebase or merge the current branch onto it, with a backup branch created first. Handle the entire process autonomously. Do not ask the user questions.

## Critical Rules

- **Always fetch first**: Run `git fetch origin main` explicitly. Never rely on a stale local `main`.
- **Always back up**: Create a backup branch before any rebase or merge operation.
- **Never force push**: Do not force push unless the user explicitly requests it.
- **Dirty tree protection**: If the working tree is dirty, stash changes before proceeding and pop them after.
- **Fully autonomous**: Analyze the branch state, choose the best strategy, and execute it. Do not ask the user to confirm the strategy. Just do it and report what you did.
- **Non-interactive only**: Never use interactive or TTY-dependent commands. No `git rebase -i`, no `git add -i`, no editors, no pagers. Always pass flags that suppress interactive behavior (e.g., `--no-edit` for merge commits). All commands must work in a non-TTY environment.
- **Flat backup branch names**: Never use slashes in backup branch names. Branch names with slashes (e.g., `user/feature`) create nested ref directories, and adding more slashes (e.g., `backup/user/feature/timestamp`) causes git ref conflicts. Always use dashes.

## Backup Branch Naming

Use a flat, dash-separated name to avoid git ref directory conflicts:

```
backup--<branch-with-slashes-replaced-by-dashes>--<timestamp>
```

Replace all `/` in the current branch name with `-`. Use double dashes `--` as the delimiter between segments so it's visually distinct from the branch name itself.

Example: branch `agoodkind/datagen-rich-ext-txns` becomes `backup--agoodkind-datagen-rich-ext-txns--20260210-172022`.

## Strategy Selection (autonomous, no user input)

Analyze these signals and pick the right strategy:

| Signal | Rebase | Merge |
|--------|--------|-------|
| Branch has NOT been pushed to remote | Yes | |
| Branch has been pushed/shared with others | | Yes |
| Local commits are clean, linear (no merge commits) | Yes | |
| Local history already contains merge commits | | Yes |
| Few local commits (< 10) on top of main | Yes | |
| Many local commits (10+) diverged significantly | | Yes |
| Branch is a long-lived feature branch with collaborators | | Yes |

When signals conflict, prefer merge (it's safer and non-destructive).

## Steps

1. Run `git branch --show-current` to get the current branch name. Abort if on `main` directly.
2. Run `git status --porcelain` to check for uncommitted changes.
   - If dirty, run `git stash push -m "pre-update-main auto-stash"` and note that a stash was created.
3. Create the backup branch using the flat naming scheme described above.
4. Fetch latest main: `git fetch origin main`.
5. Gather strategy signals (run all of these):
   - `git log --oneline HEAD..origin/main` (new commits on main)
   - `git log --oneline origin/main..HEAD` (local commits)
   - `git log --oneline @{u}..HEAD 2>/dev/null` (whether branch is pushed)
   - `git log --merges --oneline origin/main..HEAD` (whether local history has merges)
6. Choose rebase or merge using the table above. Do not ask. Just execute.
7. Execute the chosen strategy:
   - **Rebase**: `git rebase origin/main` (never use `-i`)
   - **Merge**: `git merge --no-edit origin/main`
   - If it succeeds, show the new log with `git log --oneline -5`
   - If conflicts occur:
     - Show which files conflict with `git diff --name-only --diff-filter=U`
     - Tell the user to resolve conflicts, then continue or abort
     - Remind them of the backup branch name
     - Do NOT attempt to auto-resolve conflicts
8. If changes were stashed in step 2, run `git stash pop` after success. If the pop conflicts, inform the user.
9. Summarize: backup branch name, strategy used and why (one sentence), number of commits integrated, current state.

## Output Format

Report each step as it runs. On success, show:

- Backup branch name
- Strategy used and why (one sentence)
- Number of commits integrated
- Current branch status

On failure (conflicts), show the conflicting files and remind the user of the backup branch and how to abort.
