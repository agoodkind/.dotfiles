#!/usr/bin/env bash
set -euo pipefail

: "${UPDATE_PR:?UPDATE_PR is required}"
: "${UPDATE_BRANCH:?UPDATE_BRANCH is required}"
: "${UPDATE_HEAD:?UPDATE_HEAD is required}"

REPOSITORY=$(gh repo view --json nameWithOwner --jq .nameWithOwner)

verify_pull_request() {
    local valid
    valid=$(gh api "repos/$REPOSITORY/pulls/$UPDATE_PR" --jq ".state == \"open\" and .head.sha == \"$UPDATE_HEAD\" and .head.ref == \"$UPDATE_BRANCH\" and .head.ref == \"automation/submodule-updates\" and .base.ref == \"main\" and .head.repo.full_name == \"$REPOSITORY\" and .base.repo.full_name == \"$REPOSITORY\" and .user.login == \"github-actions[bot]\"")
    if [[ "$valid" != "true" ]]; then
        printf 'PR %s no longer matches the authorized submodule update.\n' "$UPDATE_PR" >&2
        exit 1
    fi
}

verify_pull_request
VERIFIED=$(gh api "repos/$REPOSITORY/commits/$UPDATE_HEAD" --jq .commit.verification.verified)
if [[ "$VERIFIED" != "true" ]]; then
    printf 'Head %s does not have a verified commit signature.\n' "$UPDATE_HEAD" >&2
    exit 1
fi
for workflow in ci.yml lint.yml fresh-linux-bootstrap.yml fresh-macos-bootstrap.yml; do
    workflow_id=$(gh api "repos/$REPOSITORY/actions/workflows/$workflow" --jq .id)
    run=''
    for attempt in {1..13}; do
        verify_pull_request
        run=$(gh api "repos/$REPOSITORY/actions/workflows/$workflow_id/runs?event=pull_request&head_sha=$UPDATE_HEAD&per_page=100" --jq "[.workflow_runs[] | select(.workflow_id == $workflow_id and .event == \"pull_request\" and .head_sha == \"$UPDATE_HEAD\" and any(.pull_requests[]; .number == $UPDATE_PR))] | sort_by(.id) | last | if . == null then empty else [.id, .status, (.conclusion // \"pending\")] | @tsv end")
        if [[ -n "$run" ]]; then
            break
        fi
        if [[ "$attempt" -lt 13 ]]; then
            printf 'PR %s has no eligible %s run yet; retrying in 5 seconds.\n' "$UPDATE_PR" "$workflow"
            sleep 5
        fi
    done
    if [[ -z "$run" ]]; then
        printf 'PR %s has no eligible %s run for head %s.\n' "$UPDATE_PR" "$workflow" "$UPDATE_HEAD" >&2
        exit 1
    fi
    IFS=$'\t' read -r run_id status conclusion <<<"$run"
    if [[ "$status" == "action_required" || "$conclusion" == "action_required" ]]; then
        printf 'Approving %s run %s for PR %s.\n' "$workflow" "$run_id" "$UPDATE_PR"
        if gh api --method POST "repos/$REPOSITORY/actions/runs/$run_id/approve"; then
            continue
        else
            approval_status=$?
            printf 'Approval of run %s failed with exit %s; checking concurrent approval.\n' "$run_id" "$approval_status" >&2
            approval_pending=$(gh api "repos/$REPOSITORY/actions/runs/$run_id" --jq '.status == "action_required" or .conclusion == "action_required"')
            if [[ "$approval_pending" == "true" ]]; then
                exit "$approval_status"
            fi
            printf 'Run %s no longer requires approval.\n' "$run_id"
        fi
    elif [[ "$status" == "completed" && "$conclusion" == "failure" ]]; then
        printf 'Retrying failed jobs in %s run %s.\n' "$workflow" "$run_id"
        gh api --method POST "repos/$REPOSITORY/actions/runs/$run_id/rerun-failed-jobs"
    elif [[ "$status" == "completed" && "$conclusion" == "cancelled" ]]; then
        printf 'Retrying canceled %s run %s.\n' "$workflow" "$run_id"
        gh api --method POST "repos/$REPOSITORY/actions/runs/$run_id/rerun"
    elif [[ "$status" != "completed" || "$conclusion" == "success" ]]; then
        printf '%s run %s has status %s and conclusion %s.\n' "$workflow" "$run_id" "$status" "$conclusion"
    else
        printf '%s run %s requires attention: %s.\n' "$workflow" "$run_id" "$conclusion" >&2
        exit 1
    fi
done
