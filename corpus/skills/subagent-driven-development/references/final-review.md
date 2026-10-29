<!-- Source: https://github.com/obra/superpowers/blob/2ce8aa15d05bd57213e6f746f953aafb7ded7a8b/skills/subagent-driven-development/references/final-review.md -->

# Final Review and Finish

Complete every task before the final branch review.

Prepare a review package with the branch's starting commit as the base and the
current commit as the head. Include the commit list, diff statistics, and full
diff. Dispatch the final reviewer on the most capable available model. Give the
reviewer the plan, package, and deferred minor and parked findings. The reviewer
decides which findings require fixes before merge. Save the commit list,
statistics, and full diff in one file in the plan's private workspace.

If the final branch review returns findings, dispatch one fix subagent with the
complete findings list. Do not dispatch one fixer per finding. Each fixer would
need to reconstruct the same context and repeat verification. Run exactly one
scoped re-review of the resulting fix diff. Give the reviewer the fix base,
current head, findings, report, and diff. Save the fix commit list, statistics,
and full diff in one file. Use [re-review-prompt.md](../re-review-prompt.md)
for that review.

Assess remaining findings using the guidance in
[fix-loop.md](fix-loop.md). Record reasons for deferred findings and identify
the smallest change needed for findings that affect dependent work. Do not run
a second fix wave. Report unresolved findings that affect completion.

Before removing the private plan workspace, report material decisions and
unresolved findings to the user. Collect every `Ruling:` entry, including
preflight decisions and deferred findings. Report each decision in order with
its cost if wrong. Delete only this
plan's workspace after the final review is clean and its fixes are integrated.
Follow the repository's branch completion and cleanup rules.
