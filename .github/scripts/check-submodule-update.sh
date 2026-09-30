#!/usr/bin/env bash
set -euo pipefail

: "${UPDATE_PR:?UPDATE_PR is required}"
: "${UPDATE_BRANCH:?UPDATE_BRANCH is required}"
: "${UPDATE_HEAD:?UPDATE_HEAD is required}"
: "${UPDATE_RECOVER_MISSING:?UPDATE_RECOVER_MISSING is required}"

REPOSITORY=$(gh repo view --json nameWithOwner --jq .nameWithOwner)

pull_request_state() {
    gh api "repos/$REPOSITORY/pulls/$UPDATE_PR" --jq "if .head.sha == \"$UPDATE_HEAD\" and .head.ref == \"$UPDATE_BRANCH\" and .head.ref == \"automation/submodule-updates\" and .base.ref == \"main\" and .head.repo.full_name == \"$REPOSITORY\" and .base.repo.full_name == \"$REPOSITORY\" and .user.login == \"github-actions[bot]\" and .merged == false then .state else \"unauthorized\" end"
}

verify_pull_request() {
    local state
    state=$(pull_request_state)
    if [[ "$state" != "open" ]]; then
        printf 'PR %s no longer matches the authorized submodule update.\n' "$UPDATE_PR" >&2
        exit 1
    fi
}

reopen_pull_request() {
    local attempt state reopen_status
    for attempt in 1 2 3; do
        state=$(pull_request_state) || return 1
        if [[ "$state" == "open" ]]; then
            TEMPORARILY_CLOSED=false
            return 0
        fi
        if [[ "$state" != "closed" ]]; then
            printf 'PR %s changed scope before reopen; refusing the update.\n' "$UPDATE_PR" >&2
            return 1
        fi
        if gh api --method PATCH "repos/$REPOSITORY/pulls/$UPDATE_PR" -f state=open --jq .state; then
            TEMPORARILY_CLOSED=false
            return 0
        else
            reopen_status=$?
            printf 'Reopening PR %s failed with exit %s on attempt %s.\n' "$UPDATE_PR" "$reopen_status" "$attempt" >&2
        fi
        state=$(pull_request_state) || return 1
        if [[ "$state" == "open" ]]; then
            TEMPORARILY_CLOSED=false
            return 0
        fi
        if [[ "$attempt" -lt 3 ]]; then
            printf 'PR %s remains %s; retrying reopen in 2 seconds.\n' "$UPDATE_PR" "$state" >&2
            sleep 2
        fi
    done
    printf 'PR %s remains closed after three reopen attempts.\n' "$UPDATE_PR" >&2
    return 1
}

handle_interrupt() {
    local exit_status="$1"
    trap '' INT TERM
    if [[ "$TEMPORARILY_CLOSED" == "true" ]]; then
        if reopen_pull_request; then
            printf 'PR %s is open after interruption.\n' "$UPDATE_PR" >&2
        else
            printf 'PR %s could not be restored after interruption.\n' "$UPDATE_PR" >&2
        fi
    fi
    exit "$exit_status"
}

TEMPORARILY_CLOSED=false
trap 'handle_interrupt 130' INT
trap 'handle_interrupt 143' TERM

find_run() {
    local attempt
    local attempt_limit="$1"
    RUN=''
    for ((attempt = 1; attempt <= attempt_limit; attempt++)); do
        verify_pull_request
        RUN=$(gh api "repos/$REPOSITORY/actions/workflows/$WORKFLOW_ID/runs?event=pull_request&head_sha=$UPDATE_HEAD&per_page=100" --jq "[.workflow_runs[] | select(.workflow_id == $WORKFLOW_ID and .event == \"pull_request\" and .head_sha == \"$UPDATE_HEAD\" and any(.pull_requests[]; .number == $UPDATE_PR))] | sort_by(.id) | last | if . == null then empty else [.id, .status, (.conclusion // \"pending\")] | @tsv end")
        if [[ -n "$RUN" ]]; then
            return
        fi
        if [[ "$attempt" -lt "$attempt_limit" ]]; then
            printf 'PR %s has no eligible %s run yet; retrying in 5 seconds.\n' "$UPDATE_PR" "$WORKFLOW"
            sleep 5
        fi
    done
}

