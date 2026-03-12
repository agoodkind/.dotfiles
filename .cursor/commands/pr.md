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

**Reviewer context**: Describe the approach empirically: what happens, how data flows, and what the user or system sees. Give the reviewer the context that the diff alone cannot supply.

**Structure for readability**: Use sectioned prose like `### Summary`, `## What`, `## Why`, and `## How` when it helps the description read cleanly. Include caveats, implementation notes, or a test matrix only when they materially help a reviewer.

**Tone**: Keep the writing readable, concrete, and easy to skim. Use prose in the PR body and let each section carry one clear idea forward.

## Code References (applies to BOTH title and description)

- Use single backticks for all code symbols, function names, variable names, class names, file names, reserved keywords, CLI flags, etc. This applies in both the PR title and the description body.
- Use fenced code blocks for multi-line snippets (description only):

  ```ruby
  def example
    # relevant code
  end
  ```

- Reference specific methods, classes, or values that changed
- Code references add precision. Use them when naming things matters.
- **Behavior in Plain Language**: When referencing a change, name the symbol with a backtick and describe what it does in plain language. Let the description explain behavior and intent.

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

Use the example below as the stylistic reference for the PR title and body.

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

Title: "Follow up for dual-stack captive portal authorization in `CaptivePortal`"

```markdown
### Summary
Issue: https://...
Ticket: https://....
Upstream PR(s): https://github.com/.../.../pull/...
Downstream PR(s): https://github.com/.../.../pull/...
Datadog link: http://....
Slack thread: http://....


## What
This follow-up finishes the dual-stack Captive Portal path on top of `captive-portal-ipv6`.

It fixes client IP detection behind the local API proxy, restores IPv6 neighbor discovery for session expansion, and warm-starts roaming by immediately authorizing sibling addresses that are already known for the same MAC at login time.

## Why
I had some free time to run a fuller test matrix on top of the upstream branch, and that turned up a few gaps that were still visible in practice.

This branch tries to address the remaining review points from @AdSchellevis and the bugs that showed up during testing, especially around dual-stack login, IPv6-first login, and secondary IPv6 addresses after authentication.

## How
The API now resolves the real client address correctly when requests arrive through the local dispatcher. The `allow` path authorizes the connected address immediately, also authorizes sibling addresses that are already visible for the same MAC, and still leaves the background sync path in place for later discovery and cleanup.

## Test Matrix

| Case | Pre-auth | Auth path | Post-auth result |
|---|---|---|---|
| IPv4-only | Redirects to IPv4 portal host | IPv4 portal host | IPv4 egress works |
| Dual-stack, cold cache | IPv4 redirects to IPv4 portal host, IPv6 redirects to IPv6 portal host | IPv4 portal host | The login address works immediately. Sibling addresses join as they become visible. |
| Dual-stack, warm cache | IPv4 redirects to IPv4 portal host, IPv6 redirects to IPv6 portal host | IPv4 portal host | The login address works immediately. Already-known sibling IPv6 addresses work immediately. |
| Dual-stack, warm cache | IPv4 redirects to IPv4 portal host, IPv6 redirects to IPv6 portal host | IPv6 portal host | The login address works immediately. Already-known sibling IPv4 and IPv6 addresses work immediately. |
| IPv6-only preferred (DHCPv4 option 108) | Redirects to IPv6 portal host | IPv6 portal host | The login address works immediately. Already-known sibling IPv6 addresses work immediately. |
| NAT64 / DNS64 / PREF64 | IPv6-only preferred client uses DNS64 and RA `nat64prefix` | IPv6 portal host | IPv6-native and NAT64 HTTP access work after authentication. |
| Multi-address IPv6 | Session starts on one address | Background reconciliation | Newly observed SLAAC, privacy, and temporary IPv6 addresses join the same session later. |

Note: Right after link-up, clients often authenticate before they have exercised every candidate IPv6 source address. The login path can only warm-start what is already visible for the MAC, and the background reconciliation loop still picks up additional addresses later as the client starts using them. In CLI testing there was still one edge case where the first immediate post-login request from the DHCPv6 `/128` source needed a retry, while the privacy or SLAAC-style sibling addresses and the NAT64 path behaved as expected.

During testing I also confirmed that standard outbound NAT66 on `opnsense-dev` is required for downstream guest IPv6 egress to work consistently.
```

