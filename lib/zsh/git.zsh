# shellcheck shell=bash
###############################################################################
# Git worktree helpers
###############################################################################

###############################################################################
# git worktree shortcut: `git wtk|wkt|wk|wt <branch>`
#
# - If `GIT_WTK_REPOS=(/repo/a /repo/b ...)` and/or `GIT_WTK_PARENT_DIR=/parent`
#   is set, we try to guess the intended repo by checking which candidate repos
#   already have `<branch>` (either `refs/heads/<branch>` or `origin/<branch>`).
#   `GIT_WTK_PARENT_DIR` is searched 1 level deep (`$parent/*` only).
# - If exactly one repo matches, we run the worktree command against that repo
#   (equivalent to `git -C <repo> worktree ...`).
# - If multiple repos match, we print candidates and fall back.
# - If no repo matches (or no candidates configured), we fall back to the current
#   directory and require it to be a git repo.
# - On success (existing or created worktree), we `cd` into the worktree path.
###############################################################################
function _git_wtk() {
    local repo_cwd="$PWD"
    local -a git_prefix
    git_prefix=()

    if [[ "$1" == "--git-c" ]]; then
        # Internal helper to emulate `git -C <repo>` while still `cd`'ing into
        # the resulting worktree at the end.
        repo_cwd="$2"
        git_prefix=(-C "$repo_cwd")
        shift 2
    fi

    local branch_name="$1"
    if [[ -z "$branch_name" ]]; then
        echo "Usage: git wtk <branch-name>" >&2
        return 1
    fi

    if ! command git "${git_prefix[@]}" rev-parse \
        --is-inside-work-tree >/dev/null 2>&1; then
        echo "git wtk: not in a git repo" >&2
        echo "try: git -C /path/to/repo wtk <branch>" >&2
        return 1
    fi

    local common_dir
    # `--git-common-dir` is stable across worktrees; use it to find the main
    # worktree root, even when invoked inside a worktree.
    common_dir="$(command git "${git_prefix[@]}" rev-parse \
        --git-common-dir 2>/dev/null)" || return 1

    if [[ "$common_dir" != /* ]]; then
        # `--git-common-dir` can be relative to the current working directory.
        common_dir="$repo_cwd/$common_dir"
    fi
    common_dir="$(cd "$common_dir" 2>/dev/null && pwd -P)" || return 1

    local main_root
    if [[ "$(basename "$common_dir")" == ".git" ]]; then
        # Normal repo: common dir is `<root>/.git`.
        main_root="$(cd "$(dirname "$common_dir")" && pwd -P)" || return 1
    else
        # Worktree: common dir is usually `<root>/.git/worktrees/<name>`.
        main_root="$(command git "${git_prefix[@]}" rev-parse \
            --show-toplevel 2>/dev/null)" || return 1
    fi

    # Worktrees are created under a sibling directory next to the main checkout:
    #   /path/to/repo
    #   /path/to/repo-worktrees/<branch>
    local base_dir="${main_root}-worktrees"
    if ! mkdir -p "$base_dir" 2>/dev/null; then
        echo "git wtk: cannot create worktree dir: $base_dir" >&2
        return 1
    fi

    # Find an existing worktree path for the branch regardless of directory
    # name, using porcelain output (stable for parsing).
    local existing_wt_path
    existing_wt_path="$(command git "${git_prefix[@]}" worktree list --porcelain \
        | awk -v branch="$branch_name" '
        /^worktree/ { path=$2 }
        $1 == "branch" && $2 == "refs/heads/" branch { print path; exit }
        ')"

    if [[ -n "$existing_wt_path" ]]; then
        echo "Worktree for '$branch_name' found at $existing_wt_path."
        builtin cd "$existing_wt_path" || return 1
        return 0
    fi

    local dir_name="${branch_name//\//-}"
    local worktree_path="$base_dir/$dir_name"

    command git "${git_prefix[@]}" fetch origin >/dev/null 2>&1

    local origin_ref="origin/$branch_name"
    local has_origin_branch=false

    if command git "${git_prefix[@]}" show-ref --verify --quiet \
        "refs/remotes/$origin_ref"; then
        has_origin_branch=true
    fi

    if [[ "$has_origin_branch" == "true" ]]; then
        echo "Branch '$branch_name' found on origin. Creating worktree."
        command git "${git_prefix[@]}" worktree add --track -B "$branch_name" \
            "$worktree_path" "$origin_ref" || return $?
    else
        echo "Branch '$branch_name' not found on origin. Creating worktree."
        command git "${git_prefix[@]}" worktree add -B "$branch_name" \
            "$worktree_path" || return $?

        (
            builtin cd "$worktree_path" \
                && command git push -u origin "$branch_name"
        ) || return $?
    fi

    builtin cd "$worktree_path" || return 1
}

function _git_wtk_candidates() {
    emulate -L zsh
    setopt localoptions no_unset null_glob

    local -A seen
    local -a repos unique
    repos=()
    unique=()

    if (( ${+GIT_WTK_REPOS} )); then
        # Explicit list of repo roots; no traversal.
        repos+=("${(@)GIT_WTK_REPOS}")
    fi

    local parent_dir="${GIT_WTK_PARENT_DIR:-}"
    if [[ -n "$parent_dir" && -d "$parent_dir" ]]; then
        # Search 1 level deep (`$parent_dir/*` only). A repo can have `.git` as
        # a directory or a file (e.g. some worktree and submodule setups).
        #
        # Heuristics:
        # - Scan most-recently-modified directories first.
        # - Optionally cap work by count and/or time.
        local scan_limit="${GIT_WTK_PARENT_SCAN_LIMIT:-500}"
        local scan_seconds="${GIT_WTK_PARENT_SCAN_SECONDS:-0}"
        local scanned=0

        zmodload zsh/datetime >/dev/null 2>&1 || true
        local start_seconds="${SECONDS}"
        local start_realtime="${EPOCHREALTIME:-}"

        local child
        # Newest-first ordering by directory mtime.
        for child in "$parent_dir"/*(/Nom); do
            (( scanned++ ))
            if (( scanned > scan_limit )); then
                break
            fi

            if (( scan_seconds > 0 )); then
                if [[ -n "$start_realtime" && -n "${EPOCHREALTIME:-}" ]]; then
                    if (( EPOCHREALTIME - start_realtime > scan_seconds )); then
                        break
                    fi
                else
                    if (( SECONDS - start_seconds > scan_seconds )); then
                        break
                    fi
                fi
            fi

            [[ -e "$child/.git" ]] || continue
            repos+=("$child")
        done
    fi

    local repo
    for repo in "${repos[@]}"; do
        [[ -n "$repo" ]] || continue
        [[ -d "$repo" ]] || continue
        if [[ -z "${seen[$repo]:-}" ]]; then
            unique+=("$repo")
            seen[$repo]=1
        fi
    done

    printf '%s\n' "${unique[@]}"
}

function _git_wtk_guess_repo() {
    emulate -L zsh
    setopt localoptions no_unset

    local branch_name="$1"
    [[ -n "$branch_name" ]] || return 1

    local -a matches
    matches=()

    local repo
    for repo in "${(@f)$(_git_wtk_candidates)}"; do
        command git -C "$repo" rev-parse --is-inside-work-tree \
            >/dev/null 2>&1 || continue

        # A repo "matches" if the branch exists on origin.
        command git -C "$repo" show-ref --verify --quiet \
            "refs/remotes/origin/$branch_name" && matches+=("$repo")
    done

    if (( ${#matches[@]} == 1 )); then
        echo "${matches[1]}"
        return 0
    fi

    if (( ${#matches[@]} > 1 )); then
        echo "git wtk: multiple repos match '$branch_name'; use -C" >&2
        printf '%s\n' "${matches[@]}" >&2
        return 2
    fi

    return 1
}

###############################################################################
# git wkm: create worktrees across multiple repos and open in Cursor workspace
#
# Usage:
#   git wkm <branch> [repo1 repo2 ...]
#   git wkm <branch>                      # uses GIT_WKM_REPOS if set
#
# Config:
#   GIT_WKM_REPOS=(repo1 repo2 repo3)     # default repos (names or paths)
#   GIT_WKM_WORKSPACE_DIR=~/.workspaces   # where to store workspace files
###############################################################################
function _git_wkm() {
    emulate -L zsh
    setopt localoptions no_unset

    local branch_name="$1"
    shift

    if [[ -z "$branch_name" ]]; then
        echo "Usage: git wkm <branch> [repo1 repo2 ...]" >&2
        return 1
    fi

    local -a repos
    repos=("$@")

    # Fall back to GIT_WKM_REPOS if no repos specified.
    if (( ${#repos[@]} == 0 )) && (( ${+GIT_WKM_REPOS} )); then
        repos=("${(@)GIT_WKM_REPOS}")
    fi

    if (( ${#repos[@]} == 0 )); then
        echo "git wkm: no repos specified and GIT_WKM_REPOS not set" >&2
        return 1
    fi

    local parent_dir="${GIT_WTK_PARENT_DIR:-}"
    local workspace_dir="${GIT_WKM_WORKSPACE_DIR:-${HOME}/.workspaces}"
    mkdir -p "$workspace_dir" 2>/dev/null || true

    print "Creating worktrees for branch '$branch_name' in ${#repos[@]} repos..."
    print ""

    local -a worktree_paths
    worktree_paths=()

    local repo repo_path
    for repo in "${repos[@]}"; do
        print -P "%B[$repo]%b"
        repo_path="$repo"

        # Resolve repo name to path under GIT_WTK_PARENT_DIR.
        if [[ ! -d "$repo_path" && -n "$parent_dir" ]]; then
            if [[ -d "$parent_dir/$repo_path" ]]; then
                repo_path="$parent_dir/$repo_path"
            fi
        fi

        if [[ ! -d "$repo_path" ]]; then
            print "  ✗ repo not found: $repo"
            print ""
            continue
        fi

        repo_path="$(cd "$repo_path" 2>/dev/null && pwd -P)"
        if [[ -z "$repo_path" ]]; then
            print "  ✗ could not resolve path"
            print ""
            continue
        fi

        # Fetch to make sure we have latest refs.
        print "  fetching origin..."
        command git -C "$repo_path" fetch origin >/dev/null 2>&1

        # Check if branch exists on origin for this repo.
        if ! command git -C "$repo_path" show-ref --verify --quiet \
            "refs/remotes/origin/$branch_name"; then
            print "  ✗ branch '$branch_name' not found on origin"
            print ""
            continue
        fi

        # Check if worktree already exists (via git worktree list).
        local existing_wt_path
        existing_wt_path="$(command git -C "$repo_path" worktree list --porcelain \
            | awk -v branch="$branch_name" '
                /^worktree/ { path=$2 }
                $1 == "branch" && $2 == "refs/heads/" branch { print path; exit }
            ')"

        if [[ -n "$existing_wt_path" && -d "$existing_wt_path" ]]; then
            print "  ✓ existing worktree: $existing_wt_path"
            worktree_paths+=("$existing_wt_path")
            print ""
            continue
        fi

        # Compute worktree path (matches _git_wtk layout).
        local dir_name="${branch_name//\//-}"
        local wt_path="${repo_path}-worktrees/$dir_name"

        # Create worktree in subshell (so its `cd` doesn't affect us).
        print "  creating worktree..."
        local wt_output
        wt_output="$( _git_wtk --git-c "$repo_path" "$branch_name" 2>&1 )"
        local wt_status=$?

        if [[ $wt_status -ne 0 ]]; then
            print "  ✗ failed: $wt_output"
            print ""
            continue
        fi

        if [[ ! -d "$wt_path" ]]; then
            print "  ✗ worktree not found at expected path: $wt_path"
            print ""
            continue
        fi

        worktree_paths+=("$wt_path")
        print "  ✓ created: $wt_path"
        print ""
    done

    if (( ${#worktree_paths[@]} == 0 )); then
        print "git wkm: no worktrees created or found" >&2
        return 1
    fi

    print "---"
    print "${#worktree_paths[@]} worktree(s) ready"

    # Create Cursor workspace file.
    local ws_name="${branch_name//\//-}"
    local ws_file="$workspace_dir/${ws_name}.code-workspace"

    local folders_json=""
    local wt
    for wt in "${worktree_paths[@]}"; do
        [[ -n "$folders_json" ]] && folders_json+=","
        folders_json+="
    { \"path\": \"$wt\" }"
    done

    cat > "$ws_file" <<EOF
{
  "folders": [$folders_json
  ]
}
EOF

    print ""
    print "Workspace: $ws_file"
    print ""

    # Open in Cursor if available.
    if command -v cursor >/dev/null 2>&1; then
        print "Opening in Cursor..."
        cursor "$ws_file"
    else
        print "Run: cursor '$ws_file'"
    fi
}

###############################################################################
# git() wrapper - intercepts custom subcommands
###############################################################################
function git() {
    if [[ $# -eq 0 ]]; then
        command git
        return $?
    fi

    local subcmd="$1"
    shift

    case "$subcmd" in
        wkm|wtm)
            _git_wkm "$@"
            ;;
        wtk|wkt|wk|wt)
            local branch_name=""
            local repo_override=""

            # Support `-C` both before and after the branch:
            # - `git -C repo wk <branch>`
            # - `git wk <branch> -C repo`
            while (( $# > 0 )); do
                case "$1" in
                    -C)
                        repo_override="${2:-}"
                        shift 2 || break
                        ;;
                    *)
                        if [[ -z "$branch_name" ]]; then
                            branch_name="$1"
                        fi
                        shift
                        ;;
                esac
            done

            if [[ -z "$branch_name" ]]; then
                _git_wtk
                return $?
            fi

            if [[ -n "$repo_override" ]]; then
                local repo_path="$repo_override"

                # Treat repo_override as a name under GIT_WTK_PARENT_DIR if the
                # directory doesn't exist as-is.
                if [[ ! -d "$repo_path" && -n "${GIT_WTK_PARENT_DIR:-}" ]]; then
                    if [[ -d "$GIT_WTK_PARENT_DIR/$repo_path" ]]; then
                        repo_path="$GIT_WTK_PARENT_DIR/$repo_path"
                    fi
                fi

                if [[ -d "$repo_path" ]]; then
                    repo_path="$(cd "$repo_path" 2>/dev/null && pwd -P)" \
                        || return 1
                    _git_wtk --git-c "$repo_path" "$branch_name"
                    return $?
                fi

                echo "git wtk: repo not found: $repo_override" >&2
                return 1
            fi

            local guessed_repo=""
            guessed_repo="$(_git_wtk_guess_repo "$branch_name")"
            if [[ -n "$guessed_repo" ]]; then
                _git_wtk --git-c "$guessed_repo" "$branch_name"
                return $?
            fi

            _git_wtk "$branch_name"
            ;;
        *)
            command git "$subcmd" "$@"
            ;;
    esac
}

