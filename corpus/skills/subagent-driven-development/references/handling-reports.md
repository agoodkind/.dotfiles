<!-- Source: https://github.com/obra/superpowers/blob/2ce8aa15d05bd57213e6f746f953aafb7ded7a8b/skills/subagent-driven-development/references/handling-reports.md -->

# Handling a Report That Is Not DONE

For `DONE_WITH_CONCERNS`, read the concerns before proceeding. Address concerns
about correctness or scope before review. Record observations such as a file
growing large and proceed to review.

For `NEEDS_CONTEXT`, provide the missing information and dispatch the task
again.

For `BLOCKED`, assess the blocker:

1. If context is missing, provide it and dispatch again with the same model.
2. If the task requires more reasoning, dispatch again with a more capable model.
3. If the task is too large, divide it into smaller pieces.
4. If the plan is wrong, decide the correction, record it, and include the
   decision in the next dispatch.

Do not ignore an escalation or ask the same model to retry without a change.
When an implementer reports a blocker, change the context, model, task, or plan.
