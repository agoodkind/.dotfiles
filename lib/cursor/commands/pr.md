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

## Steps

1. Run `git log main..HEAD --oneline` to see commits on the branch
2. Run `git diff main...HEAD` to analyze all changes
3. Craft a concise PR title (imperative mood, no prefixes)
4. Write 2-3 paragraph description following the flow: symptom → root cause → fix
5. Execute `gh pr create --title "<title>" --body "<description>"` (use `required_permissions: ["network"]`)

## Examples

✅ Good Title:
- "Add amount validation to transaction creation"
- "Reset list state on pull-to-refresh"

✅ Good Description:
"The transaction list was showing duplicate entries after pull-to-refresh. The issue was that fetchTransactions was appending results instead of replacing them when the offset was zero. This PR resets the list state before fetching when triggered by refresh."

❌ Bad Title:
- "fix: Fix transaction bug"
- "Update transaction logic to improve reliability"

❌ Bad Description:
"This PR fixes a bug in the transaction fetching logic. Previously the code was appending results incorrectly. Now it properly resets state. This improves the user experience by preventing confusion from duplicate items."

## Output Format

Create a PR with a single-line title and 2-3 paragraph prose description. If the PR creation fails, report the error. No bullet lists, no prefix labels, no benefit statements.

