---
name: simplify
description: Review changed code for reuse, simplicity, and efficiency, then apply focused improvements without changing intended behavior.
---

<!-- Adapted from Claude Code 2.1.318 bundled /simplify. -->

# Simplify

Improve the quality of changed code by reviewing reuse, simplification,
efficiency, and the underlying mechanism, then fixing the findings. Limit this
review to code quality. Do not search for correctness bugs. Use `/code-review`
for a bug review. Preserve intended behavior and follow the repository
instructions and conventions.

## Phase 0: Gather the diff

Run `git diff @{upstream}...HEAD` to get the unified diff under review. If the
branch does not have an upstream, compare HEAD with the repository's actual
base branch, or use `git diff HEAD~1` for a single-commit review.

If the user specifies a pull request, branch, file path, symbol, commit range,
or other area, review that target instead. Otherwise, run `git diff --no-color`
and `git diff --cached --no-color` and include both unstaged and staged changes
in the scope. If the range diff and local diffs are empty, review concrete
files, symbols, or changes mentioned in the conversation. If the conversation
does not identify a scope, review `git show --stat --patch --no-color HEAD`.
Preserve unrelated changes and inspect outside the scope only to understand
existing patterns.

## Phase 1: Review the changes

Use four independent review agents, one per area below. Give each agent the
diff. Run the agents concurrently within the host's limits, scheduling the
remaining reviewers as slots become available. Each agent must use the same
model as the parent and must only report findings. Agents must not edit files,
run formatters, create worktrees, or commit. If the diff is too large, provide
the file list, relevant hunks, and a scope summary. If review agents are
unavailable, perform all four checks directly and state in the final summary
that the review used a single pass without subagents.

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

Run the most relevant lightweight checks for changed files when practical.
Report any checks that could not run or were skipped.
