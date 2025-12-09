# Create Pull Request

Analyze the branch changes and create a pull request using the GitHub CLI, following these strict guidelines:

## Critical Rules

- **Prose Paragraphs**: Write descriptions as concise prose, NOT bullet lists
- **Present Tense**: Use present tense throughout
- **2-3 Paragraphs Max**: Keep descriptions brief and focused
- **Flow**: Observable problem → Root cause → What the PR does to fix it

## Title Rules

- **No Prefix Labels**: Avoid feat, refactor, fix, etc.
- **Direct Statement**: Use imperative mood, state what changed
- **Specific**: Be specific about what changed and where

## Description Structure

1. Start with the observable problem or symptom (what users/devs saw)
2. Explain what was actually wrong in the code
3. Describe what this PR does to fix it
4. Include implementation details when necessary
5. Acknowledge friction or concerns where relevant

## Prohibited Patterns

- Bullet lists or itemized formats
- Separate "benefits" sections
- Teaching how systems work
- Explaining obvious things
- "This PR fixes..." as an opener
- Filler phrases like "improves the user experience"
- Purpose clauses explaining why ("to improve...", "for better...")

## Branch Naming Convention

Branches must follow: `username/TICKET-NUMBER/descriptive-name` (with slashes, not dashes)

Examples:
- ✅ `agoodkind/AG-10677/fix-silver-shimmer`
- ❌ `agoodkind-AG-10677-fix-silver-shimmer` (wrong: uses dashes)

## Steps

1. Check for ticket number in this order:
   - First check current branch name (e.g. `agoodkind/AG-12345/fix-bug` → `AG-12345`)
   - Then check commit messages for ticket references
   - If not found, ask the user for the ticket number
2. Run `git log main..HEAD --oneline` to see commits on the branch
3. Run `git diff main...HEAD` to analyze all changes
4. Craft a concise PR title with ticket prefix: `[AG-12345] Your title here` (imperative mood, no feat/fix prefixes)
5. Write 2-3 paragraph description following the flow: symptom → root cause → fix
6. Add ticket link at the end of description: `Ticket: https://ag.atlassian.net/browse/AG-12345` (infer the correct alassian subdomain from context or company)
7. Execute `gh pr create --title "<title>" --body "<description>"` (use `required_permissions: ["network"]`)
8. After PR is created, output a Slack message for the review request channel

## Examples

✅ Good Title:
- "[AG-1234] Add amount validation to silver processor
- "[PLAT-567] Reset list state on pull-to-refresh"

✅ Good Description:
"The silver list was showing duplicate entries after pull-to-refresh. The issue was that fetchSilverElectrons was appending results instead of replacing them when the offset was zero. This PR resets the list state before fetching when triggered by refresh.

Ticket: https://ag.atlassian.net/browse/AG-1234"

❌ Bad Title:
- "fix: Fix silver bug"
- "[AG-1234] fix: Update silver logic to improve reliability"
- "Update silver logic" (missing ticket)

❌ Bad Description:
"This PR fixes a bug in the silver fetching logic. Previously the code was appending results incorrectly. Now it properly resets state. This improves the user experience by preventing confusion from duplicate items."

(Missing ticket link at the end)

## Output Format

Create a PR with a single-line title and 2-3 paragraph prose description. If the PR creation fails, report the error. No bullet lists, no prefix labels, no benefit statements.

After PR creation succeeds, output a Slack message for the code review channel:

```
<Symptom/benefit> by <action>: <PR_URL>
```

Keep it concise - one line with symptom/action format and URL. Start with the observable benefit or problem being solved, followed by "by" and the action taken. Examples:
- "Speed up CI checks by combining sorbet steps: https://..."
- "Fix duplicate silver entries by resetting list state on refresh: https://..."
- "Reduce memory usage by lazy loading images: https://..."

If there's a ticket, optionally prefix with `[TICKET-123]` but keep the symptom/action format.

