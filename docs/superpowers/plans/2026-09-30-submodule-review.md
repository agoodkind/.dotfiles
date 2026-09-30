# Submodule update review evidence

| Date | Branch | Class | Reviewer | Verdict | Catches | Escapes | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 2026-09-30 | scheduled-submodule-updates | Repository synchronization and workflow recovery | Independent adversarial review | MERGE-READY for runtime | Recorded object fetching, staged gitlinks, ignored child files, partial rollback, workflow dispatch recovery | Parent index restoration and ignored parent files | All six independent runtime attacks pass after fixes. Live scheduled acceptance remains pending. |
| 2026-09-30 | scheduled-submodule-updates | Scoped CI repairs | Independent regression review | MERGE-READY for runtime | None | None | All six safety fixtures passed again in 23.083 seconds. Freshsmoke tests passed after removing the deleted import prerequisite. |

## Reproduced results

The final repository suite passed in 41.498 seconds. The updater package has no tests. The recorded-version regression failed against the origin/main implementation after an upstream submodule update created a local parent commit.

A fresh explicit test binary reproduced nested revision failure with fetch.recurseSubmodules=false, staged parent index loss during fast-forward, and checkout of a staged gitlink instead of the HEAD gitlink. Further real Git fixtures reproduced ignored child and parent file overwrites and incomplete rollback after an unavailable newly added submodule failed initialization.

All six independent attacks pass against the repaired implementation in 22.607 seconds. The implementation includes regressions through UpdateRepo. The fixtures and initial failure output are retained in the temporary review workspace.

## Static review

The pinned create-pull-request v8 implementation supports signed GITHUB_TOKEN commits and mode 160000 gitlinks. Fixed-branch behavior supports repeated updates, base refreshes, and obsolete PR closure.

Merge-tree succeeded against origin/main at e2595b1. The source removes local branch pulls, automatic gitlink commits, and the separate weekly zinit self-update. The source retains local-work validation.

## Remaining acceptance

Scheduled runs 36749843364 and 36750109678 created the same signed gitlink-only PR #211 at 802219132a0729fc9dcf3974ceb7cbb5100e0bc1. The second run created no duplicate PR or commit. Dispatched validation passed, but GitHub rejected the merge because workflow_dispatch checks do not satisfy PR rulesets. The recovery procedure must use eligible PR events.

Probe run 36752414570 used GITHUB_TOKEN with Actions write permission to approve pending PR workflow run 36749884201. The approval succeeded, and the target run completed with github-actions[bot] as its triggering actor. No external credential is required for this repository's approval endpoint.

Required checks must pass through eligible PR events on the current head, including GitGuardian. A repeated scheduled run must recover pending or failed eligible validation. Local synchronization must succeed after the recorded-version PR merges.

Concurrency and binary-input parsing attacks do not apply to this change. The review uses real repositories and subprocesses rather than mocks. Root validation covers macOS and Linux bootstrap checks and production skill rendering.
