---
name: subagent-driven-development
description: Use when the user or repository instructions request delegated implementation of independent tasks.
---

# Subagent-Driven Development

Delegate bounded tasks that can proceed independently. Keep coupled changes with one owner.

## Dispatch

Give each implementer:

- one concrete outcome;
- exact file or module ownership;
- the relevant requirements and constraints;
- required interfaces with neighboring tasks;
- the working directory;
- the verification appropriate to the task;
- a short report contract.

Tell every implementer that other agents may edit the repository. Each implementer must preserve others' changes and adapt to the current tree instead of reverting unrelated work.

Run independent tasks in parallel. Run tasks that modify the same files or depend on unfinished interfaces in sequence. Do not delegate an open design decision as an implementation task.

## Reconcile

Read each report and inspect the resulting diff. Verify the behavior required by the task. Add or run tests only when the testing rule makes them relevant.

Resolve overlapping edits in dependency order. Send a focused follow-up to the original implementer when its context remains useful. Use a separate review pass only when the user requested review or the change's risk justifies independent judgment.

Follow repository checkout, commit, and external-write rules. Use a progress ledger when context loss could cause repeated work.
