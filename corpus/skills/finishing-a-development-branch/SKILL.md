---
name: finishing-a-development-branch
description: Use when implementation is complete and the remaining work is integration, publication, or preservation of a feature branch.
---

# Finishing a Development Branch

Verify the exact state required by the requested integration action. Follow repository rules for commits, pull requests, pushes, merges, and cleanup.

If the current checkout is the main branch, leave it on the main branch. Report the completed changes and perform only the integration action the user requested.

If the work is on a feature branch or detached checkout, preserve that checkout unless the user requests a push, pull request, merge, or cleanup. Never switch the main checkout to another branch.

Before a push or pull request, verify the relevant branch diff and required checks. Before a merge, read the active merge contract. Use the repository cleanup skill only when cleanup is requested or required by the selected integration workflow.

Do not discard commits, uncommitted files, branches, or checkouts without explicit authorization for those exact targets.
