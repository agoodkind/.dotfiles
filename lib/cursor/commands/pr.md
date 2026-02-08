# Create Pull Request

Analyze the branch changes and create a pull request using the GitHub CLI, following these strict guidelines:

## Critical Rules

- **Check Existing PR First**: ALWAYS run `gh pr view --json body,title` to check for an existing PR description before creating or updating—preserve user content and only enhance it
- **Draft Mode Required**: ALWAYS use `--draft` flag when executing `gh pr create` - never create non-draft PRs
- **Prose Paragraphs**: Write descriptions as concise prose, NOT bullet lists
- **Present Tense**: Use present tense throughout
- **2-3 Paragraphs Max**: Keep descriptions brief and focused
- **Classify PR Type**: Determine if this is a bug fix or feature/iteration PR (see below)

## Title Rules

- **No Prefix Labels**: Avoid feat, refactor, fix, etc.
- **No Ticket Numbers**: Never include ticket numbers in the title - they go in the body
- **Direct Statement**: Use imperative mood, state what changed
- **Specific**: Be specific about what changed and where

## PR Type Classification

Before writing the description, classify the PR:

**Bug Fix PR** — Something was broken and this fixes it:
- Users/devs experienced unexpected behavior
- Error states, crashes, incorrect data
- Regressions from previous changes

**Feature/Iteration PR** — Adding or extending functionality:
- New capabilities that didn't exist before
- Implementing spec'd behavior (even if prior code was scaffolded)
- Enhancements to existing features

## Description Structure

### For Bug Fix PRs

Flow: Observable problem → Root cause → What the PR does to fix it

1. Start with the observable problem or symptom (what users/devs saw)
2. Explain what was actually wrong in the code (high-level, not a code walkthrough)
3. Describe what this PR does to fix it
4. Focus on the *nature* of the fix, not a list of changed methods

### For Feature/Iteration PRs

Flow: Context/motivation → What it adds → Implementation approach

1. Start with context: what capability is being added and why it's needed
2. Describe what the PR implements (the "what", not framed as fixing)
3. Summarize the approach at a high level — how it works conceptually
4. Note dependencies or follow-up work if relevant

**Key difference**: Feature PRs describe what's being *built*, not what was *broken*. Avoid framing scaffolded/stub code as "bugs" — if prior code returned empty arrays as placeholder, that's not a symptom to fix, it's a feature to implement.

**Avoid laundry lists**: Don't enumerate every method, class, or file changed. Instead, describe the approach empirically — what happens, how data flows, what the user sees. Reviewers can see the diff; they need context, not a changelog.

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

## Uploading Images and Videos

When including screenshots or videos in PR descriptions, upload files to the repo's orphan `agoodkind/pr-assets` branch. This pushes via the GitHub Contents API with timestamped names. Since the asset lives in the same repo, GitHub renders it inline, even for private repos. First run per-repo auto-creates the orphan branch.

If the `gh_upload` MCP tool is available, use it with `file` (absolute path) and optional `cwd`. It returns `{ "url": "..." }`.

Otherwise, run the shell command directly:

```bash
gh upload screenshot.png
gh upload demo.mp4
```

Both return the raw download URL. Use it in markdown as `![alt](URL)` or `[filename](URL)`.

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
- Laundry lists of methods, classes, or files changed
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
5. **Classify PR type**: Based on the changes, determine if this is a bug fix or feature/iteration PR
   - Look at commit messages, branch name hints, and nature of changes
   - New methods/classes/resolvers → likely feature
   - Correcting existing behavior → likely bug fix
6. Craft a concise PR title without ticket prefix (imperative mood, no feat/fix prefixes)
7. Write 2-3 paragraph description using the appropriate flow for the PR type:
   - Bug fix: symptom → root cause → fix
   - Feature: context → what it adds → implementation approach
8. If before/after images are provided (in user input or existing PR description), convert them to the table format described above
9. Add ticket link at the start of description when a ticket exists: `Ticket: https://ag.atlassian.net/browse/AG-12345` (infer the correct atlassian subdomain from context or company); omit if no ticket
10. Execute the command:
    - If PR exists: `gh pr edit --title "<title>" --body "<description>"` (use `required_permissions: ["network"]`)
    - If no PR: `gh pr create --draft --title "<title>" --body "<description>"` (use `required_permissions: ["network"]`)
11. After PR is created/updated, output:
    - A Slack message for the review request channel
    - The PR URL in a single-line code fence for easy copying

## Examples

### Bug Fix PR

✅ Good Title: "Reset list state on pull-to-refresh"

✅ Good Description:
"Ticket: <https://ag.atlassian.net/browse/AG-1234>

The silver list was showing duplicate entries after pull-to-refresh. The issue was that `fetchSilverElectrons` was appending results instead of replacing them when `offset` was zero. This PR resets the list state before fetching when triggered by refresh."

### Feature/Iteration PR

✅ Good Title: "Implement account logos in spending insights resolvers"

✅ Good Description:
"Ticket: <https://ag.atlassian.net/browse/ST-10627>
Depends on: #24675

Spending insights screens need to display bank logos for accounts that have transactions in the selected period. This PR implements the resolver logic across three spending insights endpoints.

Each resolver identifies which accounts have transaction data, maps those to their institution, and returns up to 4 logos. Chime accounts are prioritized when present, followed by external accounts. The Hub resolver shows logos for all accounts in the selected scope, while the breakdown and history detail views filter to only accounts with actual transactions in the result set."

❌ Bad Feature PR (laundry list):
"This PR adds eight helper methods to `utils.rb` including `institution_name_to_icon`, `extract_external_account_uuids_with_data`, `extract_unique_institutions`, and `build_account_logos`. It also updates `hub_resolver.rb`, `main_category_breakdown_resolver.rb`, and `spending_history_details_resolver.rb`."

(Wrong: lists methods and files instead of explaining what happens)

❌ Bad Feature PR (framed as bug fix):
"The types were added in #24675 but returned empty arrays without resolver implementation. This PR fixes the empty arrays by implementing the actual resolver logic."

(Wrong: frames placeholder code as a bug rather than describing the feature being built)

### Common Bad Patterns

❌ Bad Title:

- "fix: Fix silver bug"
- "[AG-1234] Add amount validation to silver processor"
- "Update logic" (too vague)

❌ Bad Description:
"This PR fixes a bug in the silver fetching logic. Previously the code was appending results incorrectly. Now it properly resets state. This improves the user experience by preventing confusion from duplicate items."

(Missing ticket link, filler phrases)

## Output Format

Create a PR with a single-line title and 2-3 paragraph prose description. If the PR creation fails, report the error. No bullet lists, no prefix labels, no benefit statements.

After PR creation succeeds, output a Slack message for the code review channel:

```
<What it does/fixes> <PR_URL>
```

Keep it concise—one line describing the change followed by the URL. Examples:

- Bug fix: "Fix duplicate silver entries by resetting list state https://..."
- Feature: "Add account logos to spending insights resolvers https://..."
- Iteration: "Extend payment validation to handle zero amounts https://..."

Then output the PR URL in a single-line code fence for easy copying:

`<PR_URL>`
