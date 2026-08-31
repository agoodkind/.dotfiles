#!/usr/bin/env bash

protected_branch_policy_applies() {
    local repository_root
    local configured_paths
    local enforced_path

    if ! git rev-parse --verify --quiet '@{upstream}' >/dev/null; then
        return 1
    fi

    repository_root="$(git rev-parse --show-toplevel)"
    repository_root="$(cd "$repository_root" && pwd -P)"
    if ! configured_paths="$(git config --get-all goodkind.protectedBranchEnforcePath)"; then
        configured_paths=""
    fi

    while IFS= read -r enforced_path; do
        if [[ -z "$enforced_path" ]]; then
            continue
        fi
        enforced_path="${enforced_path/#\~/$HOME}"
        enforced_path="${enforced_path%/}"
        if [[ -d "$enforced_path" ]]; then
            enforced_path="$(cd "$enforced_path" && pwd -P)"
        fi
        if [[ "$repository_root" == "$enforced_path" || "$repository_root" == "$enforced_path/"* ]]; then
            return 0
        fi
    done <<<"$configured_paths"

    return 1
}
