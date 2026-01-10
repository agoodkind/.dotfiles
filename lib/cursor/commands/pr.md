# Create Pull Request

Analyze the branch changes and create a pull request using the GitHub CLI, following these strict guidelines:

## Critical Rules

- **Check Existing PR First**: ALWAYS run `gh pr view --json body,title` to check for an existing PR description before creating or updating—preserve user content and only enhance it
- **Draft Mode Required**: ALWAYS use `--draft` flag when executing `gh pr create` - never create non-draft PRs
- **Prose Paragraphs**: Write descriptions as concise prose, NOT bullet lists
- **Present Tense**: Use present tense throughout
- **2-3 Paragraphs Max**: Keep descriptions brief and focused
- **Flow**: Observable problem → Root cause → What the PR does to fix it

## Title Rules

- **No Prefix Labels**: Avoid feat, refactor, fix, etc.
- **No Ticket Numbers**: Never include ticket numbers in the title - they go in the body
- **Direct Statement**: Use imperative mood, state what changed
- **Specific**: Be specific about what changed and where

## Description Structure

1. Start with the observable problem or symptom (what users/devs saw)
2. Explain what was actually wrong in the code
3. Describe what this PR does to fix it
4. Include implementation details when necessary
5. Acknowledge friction or concerns where relevant

## Code References

- Use inline code for identifiers: `method_name`, `ClassName`, `variable`
- Use fenced code blocks for multi-line snippets:

  ```ruby
  def example
    # relevant code
  end
  ```

- Reference specific methods, classes, or values that changed
- Code references add precision—use them when naming things matters

## Before/After Image Table Conversion

When the PR description (existing or being created) contains before/after images, automatically convert them to a side-by-side table format.

**Detect these patterns:**

- "Before" followed by an `<img>` tag or markdown image `![]()`
- "After" followed by an `<img>` tag or markdown image `![]()`
- Variations like "Before:", "**Before**", "### Before", etc.

**Convert to this format:**

```markdown
| Before | After |
|--------|-------|
| ![alt](image_url) | ![alt](image_url) |
```

**Conversion rules:**

- Extract the `src` attribute from `<img>` tags and convert to markdown image syntax
- Use the `alt` attribute as the alt text (or "Before"/"After" if no alt provided)
- Drop `width`, `height`, and other HTML attributes (GitHub auto-sizes in tables)
- If multiple before/after pairs exist, create multiple rows or separate tables

**Example conversion:**

Input:

```
Before
<img width="1206" alt="Old UI" src="https://github.com/user-attachments/assets/abc123" />
After
<img width="545" alt="New UI" src="https://github.com/user-attachments/assets/def456" />
```

Output:

```markdown
| Before | After |
|--------|-------|
| ![Old UI](https://github.com/user-attachments/assets/abc123) | ![New UI](https://github.com/user-attachments/assets/def456) |
```

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

1. **Check for existing PR**: Run `gh pr view --json body,title` to see if a PR already exists for this branch
   - If a PR exists, read its current title and description—preserve user content when updating
   - If no PR exists, proceed with creating a new one
2. Check for ticket number in this order:
   - First check current branch name (e.g. `agoodkind/AG-12345/fix-bug` → `AG-12345`)
   - Then check commit messages for ticket references
   - If not found, proceed without a ticket (do not ask the user)
3. Run `git log main..HEAD --oneline` to see commits on the branch
4. Run `git diff main...HEAD` to analyze all changes
5. Craft a concise PR title without ticket prefix (imperative mood, no feat/fix prefixes)
6. Write 2-3 paragraph description following the flow: symptom → root cause → fix
7. If before/after images are provided (in user input or existing PR description), convert them to the table format described above
8. Add ticket link at the start of description when a ticket exists: `Ticket: https://ag.atlassian.net/browse/AG-12345` (infer the correct atlassian subdomain from context or company); omit if no ticket
9. Execute the command:
   - If PR exists: `gh pr edit --title "<title>" --body "<description>"` (use `required_permissions: ["network"]`)
   - If no PR: `gh pr create --draft --title "<title>" --body "<description>"` (use `required_permissions: ["network"]`)
10. After PR is created/updated, output:
    - A Slack message for the review request channel
    - The PR URL in a single-line code fence for easy copying

## Examples

✅ Good Title:

- "Add amount validation to silver processor"
- "Reset list state on pull-to-refresh"

✅ Good Description:
"Ticket: <https://ag.atlassian.net/browse/AG-1234>

The silver list was showing duplicate entries after pull-to-refresh. The issue was that `fetchSilverElectrons` was appending results instead of replacing them when `offset` was zero. This PR resets the list state before fetching when triggered by refresh."

❌ Bad Title:

- "fix: Fix silver bug"
- "[AG-1234] Add amount validation to silver processor"
- "Update logic" (too vague)

❌ Bad Description:
"This PR fixes a bug in the silver fetching logic. Previously the code was appending results incorrectly. Now it properly resets state. This improves the user experience by preventing confusion from duplicate items."

(Missing ticket link at the end)

## Output Format

Create a PR with a single-line title and 2-3 paragraph prose description. If the PR creation fails, report the error. No bullet lists, no prefix labels, no benefit statements.

After PR creation succeeds, output a Slack message for the code review channel:

```
<Symptom/benefit> <PR_URL>
```

Keep it concise—one line with the symptom or benefit followed by the URL. If calling out the action materially clarifies what changed, you can append "by <action>" before the URL. Examples:

- "Add JIRA PR badge https://..."
- "Fix duplicate silver entries by resetting list state: https://..."
- "Reduce memory usage https://..."

Then output the PR URL in a single-line code fence for easy copying:

`<PR_URL>`