verify_pull_request
VERIFIED=$(gh api "repos/$REPOSITORY/commits/$UPDATE_HEAD" --jq .commit.verification.verified)
if [[ "$VERIFIED" != "true" ]]; then
    printf 'Head %s does not have a verified commit signature.\n' "$UPDATE_HEAD" >&2
    exit 1
fi
REOPENED=false
for WORKFLOW in ci.yml lint.yml fresh-linux-bootstrap.yml fresh-macos-bootstrap.yml; do
    WORKFLOW_ID=$(gh api "repos/$REPOSITORY/actions/workflows/$WORKFLOW" --jq .id)
    find_run 7
    if [[ -z "$RUN" && "$UPDATE_RECOVER_MISSING" == "true" && "$REOPENED" == "false" ]]; then
        verify_pull_request
        printf 'PR %s has no eligible %s run; closing and reopening the same PR.\n' "$UPDATE_PR" "$WORKFLOW"
        TEMPORARILY_CLOSED=true
        if gh api --method PATCH "repos/$REPOSITORY/pulls/$UPDATE_PR" -f state=closed --jq .state; then
            reopen_pull_request
        else
            CLOSE_STATUS=$?
            printf 'Closing PR %s failed with exit %s; restoring its open state.\n' "$UPDATE_PR" "$CLOSE_STATUS" >&2
            reopen_pull_request
            exit "$CLOSE_STATUS"
        fi
        REOPENED=true
        find_run 13
    fi
    if [[ -z "$RUN" ]]; then
        printf 'PR %s has no eligible %s run for head %s.\n' "$UPDATE_PR" "$WORKFLOW" "$UPDATE_HEAD" >&2
        exit 1
    fi
    IFS=$'\t' read -r RUN_ID STATUS CONCLUSION <<<"$RUN"
    if [[ "$STATUS" == "action_required" || "$CONCLUSION" == "action_required" ]]; then
        printf 'Approving %s run %s for PR %s.\n' "$WORKFLOW" "$RUN_ID" "$UPDATE_PR"
        if gh api --method POST "repos/$REPOSITORY/actions/runs/$RUN_ID/approve"; then
            continue
        else
            APPROVAL_STATUS=$?
            printf 'Approval of run %s failed with exit %s; checking concurrent approval.\n' "$RUN_ID" "$APPROVAL_STATUS" >&2
            APPROVAL_PENDING=$(gh api "repos/$REPOSITORY/actions/runs/$RUN_ID" --jq '.status == "action_required" or .conclusion == "action_required"')
            if [[ "$APPROVAL_PENDING" == "true" ]]; then
                exit "$APPROVAL_STATUS"
            fi
            printf 'Run %s no longer requires approval.\n' "$RUN_ID"
        fi
    elif [[ "$STATUS" == "completed" && "$CONCLUSION" == "failure" ]]; then
        printf 'Retrying failed jobs in %s run %s.\n' "$WORKFLOW" "$RUN_ID"
        gh api --method POST "repos/$REPOSITORY/actions/runs/$RUN_ID/rerun-failed-jobs"
    elif [[ "$STATUS" == "completed" && "$CONCLUSION" == "cancelled" ]]; then
        printf 'Retrying canceled %s run %s.\n' "$WORKFLOW" "$RUN_ID"
        gh api --method POST "repos/$REPOSITORY/actions/runs/$RUN_ID/rerun"
    elif [[ "$STATUS" != "completed" || "$CONCLUSION" == "success" ]]; then
        printf '%s run %s has status %s and conclusion %s.\n' "$WORKFLOW" "$RUN_ID" "$STATUS" "$CONCLUSION"
    else
        printf '%s run %s requires attention: %s.\n' "$WORKFLOW" "$RUN_ID" "$CONCLUSION" >&2
        exit 1
    fi
done
