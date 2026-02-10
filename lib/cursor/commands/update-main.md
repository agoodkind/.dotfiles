# Safely Update Branch with Latest Main

Fetch the latest `origin/main` and rebase (or merge) the current branch onto it, with a backup branch created first.

## Critical Rules

- **Always fetch first**: Run `git fetch origin main` explicitly. Never rely on a stale local `main`.
- **Always back up**: Create a backup branch before any rebase or merge operation.
- **Never force push**: Do not force push unless the user explicitly requests it.
- **Dirty tree protection**: If the working tree is dirty, stash changes before proceeding and pop them after.
- **Choose strategy per situation**: Do not default to rebase or merge. Analyze the branch state and recommend the best approach, then confirm with the user before proceeding.

## Steps

1. Run `git branch --show-current` to get the current branch name. Abort if on `main` directly (tell the user to check out their feature branch first).
2. Run `git status --porcelain` to check for uncommitted changes.
   - If dirty, run `git stash push -m "pre-update-main auto-stash"` and note that a stash was created.
3. Create a backup branch: `git branch backup/<current-branch>/<timestamp>` where timestamp is `$(date +%Y%m%d-%H%M%S)`. Confirm the backup branch name to the user.
4. Fetch latest main: `git fetch origin main`. Show the fetch output so the user can see what changed.
5. Analyze the situation and recommend rebase vs merge. Consider:
   - `git log --oneline HEAD..origin/main` (how many new commits on main)
   - `git log --oneline origin/main..HEAD` (how many local commits)
   - Whether the branch has been pushed/shared (check `git log --oneline @{u}..HEAD 2>/dev/null`)
   - Whether local commits are clean, linear work (favors rebase) or contain merges already (favors merge)
   - Present the recommendation with reasoning, then ask the user to confirm before proceeding.
6. Execute the chosen strategy:
   - **Rebase**: `git rebase origin/main`
   - **Merge**: `git merge origin/main`
   - If it succeeds, confirm and show the new log with `git log --oneline -5`
   - If conflicts occur:
     - Show which files conflict with `git diff --name-only --diff-filter=U`
     - Tell the user to resolve conflicts, then `git rebase --continue` or `git merge --continue`
     - Mention they can abort with `git rebase --abort` or `git merge --abort` to return to the backup state
     - Do NOT attempt to auto-resolve conflicts
7. If changes were stashed in step 2, run `git stash pop` after success. If the pop conflicts, inform the user.
8. Summarize what happened: backup branch name, strategy used, number of new commits from main, current state.

## Output Format

Report each step as it runs. On success, show:

- Backup branch name
- Strategy used (rebase or merge) and why
- Number of commits integrated
- Current branch status

On failure (conflicts), show the conflicting files and remind the user of the backup branch and how to abort.
