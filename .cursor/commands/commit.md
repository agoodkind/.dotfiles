# Stage and Commit Changes

Commit changes with a generated commit message. If files are already staged, commit those. If nothing is staged, stage and commit all unstaged changes. Follow these strict guidelines:

## Critical Rules

- **No Explanations**: State the change only—no justification or elaboration
- **No Filler Words**: Skip pleasantries and unnecessary phrases
- **No Prefix Labels**: Avoid feat, refactor, fix, etc.
- **Direct Statements**: Use imperative mood, state what changed

## Prohibited Patterns

### Purpose Clauses

- Avoid "to [benefit]" or "for [benefit]" clauses (e.g., "to improve reliability", "for better performance")
- State what changed, not why it was changed

### Explanatory Language

- Avoid benefit words: "improves", "enhances", "streamlines", "ensures", "allows", "enables"
- Avoid "This change [verb]" constructions (e.g., "This change ensures", "This change allows")
- Avoid explanatory sentences that describe the mechanism or reasoning

### Structure Issues

- No prefix labels: "feat:", "fix:", "refactor:", etc.
- No multi-sentence explanations
- No bullet lists or multi-line formats
- Avoid conjunctions that explain mechanism: "while", "by", "based on" when used to justify

### Vague or Overly Detailed Messages

- **Too Vague**: Avoid generic terms without specifics (e.g., "Update logic", "Fix bug", "Improve code", "Refactor")
- **After Removing Fluff**: Removing fluff must not leave a vague message. Be specific about what changed.
- **Too Detailed**: Avoid step-by-step implementation details, multi-line explanations, or describing the mechanism
- Find the balance: State what changed and where, without explaining how or why

### Specific Phrases to Avoid

- "it's important to note"
- "this ensures"
- "this enhances"
- "improves" (as justification)
- "fixes" (as a prefix)
- "adds" (as a prefix)
- "updates" (as a prefix)
- "ensures"
- "enhances"
- Any justification or explanation of why the change was made

## Commit Scope & Splitting

**Default behavior**: If there's no prior conversation context (blank chat), commit all staged/unstaged changes together.

**With conversation context**: If we discussed specific changes, evaluate which files belong:

- **Relevance Check**: Only commit files related to the task discussed
- **Unrelated Changes**: If unrelated files were modified, exclude them or commit separately
- **Logical Units**: Each commit should represent one logical change
- **Ask When Uncertain**: If it's unclear whether a file should be included, ask before committing

Signs you should split commits:

- Changes affect unrelated features or systems
- The commit message would need "and" to describe all changes
- Some files are cleanup/refactoring while others are new functionality

## Files to Skip

Use common sense to exclude files that shouldn't be committed:

- Log files (*.log, debug output, etc.)
- Build artifacts and compiled output
- Temporary files and caches
- IDE/editor-generated files not in .gitignore
- Lock files that weren't intentionally changed
- Large binary files accidentally created

When in doubt, ask before including a suspicious file.

## Steps

1. Run `git status` to check for staged and unstaged changes
2. Review the changed files - skip any that shouldn't be committed (logs, artifacts, etc.)
3. If files are staged, run `git diff --staged` to analyze them
4. If nothing is staged but unstaged changes exist, stage appropriate files with `git add` (skip files that shouldn't be committed)
5. Craft a single, concise subject line that states what changed
6. Be specific about what changed and where (file names, functions, etc.)
7. Use imperative mood
8. Do NOT include explanations, justifications, or benefit statements
9. Avoid all prohibited patterns listed above
10. Execute `git commit -m "<message>"` with the generated message (use `required_permissions: ["all"]`)
11. If there are remaining relevant changes, repeat for the next logical commit

## Examples

✅ Good:

- "Remove unused import"
- "Add amount validation to silver processor"
- "Add retry mechanism for failed requests"
- "Refactor error handling in GraphQL resolver"
- "Update syslog_message field in unbound Logstash filter and adjust field removal"
- "Update Ansible configuration and playbooks"
- "Add Gemfile.lock, VSCode extensions, and settings configuration"
- "Add requirements.yml for Ansible playbooks to include community.proxmox and ansible.utils collections"
- "Remove duplicate timeout and gather_timeout settings from ansible.cfg"
- "Remove SKIP_PATTERNS handling from sync-semaphore.sh"
- "Remove redundant reboot parameters in deploy-mwan.yml"

❌ Bad:

- "feat: Add retry mechanism to improve reliability" (Prefix "feat:", fluff "to improve")
- "This PR fixes a bug that was causing issues" (Conversational, vague "bug", past tense)
- "Update validation to ensure data integrity" (Fluff "to ensure")
- "Refactor: This enhances code maintainability" (Prefix "Refactor:", fluff "enhances")
- "Comment out stdout debug output in Logstash configuration file to streamline logging." (Fluff "to streamline")
- "Add playbook to update Traefik configuration\n Create systemd service template for Traefik\n Add .gitignore for Traefik configuration files..." (Multi-line format)
- "Add delete_existing_vm option to create-vm.yml for improved VM management" (Fluff "for improved")
- "Fix bug" (Too vague)
- "Update logic" (Too vague)
- "Refactor code" (Too vague)
- "Update silver validation logic" (Too vague)
- "Refactor netfilter modules to improve consistency and functionality" (Fluff: "to improve")
- "Changed target functions, enhancing clarity in their purpose" (Fluff: "enhancing clarity")
- "Improved the handling of DSCP and TTL values" (Fluff: "Improved")
- "Update validation logic to ensure data integrity" (Fluff: "to ensure")
- "Refactor code to enhance maintainability and streamline operations" (Fluff: "enhance", "streamline" - without fluff: "Refactor code" is meaningless)
- "Add error handling to improve user experience" (Fluff: "to improve")
- "Reintroduce VM existence check in Debian playbook with improved clarity. Uncommented tasks for setting facts about existing VMs, while temporarily commenting out VM stop and destroy tasks to streamline the redeployment process. This change enhances the overall flow and maintainability of the playbook." (Multi-sentence, fluff: "improved", "streamline", "enhances")
- "Refactor deploy-mwan.yml to streamline service management and enhance clarity. Removed redundant notifications for service reloads and restarts, and added explicit service start commands based on certificate checks. Improved overall flow by consolidating service enabling and starting tasks." (Multi-sentence, fluff: "streamline", "enhance", "Improved")
- "Update VM creation condition in Debian playbook to check for undefined VMID. This change ensures that the VM creation block is only executed when a new VMID is available, improving the logic and flow of the playbook." (Multi-sentence, fluff: "ensures", "improving")
- "Enable privilege escalation in Ansible configuration by setting 'become' to True. This change allows tasks to run with elevated permissions, enhancing the flexibility of playbook execution." (Multi-sentence, fluff: "allows", "enhancing")
- "Comment out temporary VM management tasks in the Debian VM playbook to streamline the deployment process. This change enhances clarity and maintainability by removing unnecessary steps related to stopping and destroying existing VMs before creation." (Multi-sentence, fluff: "streamline", "enhances")

## Output Format

Commit staged changes (or stage and commit all unstaged changes if nothing is staged), generate a single-line commit message, and execute the commit. If ESLint or other pre-commit hooks fail, fix the issues and retry. No prefix labels, no explanations, no multi-line format.
