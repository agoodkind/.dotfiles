<!-- Source: https://github.com/obra/superpowers/blob/2ce8aa15d05bd57213e6f746f953aafb7ded7a8b/skills/subagent-driven-development/re-review-prompt.md -->

# Scoped Re-Review Prompt Template

Use this template when dispatching a re-review after a fix round. The
re-reviewer checks whether each finding was addressed and inspects the fix
diff for new breakage. The full review already happened.

The purpose is to verify each finding from the previous review and check
whether the fix introduced breakage.

```text
Subagent: Re-review Task N fix round R
Model: [MODEL, chosen using SKILL.md Model Selection]

## Task

Read the task brief: [BRIEF_FILE]

## Findings Under Verification

[FINDINGS]

## Fix

Read the implementer's report. Fix reports appear at the end: [REPORT_FILE]

Fix base: [FIX_BASE_SHA], the head from the preceding review
Head: [HEAD_SHA]
Diff file: [DIFF_FILE]

Read the diff file once. It contains the fix commits, a statistics summary,
and the fix diff with surrounding context. Do not repeat git commands. If the
diff file is missing, inspect `git diff --stat [FIX_BASE_SHA]..[HEAD_SHA]` and
`git diff [FIX_BASE_SHA]..[HEAD_SHA]`.

Keep the review read-only. Do not mutate the working tree, index, HEAD, or
branch state.

## Subagents

Complete all review work without dispatching subagents. Do not dispatch a
subagent to review part of the diff or request a second reviewer. This process
already provides each review. Another reviewer duplicates that review and its
verdict does not count. If the diff needs multiple passes, perform them and
report that choice.

## Scope

The review covers the findings list and fix diff. Assess every finding.
Inspect the fix diff for new problems the fix introduced. Do not re-review
unchanged code. Report an issue outside the fix diff under Out-of-Scope
Observations. It does not block this task or extend the loop. The final branch
review assesses the full branch.

## Verification

The implementer ran the verification for the amended code and appended the
results to the report file. Treat the report as unverified claims. Confirm the
fix report identifies the verification and shows its output. Verify the claims
against the diff. Do not repeat the suite solely to confirm the report. Run a
focused test only when the code raises a specific doubt that existing evidence
does not answer. Do not run a package-wide suite for that check.

## Output Format

Begin directly with the first finding verdict. Include only verdicts, findings
with file and line references, and checks performed. Omit a preamble and
process narration.

### Finding Verdicts

For each finding in The Findings Under Verification, in order, report
ADDRESSED or NOT ADDRESSED with file and line evidence. An attempted fix is
not addressed until the specific defect is absent.

### New Breakage in the Fix Diff

Report anything the fix broke or introduced, with severity and file and line
references. State when the fix diff is clean.

### Out-of-Scope Observations

Report issues outside the fix diff for the final review. These observations
do not block the task. State when the review found none.

### Verdict

State whether all findings are addressed without new critical or important
breakage or whether findings remain open. List any findings that remain open.
```

Supply these placeholders:

- `[MODEL]`: reviewer model from SKILL.md Model Selection. Scoped re-reviews of
  small fix diffs take a cheap to mid-tier model.
- `[BRIEF_FILE]`: the same task brief file the implementer used.
- `[FINDINGS]`: the previous review's critical and important findings and
  specification gaps, copied verbatim, one per bullet.
- `[REPORT_FILE]`: the implementer's report file, including appended fix reports.
- `[FIX_BASE_SHA]`: the head from the preceding review.
- `[HEAD_SHA]`: the current commit.
- `[DIFF_FILE]`: the file containing the fix commit list, statistics, and full
  diff from `[FIX_BASE_SHA]` to `[HEAD_SHA]`.

The reviewer returns a verdict for each finding, new breakage in the fix diff,
out-of-scope observations, and a round verdict.
