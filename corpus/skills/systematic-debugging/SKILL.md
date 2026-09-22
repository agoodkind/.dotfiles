---
name: systematic-debugging
description: Use when a bug, failure, or unexpected result requires diagnosis before a fix can be chosen.
---

# Systematic Debugging

Find the failing layer and root cause before changing production behavior.

## Investigate

1. Read the complete error, stack trace, logs, and relevant command output.
2. Reproduce the symptom through the real entry point when possible.
3. Verify the diagnostic instrument before trusting its reading.
4. Compare recent changes and a known working case.
5. Add temporary observations at component boundaries when the failing layer is unclear.
6. Trace the bad state backward to the first incorrect write, input, or decision.

See [root-cause-tracing.md](root-cause-tracing.md) for backward call-chain analysis. Use [condition-based-waiting.md](condition-based-waiting.md) for timing failures and [defense-in-depth.md](defense-in-depth.md) after identifying the root cause.

## Test a hypothesis

State one specific cause and the evidence that predicts the symptom. Change one variable or add one observation. A disproved hypothesis returns the investigation to the evidence with the new result included.

## Fix and verify

Change the source of the bad state. Avoid unrelated refactors. Add a regression test only when the testing rule requires one. Verify the original symptom through the same public boundary used to reproduce it, then run any relevant existing checks.

Several failed fix attempts indicate that the model of the system is incomplete. Rebuild the causal chain before attempting another change.
