# Push Changes

Push committed changes to the remote repository.

## Rules

- Only push if there are commits to push
- Use the current branch name
- Do not force push unless explicitly requested
- Show the push output

## Steps

1. Before doing anything else, run `git rev-parse --show-toplevel` and `git remote get-url origin` to confirm you are operating in the correct repository and that `origin` points to the expected remote URL. In multi-repo workspaces, the shell's working directory may point to a different repo than the one being discussed.
2. Check the current branch with `git branch --show-current`.
3. Check if there are commits to push with `git log @{u}..HEAD` or `git status`.
4. If there are commits, execute `git push`.
5. If the push fails because the remote is ahead, inform the user they may need to pull first.
6. Show the push output.

## Output Format

Push the current branch to the remote. If successful, confirm the push. If it fails, report the error and suggest next steps.
