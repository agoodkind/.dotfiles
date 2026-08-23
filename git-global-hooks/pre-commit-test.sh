#!/usr/bin/env bash

set -euo pipefail

unset CLAUDECODE CODEX_CI GEMINI_CLI

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GLOBAL_HOOKS="$ROOT_DIR/git-global-hooks"
TEMP_DIR="$(mktemp -d)"
TEST_HOME="$TEMP_DIR/home"
EMPTY_HOOKS="$TEMP_DIR/empty-hooks"
MISSING_EMAIL_RULES="$TEMP_DIR/missing-email-rules"

cleanup() {
    chmod -R u+w "$TEMP_DIR" 2>/dev/null || true
    rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

fail() {
    printf 'FAIL: %s\n' "$1" >&2
    exit 1
}

create_repository() {
    local repository_path="$1"
    local branch_name="$2"

    git init --quiet --initial-branch="$branch_name" "$repository_path"
    git -C "$repository_path" config core.hooksPath "$GLOBAL_HOOKS"
    git -C "$repository_path" config init.defaultBranch main
    git -C "$repository_path" config user.name "Test User"
    git -C "$repository_path" config user.email "test@example.invalid"
    git -C "$repository_path" config commit.gpgsign false
}

stage_change() {
    local repository_path="$1"
    local scenario_name="$2"

    printf '%s\n' "$scenario_name" >>"$repository_path/change.txt"
    git -C "$repository_path" add change.txt
}

assert_blocked() {
    local scenario_name="$1"
    local repository_path="$2"
    local output

    stage_change "$repository_path" "$scenario_name"
    if output="$(HOME="$TEST_HOME" GIT_EMAIL_RULES="$MISSING_EMAIL_RULES" git -C "$repository_path" commit --quiet -m "$scenario_name" 2>&1)"; then
        fail "$scenario_name: hook allowed a protected-branch commit"
    fi
    if [[ "$output" != *"make all changes in a worktree and merge via GitHub"* ]]; then
        fail "$scenario_name: hook did not provide the recovery instruction"
    fi
}

assert_allowed() {
    local scenario_name="$1"
    local repository_path="$2"
    local output

    stage_change "$repository_path" "$scenario_name"
    if ! output="$(HOME="$TEST_HOME" GIT_EMAIL_RULES="$MISSING_EMAIL_RULES" git -C "$repository_path" commit --quiet -m "$scenario_name" 2>&1)"; then
        fail "$scenario_name: hook rejected an allowed commit: $output"
    fi
}

# Git commits below set HOME to TEST_HOME. Keep Go caches on the real home so
# hook rebuilds do not write a read-only module cache under TEMP_DIR.
export GOPATH="${GOPATH:-$HOME/go}"
export GOCACHE="${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/go-build}"
export GOMODCACHE="${GOMODCACHE:-$GOPATH/pkg/mod}"
export XDG_CACHE_HOME="${XDG_CACHE_HOME:-$HOME/.cache}"

mkdir -p "$TEST_HOME" "$EMPTY_HOOKS"

main_repository="$TEMP_DIR/main"
create_repository "$main_repository" main
assert_blocked "main branch" "$main_repository"

remote_default_repository="$TEMP_DIR/release"
create_repository "$remote_default_repository" release
git -C "$remote_default_repository" remote add origin "$TEMP_DIR/remote.git"
git -C "$remote_default_repository" symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/release
assert_blocked "remote default branch" "$remote_default_repository"

configured_default_repository="$TEMP_DIR/trunk"
create_repository "$configured_default_repository" trunk
git -C "$configured_default_repository" config init.defaultBranch trunk
assert_blocked "configured default branch" "$configured_default_repository"

feature_repository="$TEMP_DIR/feature"
create_repository "$feature_repository" feature/protected-branches
assert_allowed "feature branch" "$feature_repository"

detached_repository="$TEMP_DIR/detached"
create_repository "$detached_repository" main
git -C "$detached_repository" config core.hooksPath "$EMPTY_HOOKS"
git -C "$detached_repository" commit --allow-empty --quiet -m "Create detached test state"
git -C "$detached_repository" config core.hooksPath "$GLOBAL_HOOKS"
git -C "$detached_repository" checkout --quiet --detach
assert_allowed "detached HEAD" "$detached_repository"

printf 'PASS: protected branch commit hook behavior\n'
