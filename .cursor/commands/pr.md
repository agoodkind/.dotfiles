# Create Pull Request

Analyze the branch changes and create a pull request using the GitHub CLI. Write the PR the way you want the PR to read: clear, specific, and easy for a reviewer to follow.

## Critical Rules

- **Check Existing PR First**: ALWAYS run `gh pr view --json body,title` before creating or updating a PR
- **Preserve User Content**: If a PR already exists, keep the useful existing title and body content and only improve it
- **Draft Mode Required**: ALWAYS use `--draft` with `gh pr create`
- **Classify PR Type**: Determine whether the branch is a bug fix or a feature/iteration before writing
- **Report Failures Clearly**: If PR creation or update fails, report the error and stop immediately

## Branch Naming Convention

Branches should follow `username/TICKET-NUMBER/descriptive-name`, using slash-separated segments.

Example:

- ✅ `agoodkind/AG-10677/fix-silver-shimmer`

## Title Rules

- **Direct Statement**: Write a natural, specific title that states what changed
- **Body Carries Context**: Put ticket references and other supporting context in the body, usually in `### Summary`
- **Specific Language**: Name the behavior, code path, or surface area that changed

## PR Type Classification

Before writing the description, classify the PR:

**Bug Fix PR**: Something was broken and this fixes it:
- Users/devs experienced unexpected behavior
- Error states, crashes, incorrect data
- Regressions from previous changes

**Feature/Iteration PR**: Adding or extending functionality:
- New capabilities that didn't exist before
- Implementing spec'd behavior (even if prior code was scaffolded)
- Enhancements to existing features

## Description Structure

### For Bug Fix PRs

Flow: Observable problem → Root cause → What the PR does to fix it

1. Start with the observable problem or symptom (what users/devs saw)
2. Explain what was actually wrong in the code at a high level, with reviewer context
3. Describe what this PR does to fix it, focusing on the *nature* of the fix at a high level

### For Feature/Iteration PRs

Flow: Context/motivation → What it adds → Implementation approach

1. Start with context: what capability is being added and why it's needed
2. Describe what the PR implements as a capability being added or extended
3. Summarize the approach at a high level: how it works conceptually
4. Note dependencies or follow-up work if relevant

**Key difference**: Feature PRs describe the capability being built, the behavior being added, and the shape of the implementation. If prior code returned empty arrays as a placeholder, describe this work as the feature that now fills in that path.

**Reviewer context**: Describe what happens, how data flows, and what the user or system sees. Give the reviewer the context that the diff alone cannot supply.

**Section roles**: Give each section one clear job. Use `## What` for the behavior or capability change, `## Why` for the reviewer context or motivation, and `## How` for the implementation path, ownership boundary, and any important systems that stay unchanged.

**Section length**: In most PRs, `## What`, `## Why`, and `## How` each fit in one short paragraph. Add a second short paragraph only when the reviewer needs more context.

**How section**: Lead with the path you chose. Name the owning system first, explain how the flow works at the system level, and call out important systems or paths that remain unchanged when that boundary matters.

**Optional sections**: Add sections like `## Backward compatibility` or `## Testing` when they carry distinct reviewer context of their own.

**Tone**: Keep the writing readable, concrete, and easy to skim. Use prose in the PR body and let each section carry one clear idea forward.

## Code References (applies to BOTH title and description)

- Use single backticks for the small set of symbols, classes, files, endpoints, or flow IDs that materially help a reviewer orient.
- Let prose carry the explanation around those anchors.
- Use fenced code blocks only when a short snippet materially clarifies the behavior or interface being described.

## PR Media

If the PR description includes screenshots, videos, or before/after comparisons, apply the `pr-media` skill.

## Steps

1. **Check for existing PR**: Run `gh pr view --json body,title` to see if a PR already exists for this branch
   - If a PR exists, read its current title and description. Preserve user content when updating.
   - If no PR exists, proceed with creating a new one
2. Check for ticket number in this order:
   - First check current branch name (e.g. `agoodkind/AG-12345/fix-bug` → `AG-12345`)
   - Then check commit messages for ticket references
   - When no ticket appears in the branch name or commits, continue with the PR body as-is
3. Run `git log main..HEAD --oneline` to see commits on the branch
4. Run `git diff main...HEAD` to analyze all changes
5. **Classify PR type** using the guidance above
6. Craft a concise PR title using the title rules above
7. Write the PR body using the description structure above
   - If a ticket exists, include it in a short `### Summary` block near the top of the description: `Ticket: https://ag.atlassian.net/browse/AG-12345`
   - Infer the correct Atlassian subdomain from context or company; omit the ticket line if no ticket exists
8. If screenshots, videos, or before/after images are involved, apply the `pr-media` skill
9. Execute the command:
    - If PR exists: `gh pr edit --title '<title>' --body "<description>"` (use `required_permissions: ["network"]`)
    - If no PR: `gh pr create --draft --title '<title>' --body "<description>"` (use `required_permissions: ["network"]`)
    - **Title quoting**: Always use single quotes for `--title` so backticks pass through literally, for example `--title 'Permit `status` in `AchAccountController`'`
10. After PR is created or updated, follow the output format below.


OUTPUT FORMAT:

Use the example below as the stylistic reference for the PR title and body, and as the default level of detail for an ordinary PR.

After PR creation succeeds, output a short Slack message for the code review channel. Write it as a human one-liner that briefly says what the branch changes or fixes and ends with the PR URL on the same line.

Examples:

- "follow-up for dual-stack captive portal auth and clean up the login path https://..."
- "Fix duplicate silver entries by resetting list state https://..."
- "Extend payment validation to handle zero amounts https://..."

Then output the PR URL on a new line inside a single code fence for easy copying:

`https://....`

Then output the PR URL again on its own line with no markdown formatting:

https://....



EXAMPLE:

Title: "Use ScreenFlow for external account linking consent"

```markdown
### Summary
Ticket: https://....
Slack thread: http://....


## What
This adds a ScreenFlow-based consent path for external account linking in `server-api`.

Supported clients receive the `external_account_linking` deeplink and complete consent through the server-owned flow.

## Why
We want this consent path to live in the ScreenFlow stack so the server owns the transition, consent write, and completion behavior in one place.

Legacy clients still need the existing path, so this work has to coexist cleanly with the current resolver behavior.

## How
This uses ScreenFlow in `server-api`, and it ships without any `project-otter` change.

Supported clients get the `external_account_linking` deeplink, the server records consent during the agree transition, and the flow advances to the completion screen after the RPC succeeds.

## Backward compatibility
Clients below the version gate stay on the existing consent path, so the current `ExternalAccountConsent` resolver, `ConsentExternalAccountDataUse` mutation, and `ExternalAccountsAdded` resolver continue to work as they do today.
```

