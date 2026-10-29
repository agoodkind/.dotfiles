<!-- Source: https://github.com/obra/superpowers/blob/2ce8aa15d05bd57213e6f746f953aafb7ded7a8b/skills/subagent-driven-development/references/final-review.md -->

# Final Review and Finish

Complete every task before the final branch review.

Prepare a review package with the branch's starting commit as the base and the
current commit as the head. Include the commit list, diff statistics, and full
diff. Dispatch the final reviewer on the most capable available model. Give the
reviewer the plan, package, and deferred findings and decisions. The reviewer
decides which findings require fixes before merge. Save the commit list,
statistics, and full diff in one file in the plan's private workspace.

Assess the final review's findings against the specification and current code.
Correct confirmed issues, verify the corrections, and report any unresolved
findings that affect completion. Record reasons for deferring or rejecting a
finding.

Keep the private plan workspace when unresolved findings prevent completion.

Before removing the private plan workspace, report material decisions and
unresolved findings to the user. Collect every `Ruling:` entry, including
preflight decisions and deferred findings. Report each decision in order with
its cost if wrong. Delete only this
plan's workspace after the final review is clean and its fixes are integrated.
Follow the repository's branch completion and cleanup rules.
