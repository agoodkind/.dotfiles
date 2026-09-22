---
name: verification-before-completion
description: Use before claiming that requested work is complete, fixed, passing, rendered, deployed, or otherwise verified.
---

# Verification Before Completion

Match evidence to the claim.

1. Identify the observable result that the completion claim requires.
2. Run the smallest current command or inspection that proves that result.
3. Read the exit status and relevant output.
4. State the verified result. State any unverified part separately.

Tests prove only the behavior they exercise. A build proves compilation, not deployment. A deployment proves rollout, not live acceptance. Generated source changes require inspection of rendered output.

Run tests when the testing rule makes them relevant to the claim. Do not add or run unrelated suites merely because work is ending. Verify delegated work from the resulting diff and relevant behavior instead of relying only on the delegate's report.
