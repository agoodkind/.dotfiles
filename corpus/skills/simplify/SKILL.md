---
name: simplify
description: Review changed code for reuse, simplicity, and efficiency, then apply focused improvements without changing intended behavior.
---

<!-- Adapted from Claude Code 2.1.318 bundled /simplify. -->

# Simplify

Improve the quality of changed code by reviewing reuse, simplification,
efficiency, and the underlying mechanism, then fixing the findings. Limit this
review to code quality. Preserve intended behavior and follow the repository
instructions and conventions.

## Phase 0: Gather the diff

Run `git diff @{upstream}...HEAD` to get the unified diff under review. If the
branch does not have an upstream, compare HEAD with the repository's actual
base branch, or use `git diff HEAD~1` for a single-commit review.

If the working tree contains uncommitted changes, or the range diff is empty,
also run `git diff HEAD` and include those changes. If the user specifies a
pull request, branch, file path, or commit range, review that target instead.
Treat the resulting diff as the review scope.

## Phase 1: Review the changes

Use four independent review agents, one per area below. Give each agent the
diff. Run the agents concurrently within the host's limits, scheduling the
remaining reviewers as slots become available. If review agents are
unavailable, perform all four checks directly.

Require each finding to include `file`, `line`, a one-line `summary`, and the
concrete cost: what the code duplicates, wastes, or makes harder to maintain.

### Reuse

Flag new code that implements something the codebase already provides. Search
shared utilities and files adjacent to the change. Identify the existing
helper to call instead.

### Simplification

Flag unnecessary complexity that the diff adds: redundant or derivable state,
copied code with slight variations, deep nesting, and dead code left behind.
Identify the simpler form that does the same job.

### Efficiency

Flag wasted work that the diff introduces: redundant computation, repeated
input/output operations, independent operations that run sequentially, and
blocking work during startup or frequently executed operations. Identify the
cheaper alternative.

Flag long-lived closures or objects with captured environments when evidence
shows that they unnecessarily retain large values. Identify the retained data
and its lifetime. Prefer a class or struct that copies only the fields it
needs when that change reduces retention.

### Underlying mechanism

Check whether each change fixes the root cause at the appropriate depth.
Look for fragile fixes to symptoms and special cases added to shared
infrastructure. Prefer a simpler, general change to the underlying mechanism
when it resolves the same problem. Identify that change.

## Phase 2: Apply the fixes

Wait for all four reviewers to finish, or complete all four checks directly.
Combine findings that concern the same line or mechanism, then fix each
remaining finding directly.

Skip a finding when its fix would change intended behavior, require changes
well outside the reviewed diff, or address a false positive. Note the skip
briefly. Finish with a brief summary of the fixes and skipped findings, or
state that the review did not identify a useful change.
