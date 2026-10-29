<!-- Source: https://github.com/obra/superpowers/blob/2ce8aa15d05bd57213e6f746f953aafb7ded7a8b/skills/subagent-driven-development/references/preflight.md -->

# The Pre-flight Scan

Run this scan once before dispatching Task 1.

Scan the plan once for conflicts. Record each check during the scan:

- Check tasks that contradict each other or the plan's Global Constraints.
- Check any plan mandate that the review rubric treats as a defect, such as a
  test that asserts nothing or verbatim duplication of a logic block.

Write the scan output as a table. Include one row for every pair of tasks that
share a file or an interface. For each pair, record what one task produces,
what the other consumes, and the finding. Include one row for every task.
Check whether its tests agree with its code and whether its file operations
agree with each other. A clean verdict without these rows does not establish
that the scan happened.

Write the table to the plan's private recovery state. Assess every finding
against the plan text before execution begins and record the decision. If the
scan is clean, proceed without comment. Use the specification as the binding
authority for each conflict and record each decision beside its row before
dispatching Task 1. The review loop checks conflicts that emerge during
implementation.
