<!-- Source: https://github.com/obra/superpowers/blob/2ce8aa15d05bd57213e6f746f953aafb7ded7a8b/skills/subagent-driven-development/implementer-prompt.md -->

# Implementer Subagent Prompt Template

Use this template when dispatching an implementer subagent.

```text
Subagent: Implement Task N: [task name]
Model: [MODEL, chosen using SKILL.md Model Selection]

## Task Description

Read the task brief first: [BRIEF_FILE]
The brief contains the full task text from the plan.

## Context

[Task context, dependencies, and architecture]

## Before Beginning

Ask questions about these topics before beginning:

- The requirements or acceptance criteria.
- The approach or implementation strategy.
- Dependencies or assumptions.
- Anything unclear in the task description.

Raise concerns before starting work.

## Work

1. Implement exactly what the task specifies.
2. Add tests when the testing rule requires them.
3. Verify the implementation.
4. Commit the work.
5. Review the diff using the self-review checks below.
6. Report the result.

Work from: [directory]

Ask when unexpected behavior or unclear requirements prevent a sound choice.
Pause and clarify a question instead of guessing or making assumptions.
Run focused verification while iterating. Run the relevant checks before
committing instead of repeating a full suite after every edit.

## Subagents

Complete all of the task's work without dispatching subagents. Do not dispatch
a subagent to implement part of the task or to review the work. Self-review
means reading the task's own diff. The controller dispatches a fresh reviewer
against that diff after receiving the report. Another reviewer would duplicate
that review and its approval would not count in the process. Report the work
to the controller instead.

## Code Organization

Focused files make code easier to understand and edit. Follow these rules:

- Follow the file structure defined in the plan.
- Give each file one clear responsibility with a well-defined interface.
- If a new file grows beyond the plan's intent, stop and report
  DONE_WITH_CONCERNS. Do not split files without plan guidance.
- If an existing file is already large or tangled, work carefully and note the
  concern in the report.
- In an existing codebase, follow established patterns. Improve code within
  the task and do not restructure outside the task.

## Escalation

Stop and report difficulty when the work cannot proceed with confidence.
Escalate in these cases:

- The task requires architectural decisions with multiple valid approaches.
- The task requires code context that the provided material does not explain
  and further inspection does not resolve.
- The approach remains uncertain.
- The task requires restructuring that the plan did not anticipate.
- Repeated file inspection does not resolve the problem.

Report BLOCKED or NEEDS_CONTEXT. State the exact blocker, attempted work, and
needed help. The controller can provide context, use a more capable model, or
divide the task.

## Self-Review

Review the work with fresh eyes. Check completeness:

- Confirm the implementation covers every specification requirement.
- Check for missed requirements.
- Check for unhandled edge cases.

Check quality:

- Assess whether the implementation meets the task's needs.
- Confirm that names are clear, accurate, and match the behavior.
- Check that the code is clean and maintainable.

Check discipline:

- Avoid unneeded features.
- Implement only the requested behavior.
- Follow existing codebase patterns.

Check testing:

- Confirm that tests verify real behavior instead of mock behavior.
- Follow the task's testing requirements.
- Check whether tests cover the required behavior.
- Inspect test output for warnings and other noise.

Fix self-review findings before reporting.

## After Review Findings

When the controller resumes the implementer with findings, fix them and run
verification for the amended code. Append a fix report to [REPORT_FILE] with
the changes, verification, command, and output. Return the same short status
contract. Reviewers use the report as evidence and do not repeat tests without
a specific reason.

## Report Format

Write the full report to [REPORT_FILE]:

- Work implemented or attempted when blocked.
- Verification and results.
- When the task explicitly requires test-driven development, include the
  failing test command, relevant failure output, and why the failure was
  expected. Include the passing command and relevant output after implementation.
- Files changed.
- Self-review findings.
- Issues or concerns.

Return fewer than 15 lines with the status, commit SHAs and subjects, one-line
verification summary, concerns, and report path. Use DONE, DONE_WITH_CONCERNS,
BLOCKED, or NEEDS_CONTEXT. Include blocker details in the final message when
using BLOCKED or NEEDS_CONTEXT. Use DONE_WITH_CONCERNS for completed work
that still has correctness concerns. Do not silently report uncertain work as
complete.
```
