---
name: subagent-driven-development
description: Use when the user or repository instructions request delegated implementation of independent tasks.
---

<!-- Source: https://github.com/obra/superpowers/blob/2ce8aa15d05bd57213e6f746f953aafb7ded7a8b/skills/subagent-driven-development/SKILL.md -->

# Subagent-Driven Development

Delegate bounded tasks that can proceed independently. Keep coupled changes with one owner.

## Setup

Each plan owns a private workspace for its ledger, task briefs, reports, and
review packages. Keep that workspace in the harness's local state directory
outside repositories, workspaces, and user documents. Another plan must not
read or overwrite those artifacts.

Create a separate directory for this plan under the harness's existing local
state directory. Record the plan's absolute path to distinguish it from other
plans with the same filename. Copy each task's complete text into its own
brief file. Save review packages in the same plan directory.

Track progress in a ledger file, not only in task lists.

- Check for this plan's ledger at `<workspace>/progress.md`. If one exists,
  read [resuming.md](references/resuming.md) before anything else.
- Create the ledger with its identity as the first line:
  `# SDD ledger: plan: <plan file path>`.
- After compaction, verify the ledger's recorded commits with `git log` before
  relying on the recorded progress. Do not redispatch completed work.
- Follow the repository's ledger rules for timestamps and updates.

Read the plan once, note its context and Global Constraints, and create a
task entry per task. If the plan specifies a specification, read that too: the
specification is the authority, and conflicts inside the plan resolve against
it. If the specification is unavailable, record that fact in the ledger.
Decisions made without the specification remain provisional.

Before dispatching Task 1, read [preflight.md](references/preflight.md)
and run its conflict scan. Record its findings and decisions when they affect
the next action or recovery after context loss.

## Model Selection

Use the least powerful model that can handle each role to conserve cost and increase speed.

For mechanical implementation tasks such as isolated functions with clear
specifications and one or two files, use a fast, cheap model. Most
implementation tasks are mechanical when the plan is well-specified.

For integration and judgment tasks such as multi-file coordination, pattern
matching, and debugging, use a standard model.

For architecture and design tasks, use the most capable available model.
Dispatch the final whole-branch review on the most capable available model.

Choose review models according to the diff's size, complexity, and risk. A
small mechanical diff does not need the most capable model; a subtle
concurrency change does. Scoped re-reviews of small fix diffs take a cheap to
mid-tier model.

Use a more capable model when the remaining findings require judgment beyond
the current implementer's capability.

Specify the model explicitly when dispatching a subagent. An omitted model
inherits the session's model, which may be the most capable and most expensive.
Follow the harness's model selection rules when they restrict overrides.

Wall-clock and context cost scale with how many turns a subagent takes. The
cheapest models routinely take two to three times the turns on multi-step work
and can cost more overall. Use a mid-tier model as the floor for reviewers and
for implementers working from prose descriptions. When the task's plan text
contains the complete code to write, the implementation is transcription plus
testing. Use the cheapest tier for that implementer. Single-file mechanical
fixes also take the cheapest tier.

Use these task complexity signals for implementation:

- One or two files with a complete specification: cheap model.
- Multiple files with integration concerns: standard model.
- Design judgment or broad codebase understanding: most capable model.

## Dispatch

When the plan lists several small edits of the same kind across files, read
[batching.md](references/batching.md) before dispatching them.

Use files for artifacts instead of pasting full briefs and reports into the
controller's context. A dispatch prompt contains one task, not the session's
history. A fresh subagent needs the task, its required interfaces, and the
global constraints.

Give each implementer:

- one concrete outcome;
- exact file or module ownership;
- the relevant requirements and constraints;
- required interfaces with neighboring tasks;
- the working directory;
- the verification appropriate to the task;
- a short report contract.

Tell every implementer that other agents may edit the repository. Each implementer must preserve others' changes and adapt to the current tree instead of reverting unrelated work.

Dispatch a fresh implementer for each task. Run tasks in sequence and complete
the task review before starting the next task. Do not delegate an open design
decision as an implementation task.

Record the commit before dispatching each implementer. Record the implementer's
agent identity for follow-up fixes. Supply the task brief, the
interfaces and decisions from earlier tasks, the global constraints, and the
report path. Keep exact values in the task brief. Ask the implementer to write
the full report to that path and return a short status, commit list, test
summary, and concerns. Do not paste the session history into the dispatch.
Never make a subagent read the whole plan file.

Give the brief one task. Include the exact values, signatures, and test cases
from the plan. Identify any earlier decisions that the brief cannot establish.
Name the report after the brief, such as `task-1-report.md` for
`task-1-brief.md`. Include any earlier deferred finding that affects this task.
Tell the implementer to work without dispatching other subagents.

Use [implementer-prompt.md](implementer-prompt.md) for the dispatch. The
controller dispatches reviewers after receiving the implementer's report.
The implementer must not dispatch helpers or reviewers.

Answer an implementer's questions before or during the task. If the report
identifies a concern, missing context, or a blocker, resolve that condition
before accepting the task as complete.

For a `DONE` report, generate the review package and dispatch the task reviewer.
For `DONE_WITH_CONCERNS`, `NEEDS_CONTEXT`, or `BLOCKED`, read
[handling-reports.md](references/handling-reports.md) and follow it each time.

## Reconcile

Read each report and inspect the resulting diff. Verify the behavior required by the task. Add or run tests only when the testing rule makes them relevant.

## Review the Task

Per-task reviews are task-scoped gates. The broad review happens once, at the
final whole-branch review. Never skip the task review. Require both a
specification compliance verdict and a task quality verdict. The implementer's
self-review does not replace the task review.

Use [task-reviewer-prompt.md](task-reviewer-prompt.md). Do not dispatch a task
reviewer without a diff file.

Create a review package containing the commit list, diff statistics, and the
complete diff with surrounding context. Use the commit recorded before the
task as the base. Do not use `HEAD~1` because a task may contain several
commits. Dispatch a fresh reviewer with the task brief, implementer's report,
review package, and global constraints. Copy the exact values, formats, and
relationships from the specification into the constraints.
The constraints must preserve exact values, exact formats, and required
relationships between components. The reviewer template already supplies
the review process rules. Do not substitute process rules for the project's
binding requirements.

Do not add open-ended review directives without a concrete, task-specific
reason. Do not ask the reviewer to repeat tests that the implementer already
ran on the same code. Do not instruct the reviewer to ignore or downgrade a
possible finding. Resolve any requirement the reviewer cannot verify from the
diff. Add a confirmed gap to the fix loop.

The reviewer checks the diff against every task requirement. The review must
identify missing, extra, or misunderstood work. The reviewer also checks code
quality and reports each finding with file and line evidence. The reviewer
treats the implementer's report as a claim and verifies it against the diff.
When the diff cannot establish a requirement, the reviewer reports that gap
for the controller to investigate.
For findings the reviewer cannot verify from the diff, read
[cannot-verify.md](references/cannot-verify.md) before marking the task complete.

## Resolve Review Findings

When a review finds a specification gap or a critical or important issue,
follow [fix-loop.md](references/fix-loop.md).

Resolve blocking findings before starting the next task.
Mark the task complete after the review is clean or every open finding has a
recorded decision. Include the task's commit range and review result in the
recovery state when that information is needed to resume.

## Final Review

After every task is complete, follow [final-review.md](references/final-review.md).

Follow repository checkout, commit, and external-write rules.
