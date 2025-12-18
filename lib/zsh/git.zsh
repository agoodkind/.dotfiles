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
# _git_wkm_worker: create a single worktree (called in parallel by _git_wkm)
#
# Env vars (set by caller):
#   _WKM_REPO, _WKM_PARENT, _WKM_BRANCH, _WKM_OUT, _WKM_PWD
#
# Output format (appended to $_WKM_OUT):
#   status:<message>     - progress updates
#   path:<worktree>      - final worktree path (if success)
#   done:<ok|error>      - sentinel marking completion
###############################################################################
function _git_wkm_worker() {
    local rp="$_WKM_REPO"
    local out="$_WKM_OUT"

    # Resolve repo name to path.
    if [[ ! -d "$rp" && -n "$_WKM_PARENT" && -d "$_WKM_PARENT/$rp" ]]; then
        rp="$_WKM_PARENT/$rp"
    fi

    if [[ ! -d "$rp" ]]; then
        print "status:repo not found" >> "$out"
        print "done:error" >> "$out"
        return 1
    fi

    # Resolve to absolute path.
    [[ "$rp" != /* ]] && rp="$_WKM_PWD/$rp"
    rp="${rp:A}"

    local dir_name="${_WKM_BRANCH//\//-}"
    local wt_path="${rp}-worktrees/$dir_name"

    # Check if worktree already exists.
    if [[ -d "$wt_path" ]]; then
        print "status:exists" >> "$out"
        print "path:$wt_path" >> "$out"
        print "done:ok" >> "$out"
        return 0
    fi

    # Fetch.
    print "status:fetching" >> "$out"
    command git -C "$rp" fetch origin >/dev/null 2>&1

    # Create worktree.
    print "status:creating" >> "$out"
    local origin_ref="origin/$_WKM_BRANCH"
    if command git -C "$rp" show-ref --verify --quiet "refs/remotes/$origin_ref"
    then
        command git -C "$rp" worktree add --track -B "$_WKM_BRANCH" \
            "$wt_path" "$origin_ref" >/dev/null 2>&1
    else
        command git -C "$rp" worktree add -B "$_WKM_BRANCH" \
            "$wt_path" >/dev/null 2>&1
        if [[ -d "$wt_path" ]]; then
            print "status:pushing" >> "$out"
            command git -C "$wt_path" push -u origin "$_WKM_BRANCH" >/dev/null 2>&1
        fi
    fi

    if [[ -d "$wt_path" ]]; then
        print "status:created" >> "$out"
        print "path:$wt_path" >> "$out"
        print "done:ok" >> "$out"
    else
        print "status:failed" >> "$out"
        print "done:error" >> "$out"
    fi
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

    local total=${#repos[@]}

    # Temp dir for job output.
    local tmp_dir="${TMPDIR:-/tmp}/git-wkm-$$"
    command mkdir -p "$tmp_dir"

    # Disable job notifications.
    setopt local_options no_notify no_monitor

    # Initialize output files and spawn workers (detached).
    local idx=0
    for repo in "${repos[@]}"; do
        (( idx++ ))
        : > "$tmp_dir/$idx"  # create empty file
        _WKM_REPO="$repo" \
        _WKM_PARENT="$parent_dir" \
        _WKM_BRANCH="$branch_name" \
        _WKM_OUT="$tmp_dir/$idx" \
        _WKM_PWD="$PWD" \
        _git_wkm_worker &!
    done

    # Track state for each repo.
    local -a last_status last_line_count wt_paths done_flags
    for i in {1..$total}; do
        last_status[$i]="pending"
        last_line_count[$i]=0
        wt_paths[$i]=""
        done_flags[$i]=0
    done

    # Print initial status.
    print "Creating worktrees for '$branch_name'...\n"
    for i in {1..$total}; do
        print "  ⏳ ${repos[$i]}"
    done

    # Poll and update display.
    local all_done=0
    while (( ! all_done )); do
        all_done=1
        for i in {1..$total}; do
            (( done_flags[$i] )) && continue
            all_done=0

            local out_file="$tmp_dir/$i"
            [[ -f "$out_file" ]] || continue

            # Read new lines.
            local line_num=0
            while IFS= read -r line; do
                (( line_num++ ))
                (( line_num <= last_line_count[$i] )) && continue

                local key="${line%%:*}"
                local val="${line#*:}"
                case "$key" in
                    status) last_status[$i]="$val" ;;
                    path)   wt_paths[$i]="$val" ;;
                    done)   done_flags[$i]=1 ;;
                esac
            done < "$out_file"
            last_line_count[$i]=$line_num
        done
        sleep 0.1
    done

    # Move cursor up and redraw final status.
    printf "\033[%dA" "$total"
    local -a worktree_paths worktree_names
    worktree_paths=()
    worktree_names=()
    for i in {1..$total}; do
        local st="${last_status[$i]}"
        local repo="${repos[$i]}"
        printf "\r\033[K"  # clear line
        if [[ -n "${wt_paths[$i]}" ]]; then
            print "  ✓ $repo ($st)"
            worktree_paths+=("${wt_paths[$i]}")
            worktree_names+=("${repo##*/}")  # basename if it's a path
        else
            print "  ✗ $repo ($st)"
        fi
    done

    command rm -rf "$tmp_dir"
    print ""

    if (( ${#worktree_paths[@]} == 0 )); then
        print "git wkm: no worktrees created or found" >&2
        return 1
    fi

    print "---"
    print "${#worktree_paths[@]} worktree(s) ready"

    # Create Cursor workspace file with repo names as display names.
    local ws_name="${branch_name//\//-}"
    local ws_file="$workspace_dir/${ws_name}.code-workspace"

    # Build workspace JSON with jq.
    local folders_json="[]"
    local idx
    for idx in {1..${#worktree_paths[@]}}; do
        folders_json=$(print -r -- "$folders_json" | jq \
            --arg name "${worktree_names[$idx]}" \
            --arg path "${worktree_paths[$idx]}" \
            '. + [{"name": $name, "path": $path}]')
    done

    jq -n --argjson folders "$folders_json" \
        '{"folders": $folders, "settings": {
            "git.enabled": true,
            "git.autoRepositoryDetection": "subFolders",
            "git.showCursorWorktrees": true,
            "scm.alwaysShowRepositories": true
        }}' > "$ws_file"

    print ""
    print "Workspace: $ws_file"

    # Clear Cursor workspace state to prevent repos from being hidden (Bug workaround).
    # Cursor stores SCM visibility state in SQLite; stale state can hide repos.
    _git_wkm_clear_cursor_state "$ws_file"

    # Open in Cursor.
    if command -v cursor >/dev/null 2>&1; then
        print "Opening in Cursor..."
        cursor "$ws_file"
    elif [[ "$OSTYPE" == darwin* ]]; then
        print "Opening in Cursor..."
        open -a "Cursor" "$ws_file"
    else
        print ""
        print "Run: cursor '$ws_file'"
    fi
}

###############################################################################
# _git_wkm_clear_cursor_state: clear SCM visibility state to prevent hidden repos
#
# Cursor has bugs where repos get stuck hidden:
#   - closedRepositories: repos manually closed, can't reopen
#   - scm:view:visibleRepositories: race condition hides repos on first open
#
# This function finds and clears that state before opening the workspace.
###############################################################################
function _git_wkm_clear_cursor_state() {
    local ws_file="$1"
    [[ -z "$ws_file" ]] && return

    # Only works on macOS for now
    [[ "$OSTYPE" != darwin* ]] && return

    local storage_base="$HOME/Library/Application Support/Cursor/User/workspaceStorage"
    [[ ! -d "$storage_base" ]] && return

    # Find workspace storage by searching for matching workspace.json
    local ws_hash=""
    local ws_json
    for ws_json in "$storage_base"/*/workspace.json(N); do
        if grep -q "$ws_file" "$ws_json" 2>/dev/null; then
            ws_hash="${ws_json:h}"
            break
        fi
    done

    [[ -z "$ws_hash" || ! -d "$ws_hash" ]] && return

    local db_file="$ws_hash/state.vscdb"
    [[ ! -f "$db_file" ]] && return

    # Clear problematic keys
    if command -v sqlite3 >/dev/null 2>&1; then
        sqlite3 "$db_file" "DELETE FROM ItemTable WHERE key = 'scm:view:visibleRepositories';" 2>/dev/null
        sqlite3 "$db_file" "UPDATE ItemTable SET value = '{}' WHERE key = 'vscode.git';" 2>/dev/null
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

