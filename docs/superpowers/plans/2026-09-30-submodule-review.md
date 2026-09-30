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

The pinned create-pull-request v8 implementation supports signed GITHUB_TOKEN commits and mode 160000 gitlinks. Fixed-branch behavior supports repeated updates, base refreshes, and obsolete PR closure. Explicit validation dispatch retries unchanged open PRs. Run lookup filters workflow_dispatch events, preventing approval-blocked PR runs from suppressing dispatch.

Merge-tree succeeded against origin/main at e2595b1. The source removes local branch pulls, automatic gitlink commits, and the separate weekly zinit self-update. The source retains local-work validation.

## Remaining acceptance

The first live scheduled update must produce a verified signed gitlink-only PR. Required checks must run on its current head, including the external GitGuardian check. A repeated run must create no duplicate PR or commit and recover missing validation. Local synchronization must succeed after a recorded-version PR merges.

Concurrency and binary-input parsing attacks do not apply to this change. The review uses real repositories and subprocesses rather than mocks. Root validation covers macOS and Linux bootstrap checks and production skill rendering.
