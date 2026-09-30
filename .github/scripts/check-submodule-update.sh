#!/usr/bin/env bash
set -euo pipefail

: "${UPDATE_BRANCH:?UPDATE_BRANCH is required}"
: "${UPDATE_HEAD:?UPDATE_HEAD is required}"

CURRENT_HEAD=$(gh api "repos/{owner}/{repo}/git/ref/heads/$UPDATE_BRANCH" --jq .object.sha)
if [[ "$CURRENT_HEAD" != "$UPDATE_HEAD" ]]; then
    printf '%s\n' 'The update branch changed before validation dispatch.' >&2
    exit 1
fi

for workflow in ci.yml lint.yml fresh-linux-bootstrap.yml fresh-macos-bootstrap.yml; do
    VALIDATION_EXISTS=$(gh run list --workflow "$workflow" --branch "$UPDATE_BRANCH" --commit "$UPDATE_HEAD" --limit 1 --json status,conclusion --jq 'any(.[]; .status != "completed" or .conclusion == "success")')
    if [[ "$VALIDATION_EXISTS" == "true" ]]; then
        continue
    fi
    gh workflow run "$workflow" --ref "$UPDATE_BRANCH"
done
