<!-- Source: https://github.com/obra/superpowers/blob/2ce8aa15d05bd57213e6f746f953aafb7ded7a8b/skills/subagent-driven-development/references/fix-loop.md -->

# The Fix Loop

The task review reported a specification gap, a critical or important finding,
or an item that the controller confirmed as a real gap.

Before the loop starts, handle these findings separately:

- Record minor findings in the progress ledger and give the final branch
  reviewer that list. The reviewer decides which findings need fixes before
  merge. Minor findings do not enter the loop. Record each finding in the form
  `Task <N>: minor (deferred): <one-liner>`.
- When a finding conflicts with the plan, weigh it against the plan text and
  use the specification as the binding authority. Record the decision before
  acting. Do not dismiss a finding because the plan mandates the behavior.
  Do not dispatch a fix that contradicts the plan without a recorded decision.

For confirmed findings, dispatch a fix and a scoped re-review. Assess disputed
findings against the specification before requesting a fix. Use evidence of
progress to decide whether to resume the implementer, change the approach, or
escalate a blocker.

Resume the original implementer when its context remains useful. Send the open
findings verbatim. The implementer already knows the task, the code, and its
decisions. If the harness cannot resume a subagent, dispatch a fresh implementer
with the brief path, report path, and findings. The report preserves the prior
work for the new implementer.

Dispatch a fresh implementer when the current approach stops producing useful
progress. Use a more capable model when the unresolved problem requires it.
Supply the brief path, report path, open findings, and previous attempts.

In every round, the implementer fixes the findings, runs verification for the
amended work, appends a fix report to the same report file, and returns a short
status. Before dispatching the reviewer, confirm that the fix report identifies
the verification, its command, and its output. Include the relevant test files
in the fix request. A small fix does not require the complete test suite.

Scope the re-review to the findings and the fix diff. Use the commit reviewed
in the preceding round as the fix base. Give the re-reviewer the findings,
brief, report, and diff from that base to the current commit. The re-reviewer
marks each finding addressed or not addressed and flags new breakage in the
fix diff. Add new critical or important breakage to the open findings. Record
observations outside the fix diff as deferred minor findings. Those observations
do not extend the loop. Save the fix commit list, statistics, and full diff
in one file in the plan's private workspace. Dispatch
[re-review-prompt.md](../re-review-prompt.md) with the findings list, brief,
report file, and output path.

After each fix, record the addressed and open findings and
commit range when those details are needed to resume. The controller does not
fix findings directly. The controller resumes the implementer and dispatches
a reviewer for the fix.

Use this format for a recovery entry:

`Task <N>: fixes (<X> addressed, <Y> open; <finding one-liners>; commits <a7>..<b7>)`

Assess each open finding when its evidence is available:

- Defer a finding when the reviewer is wrong or the finding is contestable.
  Record why the code can remain as it is. Include that decision in the final
  review. Use `Task <N>: parked; <finding>; Ruling: <why the code stands>`.
- Defer a confirmed finding when later tasks do not depend on it. Record that
  the finding is real and remains unresolved.
- For a confirmed finding that affects a later task or reveals a plan defect,
  decide the smallest change needed to unblock the dependent work. Record the
  finding and decision and include them in the next task's dispatch. Stop when
  every path forward requires a guess. Use
  `Task <N>: Ruling: <finding>; <what was decided and why>`.

Record every material decision and do not silently discard findings.
