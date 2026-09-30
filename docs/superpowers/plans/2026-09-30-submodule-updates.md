# Scheduled submodule updates implementation plan

## Goal

Update external submodule revisions through signed GitHub pull requests. Local synchronization must use the revisions recorded in dotfiles without creating parent commits or replacing local submodule work.

## Current behavior

The repository updater advances submodule branches and commits their pointers on local main. A later dotfiles update creates divergent history. Weekly maintenance also runs a separate zinit self-update. Linked-worktree cleanup can unregister submodules in shared repository configuration when it runs deinit.

## Tasks

### 1. Synchronize recorded revisions locally

Modify [repository.go](../../../dots/internal/sync/repository/repository.go) and [repository_test.go](../../../dots/internal/sync/repository/repository_test.go).

1. Remove upstream branch pulls and automatic staging and commits from local submodule synchronization.
2. Initialize missing submodules and check out recorded revisions only after checking existing work for dirty files and local commits.
3. Preserve local work and report the exact submodule when synchronization would replace it.
4. Fast-forward the parent from the fetched origin/main ref.
5. Exercise UpdateRepo with real temporary remotes. Advance the submodule remote, then the parent remote. Verify that local main fast-forwards without automatic commits. Verify repeated runs, dirty files, local commits, and fetch failures.

### 2. Remove the duplicate weekly update

Modify [updater.go](../../../dots/internal/dispatch/updater/updater.go).

Remove zinit self-update from weekly maintenance. Retain updates for plugins managed by zinit.

### 3. Schedule signed pointer-update pull requests

Create a daily GitHub Action with manual dispatch. Update all declared submodules from their configured upstream branches in the disposable runner checkout. Create or update one signed pull request containing only gitlink changes.

Use the repository GITHUB_TOKEN. Run CI, lint, and bootstrap validation on pull_request events. Approve pending runs only for the current signed update PR, its exact head, and the expected workflows. Use a pull_request_target workflow that executes trusted base code to approve new runs. Recover missing runs by closing and reopening the same bot PR once from the scheduled updater. Retry failed or canceled validation on unchanged open update PRs. Skip successful and running validation for the same commit. Keep all active GitHub ruleset requirements. Verify commit signatures and check names on the first live update PR. Do not bypass failed checks.

Add pull_request to CI. Keep manual workflow dispatch for diagnostics; GitHub excludes those runs from required PR checks. Declare the zsh-defer upstream branch explicitly in [.gitmodules](../../../.gitmodules).

Remove the Claude-Opus-5-tools submodule and its source import from [the corpus manifest](../../../corpus/targets.toml). Verify that no production references require its imported skills or agents. Keep generic import support and its fixture-based tests.

### 4. Handle submodule worktrees during cleanup

Modify [cleanup-git](../../../corpus/skills/cleanup-git/SKILL.md.tmpl).

Inspect every submodule's files, HEAD, and local refs before removal. Prove containment or preserve unique work. Do not run deinit against shared configuration during linked-worktree cleanup. After all content is classified, allow force removal only for Git's submodule structural restriction. Verify the surviving checkouts' registrations and submodule files after removal.

Reproduce the procedure using real temporary repositories and two populated worktrees. Verify that removal does not alter the surviving checkout's submodule configuration, HEAD, or files.

### 5. Verify and deliver

Run focused Go tests, the repository checks, Go formatting and vet, workflow validation, and production skill rendering. Review the complete patch. Create signed commits and a pull request. Complete its review and active ruleset checks. After authorized merge, fast-forward the main checkout, initialize its recorded submodules, dispatch the scheduled updater, and verify its signed update PR and CI runs. Clean up the implementation branch and worktree after proving containment.

## Acceptance

Local sync creates no parent commits and does not advance submodules beyond recorded revisions. Scheduled updates create signed gitlink-only PRs with required checks. Repeated local sync succeeds after those PRs merge. Worktree cleanup removes contained submodule work without altering surviving checkouts.
