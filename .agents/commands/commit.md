# Stage and Commit Changes

Commit changes with a generated commit message. If files are already staged, commit those. If nothing is staged, stage and commit all unstaged changes.

## Message Rules

- Single subject line, imperative mood, no trailing period
- Start directly with a verb and subject -- no label, colon, or scope prefix before the message
- State what changed and where; omit why it was changed or what benefit it brings
- Be specific: name the file, function, or system affected
- Keep it to one sentence; no multi-line format, no bullet lists, no body section
- A specific message after removing fluff is better than a vague one

## Examples

- `Remove unused import in auth middleware`
- `Add amount validation to silver processor`
- `Add retry mechanism for failed requests`
- `Refactor error handling in GraphQL resolver`
- `Update syslog_message field in unbound Logstash filter and adjust field removal`
- `Add Gemfile.lock, VSCode extensions, and settings configuration`
- `Add requirements.yml for Ansible playbooks to include community.proxmox and ansible.utils collections`
- `Remove duplicate timeout and gather_timeout settings from ansible.cfg`
- `Remove SKIP_PATTERNS handling from sync-semaphore.sh`
- `Remove redundant reboot parameters in deploy-mwan.yml`

## Commit Scope & Splitting

**Default behavior**: If there's no prior conversation context (blank chat), commit all staged/unstaged changes together.

**With conversation context**: If we discussed specific changes, evaluate which files belong:

- Only commit files related to the task discussed
- If unrelated files were modified, exclude them or commit separately
- Each commit should represent one logical change
- If it's unclear whether a file should be included, ask before committing

Split into separate commits when:

- Changes affect unrelated features or systems
- The commit message would need "and" to describe all changes
- Some files are cleanup while others are new functionality

## Files to Skip

Use common sense to exclude files that shouldn't be committed:

- Log files, debug output
- Build artifacts and compiled output
- Temporary files and caches
- IDE/editor-generated files not in .gitignore
- Lock files that weren't intentionally changed
- Large binary files accidentally created

When in doubt, ask before including a suspicious file.

## Steps

1. Before doing anything else, run `git rev-parse --show-toplevel` and `git remote get-url origin` to confirm you are operating in the correct repository. In multi-repo workspaces, the shell's working directory may point to a different repo than the one being discussed.
2. Run `git fetch origin main` to ensure the remote ref is up to date.
3. Run `git status` to check for staged and unstaged changes.
4. Run `git diff $(git merge-base HEAD origin/main) HEAD` to see everything that diverges from main. Use this as the ground truth for what is in scope.
5. Review the changed files and skip any that should not be committed.
6. If files are staged, run `git diff --staged` to confirm what will be included.
7. If nothing is staged but unstaged changes exist, stage appropriate files with `git add`.
8. Craft a single, concise subject line that states what changed.
9. Execute `git commit -m "<message>"` with the generated message (use `required_permissions: ["all"]`).
10. If there are remaining relevant changes, repeat for the next logical commit.

If pre-commit hooks fail, fix the issues and retry.
