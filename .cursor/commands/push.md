# Push Changes

Push committed changes to the remote repository.

## Rules

- Only push if there are commits to push
- Use the current branch name
- Do not force push unless explicitly requested
- Show the push output

## Steps

1. Check current branch with `git branch --show-current`
2. Check if there are commits to push with `git log @{u}..HEAD` or `git status`
3. If there are commits, execute `git push`
4. If push fails due to remote being ahead, inform the user they may need to pull first
5. Show the push output

## Output Format

Push the current branch to the remote. If successful, confirm the push. If it fails, report the error and suggest next steps.
