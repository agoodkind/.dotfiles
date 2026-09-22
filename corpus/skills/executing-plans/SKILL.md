---
name: executing-plans
description: Use when implementing an existing plan directly in the current session.
---

# Executing Plans

Implement the plan in dependency order. Treat the plan as guidance and the verified repository state as evidence.

1. Read the plan, linked specification, repository instructions, and affected code.
2. Compare the plan with the current repository. Resolve stale paths and assumptions before editing.
3. Implement each coherent task completely. Preserve existing work and record material deviations from the plan.
4. Run the verification that proves each task's observable result. Add tests only when the testing rule requires them.
5. Continue through the remaining tasks without routine check-ins. Stop only when a missing user choice changes the result, required authority is absent, or the plan cannot be reconciled with current evidence.
6. Review the final diff against the plan and report unresolved gaps.

Use a progress ledger only when the task is long enough that context loss could cause repeated work. Follow the repository's checkout and commit rules.
