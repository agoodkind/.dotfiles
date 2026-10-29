<!-- Source: https://github.com/obra/superpowers/blob/2ce8aa15d05bd57213e6f746f953aafb7ded7a8b/skills/subagent-driven-development/task-reviewer-prompt.md -->

# Task Reviewer Prompt Template

Use this template to review a task's diff. The reviewer returns separate
verdicts for specification compliance and code quality.

```text
Subagent: Review Task N for specification compliance and quality
Model: [MODEL, chosen using SKILL.md Model Selection]

## Requested Work

Read the task brief: [BRIEF_FILE]

Global constraints from the specification that bind the task:
[GLOBAL_CONSTRAINTS]

## Implementer Report

Read the implementer's report: [REPORT_FILE]

## Diff Under Review

Base: [BASE_SHA]
Head: [HEAD_SHA]
Diff file: [DIFF_FILE]

Read the diff file once. The package includes the commit list, statistics, and
complete diff with surrounding context. The context lines show the changed
files. Read a changed file separately only when a hunk needed for review ends
in the middle of a function. Report that check. Do not repeat git commands. If
the diff file is missing, inspect `git diff --stat [BASE_SHA]..[HEAD_SHA]` and
`git diff [BASE_SHA]..[HEAD_SHA]`.

Do not inspect the broader codebase without a concrete identified risk.
Inspect code outside the diff only to evaluate that risk. Run one focused
check per identified risk and include the risk and check in the report. For
changes to lock ordering, a function or API contract, or shared mutable state,
checking callers is an appropriate method.

Keep the review read-only. Do not mutate the working tree, index, HEAD, or
branch state.

## Subagents

Complete the review without dispatching subagents. Do not dispatch a subagent
to review part of the diff or request a second reviewer. That reviewer would
duplicate an existing review, and its verdict would not count in the process.
If the diff needs multiple passes, perform them and report that choice.

## Implementer Claims

Treat the implementer's report as unverified claims about the code. The report
may be incomplete, inaccurate, or optimistic. Verify its claims against the
diff. Design rationales are claims too. Statements such as "left it per
YAGNI" or "kept it simple deliberately" do not establish correctness. Assess
the code on its merits. A stated rationale does not reduce a finding's severity.

## Verification

The implementer reports verification results for the changed code. Do not
repeat the suite solely to confirm the report. Run a focused test only when
the code raises a specific doubt that existing evidence does not answer. Do
not run a package-wide suite, race detector, or repeated high-count loop for
that check. Recommend heavy validation instead of running it. If commands
cannot run in the environment, identify the test that would answer the doubt.

Report warnings or other noise in the implementer's test output as findings.
The test output should be pristine.

Evidence that the reviewer cannot see may still exist. If the report or its
test evidence appears truncated, or the stated results cannot be located, read
the report at its stated path again. Report missing or garbled evidence to the
controller. Do not regenerate unread evidence by repeating the suite.

## Specification Compliance

Compare the diff with the requested work:

- Missing: requirements skipped, missed, or claimed without implementation.
- Extra: unrequested features, unnecessary engineering, or added features.
- Misunderstood: a feature implemented in the wrong way or a different problem
  solved.

If the brief lists several files with separate changes, check each file against
the diff. Every listed file must have its corresponding change. A listed file
without its required change is a missing finding.

Report a requirement that spans tasks or depends on unchanged code as an item
the controller must verify. Do not expand the review search for that item.

## Code Quality

Check code quality:

- Check separation of concerns.
- Check error handling.
- Check duplication and premature abstraction.
- Check edge cases.

Check tests:

- Confirm that new and changed tests verify real behavior, not mocks.
- Check coverage of the task's edge cases.

Check structure:

- Confirm each file has one clear responsibility with a well-defined interface.
- Confirm that units are decomposed for understanding and independent tests.
- Check the implementation against the plan's file structure.
- Check for new files that are already large or existing files that grew
  substantially. Do not report pre-existing file size as a new issue.

Provide file and line evidence for every finding and for every check that
would otherwise receive a bare yes. Begin the final report with the
specification verdict. Include verdicts, findings with file and line evidence,
and checks performed. Omit a preamble, process narration, and closing summary.

## Calibration

Classify issues by actual severity. Important findings include incorrect or
fragile behavior, missed requirements, and maintainability damage that blocks
merge. Examples include duplicated logic, swallowed errors, and tests that
assert nothing. Broader coverage suggestions and polish are minor.

When the plan mandates behavior that the rubric treats as a defect, report an
important finding and label it plan-mandated. The plan does not establish its
own correctness. Identify specific strengths before listing issues.

## Output Format

### Specification Compliance

Report compliant or issues found, with missing, extra, or misunderstood
behavior and file and line references. List any requirements that cannot be
verified from the diff and the check needed from the controller. Report these
items alongside the verdict for the requirements the diff can verify.

### Strengths

Identify specific strengths.

### Issues

List issues under Critical (Must Fix), Important (Should Fix), and Minor
(Nice to Have). For each, provide the location, problem, effect, and fix when
the fix is not obvious.

### Assessment

Report Approved or Needs fixes and give a one or two sentence assessment.
```

Supply the model, brief, global constraints, report, base, head, and diff path.
Save the commit list, statistics, and full diff for the base-to-head range in
one file. The reviewer returns specification and task quality verdicts,
strengths, and issues by severity.
