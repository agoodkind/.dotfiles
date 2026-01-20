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
    common_dir="$(builtin cd "$common_dir" 2>/dev/null && pwd -P)" || return 1

    local main_root
    if [[ "$(basename "$common_dir")" == ".git" ]]; then
        # Normal repo: common dir is `<root>/.git`.
        main_root="$(builtin cd "$(dirname "$common_dir")" && pwd -P)" || return 1
    else
        # Worktree: common dir is usually `<root>/.git/worktrees/<name>`.
        main_root="$(command git "${git_prefix[@]}" rev-parse \
            --show-toplevel 2>/dev/null)" || return 1
    fi

    # Worktrees are created under a sibling directory next to the main checkout:
    #   /path/to/repo
    #   /path/to/repo-worktrees/<branch>
    local base_dir="${main_root}-worktrees"
    if ! command mkdir -p "$base_dir" 2>/dev/null; then
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
#   status:<message>     - progress updates (keep short to avoid line wrap)
#   path:<worktree>      - final worktree path (if success)
#   done:<ok|error>      - sentinel marking completion
###############################################################################

# Wrapper for async_run: takes positional args, sets env vars, calls worker
# Usage: _git_wkm_worker_async <repo> <parent> <branch> <out> <pwd>
function _git_wkm_worker_async() {
    _WKM_REPO="$1" \
    _WKM_PARENT="$2" \
    _WKM_BRANCH="$3" \
    _WKM_OUT="$4" \
    _WKM_PWD="$5" \
    _git_wkm_worker
}

function _git_wkm_worker() {
    local rp="$_WKM_REPO"
    local out="$_WKM_OUT"

    # Resolve repo name to path.
    if [[ ! -d "$rp" && -n "$_WKM_PARENT" && -d "$_WKM_PARENT/$rp" ]]; then
        rp="$_WKM_PARENT/$rp"
    fi

    if [[ ! -d "$rp" ]]; then
        print "status:repo not found" >> "$out"
        print "error:repository directory not found: $_WKM_REPO" >> "$out"
        print "done:error" >> "$out"
        return 1
    fi

    # Resolve to absolute path.
    [[ "$rp" != /* ]] && rp="$_WKM_PWD/$rp"
    rp="${rp:A}"

    local dir_name="${_WKM_BRANCH//\//-}"
    local wt_path="${rp}-worktrees/$dir_name"

    # Check if worktree already exists at expected path.
    if [[ -d "$wt_path" ]]; then
        print "status:exists" >> "$out"
        print "path:$wt_path" >> "$out"
        print "done:ok" >> "$out"
        return 0
    fi

    # Check if branch is already checked out in another worktree.
    local existing_wt
    existing_wt=$(command git -C "$rp" worktree list --porcelain 2>/dev/null \
        | command awk -v branch="$_WKM_BRANCH" '
            /^worktree / { wt = substr($0, 10) }
            /^branch refs\/heads\// { 
                b = substr($0, 19)
                if (b == branch) { print wt; exit }
            }')
    if [[ -n "$existing_wt" && "$existing_wt" != "$wt_path" ]]; then
        local short_path="${existing_wt##*/}"
        # Check if the existing worktree is stale (merged + clean) - if so, remove it.
        if _git_wkm_worktree_is_clean "$existing_wt" && \
           _git_wkm_branch_is_stale "$rp" "$_WKM_BRANCH"; then
            print "status:removing stale: $short_path" >> "$out"
            local remove_err=""
            remove_err=$(command git -C "$rp" worktree remove "$existing_wt" 2>&1)
            if [[ $? -ne 0 ]]; then
                print "status:cleanup failed" >> "$out"
                print "error:failed to remove stale worktree: $remove_err" >> "$out"
                print "done:error" >> "$out"
                return 1
            fi
            # Delete the local branch since worktree remove doesn't.
            command git -C "$rp" branch -d "$_WKM_BRANCH" >/dev/null 2>&1 || true
            # Continue to create fresh worktree below.
        else
            # Not stale - reuse the existing worktree at its current path.
            print "status:exists (reused)" >> "$out"
            print "path:$existing_wt" >> "$out"
            print "done:ok" >> "$out"
            return 0
        fi
    fi

    # Fetch with timeout.
    print "status:fetching" >> "$out"
    timeout 30 command git -C "$rp" fetch origin >/dev/null 2>&1 || true

    # Create worktree.
    print "status:creating" >> "$out"
    local origin_ref="origin/$_WKM_BRANCH"
    local git_err=""
    if command git -C "$rp" show-ref --verify --quiet "refs/remotes/$origin_ref"
    then
        # Branch exists on origin - track it.
        git_err=$(command git -C "$rp" worktree add --track -B "$_WKM_BRANCH" \
            "$wt_path" "$origin_ref" 2>&1)
    else
        # New branch - start from origin/main to avoid stale base.
        git_err=$(command git -C "$rp" worktree add -B "$_WKM_BRANCH" \
            "$wt_path" origin/main 2>&1)
    fi

    if [[ -d "$wt_path" ]]; then
        print "status:created" >> "$out"
        print "path:$wt_path" >> "$out"
        print "done:ok" >> "$out"
    else
        # Store full error for later display, truncate for status line.
        local full_err="${git_err%%$'\n'*}"
        full_err="${full_err#fatal: }"
        local err_msg="$full_err"
        (( ${#err_msg} > 35 )) && err_msg="${err_msg:0:32}..."
        [[ -z "$err_msg" ]] && err_msg="failed"
        print "status:$err_msg" >> "$out"
        print "error:$full_err" >> "$out"
        print "done:error" >> "$out"
    fi
}

###############################################################################
# git wkm cleanup helpers
###############################################################################
function _git_wkm_resolve_repo_path() {
    local repo="$1"
    local parent_dir="$2"
    local pwd="$3"

    local rp="$repo"
    if [[ ! -d "$rp" && -n "$parent_dir" && -d "$parent_dir/$rp" ]]; then
        rp="$parent_dir/$rp"
    fi

    [[ "$rp" != /* ]] && rp="$pwd/$rp"
    rp="${rp:A}"

    if [[ -d "$rp" ]]; then
        print -r -- "$rp"
        return 0
    fi

    return 1
}

function _git_wkm_worktree_branch_map() {
    emulate -L zsh
    setopt localoptions no_unset

    local repo_path="$1"
    local wt=""
    local br=""
    local line=""

    while IFS= read -r line; do
        case "$line" in
            worktree\ *)
                wt="${line#worktree }"
                br=""
                ;;
            branch\ refs/heads/*)
                br="${line#branch refs/heads/}"
                if [[ -n "$wt" && -n "$br" ]]; then
                    print -r -- "$wt"$'\t'"$br"
                fi
                ;;
        esac
    done < <(command git -C "$repo_path" worktree list --porcelain 2>/dev/null)
}

function _git_wkm_worktree_is_clean() {
    local wt_path="$1"
    [[ -d "$wt_path" ]] || return 1
    [[ -z "$(command git -C "$wt_path" status --porcelain 2>/dev/null)" ]]
}

function _git_wkm_repo_upstream_main_ref() {
    local repo_path="$1"

    if command git -C "$repo_path" show-ref --verify --quiet \
        "refs/remotes/origin/main" 2>/dev/null; then
        print -r -- "origin/main"
        return 0
    fi

    if command git -C "$repo_path" show-ref --verify --quiet \
        "refs/remotes/origin/master" 2>/dev/null; then
        print -r -- "origin/master"
        return 0
    fi

    if command git -C "$repo_path" show-ref --verify --quiet \
        "refs/remotes/origin/trunk" 2>/dev/null; then
        print -r -- "origin/trunk"
        return 0
    fi

    return 1
}

function _git_wkm_branch_is_stale() {
    local repo_path="$1"
    local branch="$2"

    case "$branch" in
        main|master|trunk) return 1 ;;
    esac

    command git -C "$repo_path" show-ref --verify --quiet \
        "refs/heads/$branch" 2>/dev/null || return 1

    local upstream_ref=""
    upstream_ref="$(_git_wkm_repo_upstream_main_ref "$repo_path")" || return 1

    # A branch is stale if it has no unique patches compared to origin/<main>.
    # This catches squash-merges since `git cherry` compares patch-ids.
    local line=""
    while IFS= read -r line; do
        case "$line" in
            +\ *) return 1 ;;
        esac
    done < <(command git -C "$repo_path" cherry "$upstream_ref" "$branch" \
        2>/dev/null)

    return 0
}

function _git_wkm_cleanup_repo_worker() {
    emulate -L zsh
    setopt localoptions no_xtrace
    unsetopt errexit err_return pipe_fail

    # If xtrace is enabled globally (e.g., via zinit debug), it will interleave
    # with the status redraw and spam the terminal. Force it off + redirect.
    local saved_xtracefd="${XTRACEFD:-2}"
    local xtrace_devnull_fd=""
    exec {xtrace_devnull_fd}>/dev/null
    XTRACEFD=$xtrace_devnull_fd
    unsetopt xtrace verbose
 
    local cleanup_done=false
    {
        local repo="$1"
        local parent_dir="$2"
        local branch_filter="$3"
        local should_fetch="$4"
        local out="$5"

        function _cleanup_log() {
            [[ -n "$out" ]] || return 0
            # Never spam the terminal if the temp dir is gone; just best-effort.
            command mkdir -p "${out:h}" 2>/dev/null || return 0
            print -r -- "$1" >> "$out" 2>/dev/null || true
        }

        [[ -n "$out" ]] || return 1
        command mkdir -p "${out:h}" 2>/dev/null || out="/dev/null"
        : >| "$out" 2>/dev/null || out="/dev/null"

        function _cleanup_done_ok() {
            cleanup_done=true
            _cleanup_log "done:ok"
        }

        function _cleanup_done_error() {
            cleanup_done=true
            _cleanup_log "done:error"
        }

        _cleanup_log "status:resolving"

        local rp=""
        rp="$(_git_wkm_resolve_repo_path "$repo" "$parent_dir" "$PWD")" || {
            _cleanup_log "status:not found"
            _cleanup_log "error:repository not found: $repo"
            _cleanup_done_error
            return 1
        }

        if [[ "$should_fetch" == "true" ]]; then
            _cleanup_log "status:fetching"
            command git -C "$rp" fetch origin --quiet 2>/dev/null || true
        fi

        local wt_root="${rp}-worktrees"
        if [[ ! -d "$wt_root" ]]; then
            _cleanup_log "status:no worktrees"
            _cleanup_log "meta:repo:$repo"
            _cleanup_log "meta:checked:0"
            _cleanup_log "meta:stale:0"
            _cleanup_done_ok
            return 0
        fi

        _cleanup_log "status:listing worktrees"

    local dir_name=""
    local target_wt=""
    if [[ -n "$branch_filter" ]]; then
        dir_name="${branch_filter//\//-}"
        target_wt="$wt_root/$dir_name"
    fi

    typeset -A branch_count
    local -a map_lines
    map_lines=()

    local line=""
    while IFS= read -r line; do
        map_lines+=("$line")
        local br="${line#*$'\t'}"
        local cur="${branch_count[$br]:-0}"
        branch_count[$br]=$(( cur + 1 ))
    done < <(_git_wkm_worktree_branch_map "$rp")

    local wt_count=${#map_lines[@]}
    if (( wt_count == 0 )); then
        _cleanup_log "status:no worktrees"
        _cleanup_log "meta:repo:$repo"
        _cleanup_log "meta:checked:0"
        _cleanup_log "meta:stale:0"
            _cleanup_done_ok
        return 0
    fi

    _cleanup_log "status:scanning $wt_count worktrees"

    local upstream_ref=""
    upstream_ref="$(_git_wkm_repo_upstream_main_ref "$rp")" || upstream_ref=""

    local checked=0
    local stale=0
    local m=""
    for m in "${map_lines[@]}"; do
        local wt="${m%%$'\t'*}"
        local branch="${m#*$'\t'}"

        [[ "$wt" == "$wt_root/"* ]] || continue
        [[ -n "$target_wt" && "$wt" != "$target_wt" ]] && continue
        (( checked++ ))

        # Update progress periodically.
        if (( checked % 5 == 0 )); then
            _cleanup_log "status:checked $checked/$wt_count"
        fi

        case "$branch" in
            main|master|trunk) continue ;;
        esac

        if (( ${branch_count[$branch]:-0} > 1 )); then
            continue
        fi

        if ! _git_wkm_worktree_is_clean "$wt"; then
            continue
        fi

        [[ -n "$upstream_ref" ]] || continue

        if ! _git_wkm_branch_is_stale "$rp" "$branch"; then
            continue
        fi

        (( stale++ ))
        _cleanup_log "stale:$rp"$'\t'"$wt"$'\t'"$branch"
    done

    _cleanup_log "meta:repo:$repo"
    _cleanup_log "meta:checked:$checked"
    _cleanup_log "meta:stale:$stale"
        _cleanup_done_ok
    return 0
    } always {
        if ! $cleanup_done; then
            _cleanup_log "status:failed"
            _cleanup_log "error:worker exited unexpectedly"
            _cleanup_log "done:error"
        fi
        XTRACEFD=$saved_xtracefd
        exec {xtrace_devnull_fd}>&- 2>/dev/null
    }
}

function _git_wkm_cleanup() {
    emulate -L zsh
    setopt localoptions no_notify no_monitor no_xtrace
    unsetopt errexit err_return pipe_fail

    # Ensure xtrace can't leak into the ANSI redraw loop (keeps output dynamic).
    local saved_xtracefd="${XTRACEFD:-2}"
    local xtrace_devnull_fd=""
    exec {xtrace_devnull_fd}>/dev/null
    XTRACEFD=$xtrace_devnull_fd
    unsetopt xtrace verbose bgnice

    {
        local branch_filter="$1"
        shift

        local apply_mode="$1"
        shift

        local parent_dir="$1"
        shift

        local jobs="$1"
        shift

        local should_fetch="$1"
        shift

        local -a repos
        repos=("$@")

    if (( ${#repos[@]} == 0 )); then
        echo "git wkm --cleanup: no repos specified and GIT_WKM_REPOS not set" >&2
        return 1
    fi

    local -a stale_rows
    stale_rows=()

    local tmp_dir="${TMPDIR:-/tmp}/git-wkm-cleanup-$$"
    command mkdir -p "$tmp_dir" 2>/dev/null || return 1

    local max_jobs="$jobs"
    [[ -z "$max_jobs" ]] && max_jobs=4
    (( max_jobs < 1 )) && max_jobs=1
    local total=${#repos[@]}
    local total_checked=0
    local total_stale=0

    # Print header (matches create mode style).
    if [[ -n "$branch_filter" ]]; then
        print "Scanning $total repos for stale worktrees (branch: $branch_filter)..."
    else
        print "Scanning $total repos for stale worktrees..."
    fi
    print "  ⚡ Spawning up to $max_jobs parallel workers...\n"

    # Track state for each repo (matches create mode pattern).
    local -a last_status last_line_count repo_pids done_flags error_msgs repo_stale spawn_poll err_files
    local i=0
    for i in {1..$total}; do
        last_status[$i]="queued"
        last_line_count[$i]=0
        done_flags[$i]=0
        repo_pids[$i]=""
        error_msgs[$i]=""
        repo_stale[$i]=0
        spawn_poll[$i]=0
    done

    # Print initial status lines (will be updated in place).
    for i in {1..$total}; do
        print "  ⏳ ${repos[$i]##*/} (queued)"
    done

    local completed=0
    local started=0
    local start_time=$SECONDS

    # Spawn initial batch of workers.
    while (( started < max_jobs && started < total )); do
        (( started++ )) || true
        : > "$tmp_dir/$started"
        err_files[$started]="$tmp_dir/$started.stderr"
        : >| "${err_files[$started]}" 2>/dev/null || true
        last_status[$started]="starting"
        spawn_poll[$started]=0
        (
            _git_wkm_cleanup_repo_worker "${repos[$started]}" "$parent_dir" \
                "$branch_filter" "$should_fetch" "$tmp_dir/$started"
        ) 2>"${err_files[$started]}" &
        repo_pids[$started]=$!
    done

    # Poll and update display in real-time (matches create mode).
    local poll_count=0
    while (( completed < total )); do
        set +x  # Ensure xtrace stays off in the loop

        local j=0
        for j in {1..$total}; do
            (( done_flags[$j] )) && continue

            local out_file="$tmp_dir/$j"
            [[ -f "$out_file" ]] || continue

            local line_num=0
            local line=""
            while IFS= read -r line || [[ -n "$line" ]]; do
                (( line_num++ ))
                (( line_num <= last_line_count[$j] )) && continue

                local key="${line%%:*}"
                local val="${line#*:}"
                case "$key" in
                    status) last_status[$j]="$val" ;;
                    error)  error_msgs[$j]="$val" ;;
                    stale)
                        stale_rows+=("$val")
                        (( total_stale++ ))
                        (( repo_stale[$j]++ ))
                        ;;
                    meta)
                        case "$val" in
                            checked:*) total_checked=$(( total_checked + ${val#checked:} )) ;;
                        esac
                        ;;
                    done)
                        done_flags[$j]=1
                        (( completed++ ))
                        # Build final status.
                        if [[ -n "${error_msgs[$j]}" ]]; then
                            last_status[$j]="failed"
                        elif (( repo_stale[$j] > 0 )); then
                            last_status[$j]="${repo_stale[$j]} stale"
                        else
                            last_status[$j]="clean"
                        fi
                        # Spawn next worker if any remain.
                        if (( started < total )); then
                            (( started++ )) || true
                            : > "$tmp_dir/$started"
                            err_files[$started]="$tmp_dir/$started.stderr"
                            : >| "${err_files[$started]}" 2>/dev/null || true
                            last_status[$started]="starting"
                            spawn_poll[$started]=$poll_count
                            (
                                _git_wkm_cleanup_repo_worker "${repos[$started]}" \
                                    "$parent_dir" "$branch_filter" "$should_fetch" \
                                    "$tmp_dir/$started"
                            ) 2>"${err_files[$started]}" &
                            repo_pids[$started]=$!
                        fi
                        ;;
                esac
            done < "$out_file"
            last_line_count[$j]=$line_num

            # Fallback: if worker exited but never wrote done:, don't hang.
            # Only check after grace period (10 polls = 1s) to let worker start.
            local grace_polls=10
            if (( ! done_flags[$j] )) && [[ -n "${repo_pids[$j]}" ]] && \
               (( poll_count - spawn_poll[$j] >= grace_polls )); then
                if ! kill -0 "${repo_pids[$j]}" 2>/dev/null; then
                    done_flags[$j]=1
                    (( completed++ ))
                    last_status[$j]="failed"
                    [[ -z "${error_msgs[$j]}" ]] && error_msgs[$j]="worker exited unexpectedly"
                    # Spawn next worker if any remain (mirrors done: path).
                    if (( started < total )); then
                        (( started++ )) || true
                        : > "$tmp_dir/$started"
                        err_files[$started]="$tmp_dir/$started.stderr"
                        : >| "${err_files[$started]}" 2>/dev/null || true
                        last_status[$started]="starting"
                        spawn_poll[$started]=$poll_count
                        (
                            _git_wkm_cleanup_repo_worker "${repos[$started]}" \
                                "$parent_dir" "$branch_filter" "$should_fetch" \
                                "$tmp_dir/$started"
                        ) 2>"${err_files[$started]}" &
                        repo_pids[$started]=$!
                    fi
                fi
            fi
        done

        # If any workers reported the generic error, try to pull a real message
        # from their captured stderr.
        for j in {1..$total}; do
            [[ "${error_msgs[$j]:-}" == "worker exited unexpectedly" ]] || continue
            local ef="${err_files[$j]:-}"
            [[ -n "$ef" && -f "$ef" ]] || continue
            local err_line=""
            IFS= read -r err_line < "$ef" 2>/dev/null || err_line=""
            [[ -z "$err_line" ]] && continue
            (( ${#err_line} > 120 )) && err_line="${err_line:0:117}..."
            error_msgs[$j]="$err_line"
        done

        # Update display every few polls to reduce flicker.
        if (( poll_count % 2 == 0 )); then
            printf "\033[%dA" "$total"  # Move cursor up
            for j in {1..$total}; do
                local icon="⏳"
                local st="${last_status[$j]}"
                if (( done_flags[$j] )); then
                    if [[ -n "${error_msgs[$j]}" ]]; then
                        icon="✗"
                    else
                        icon="✓"
                    fi
                fi
                printf "\r\033[K  %s %s (%s)\n" "$icon" "${repos[$j]##*/}" "$st"
            done
        fi
        (( poll_count++ ))

        (( completed < total )) && sleep 0.1
    done

    # Wait for all background workers to finish.
    for j in {1..$total}; do
        if [[ -n "${repo_pids[$j]}" ]] && [[ "${repo_pids[$j]}" != "0" ]]; then
            wait "${repo_pids[$j]}" 2>/dev/null || true
        fi
    done
    # Belt-and-suspenders: wait for any stragglers not tracked in repo_pids.
    wait 2>/dev/null || true

    # Final redraw to ensure all repos show final state.
    printf "\033[%dA" "$total"
    for j in {1..$total}; do
        local icon="✓"
        [[ -n "${error_msgs[$j]}" ]] && icon="✗"
        printf "\r\033[K  %s %s (%s)\n" "$icon" "${repos[$j]##*/}" "${last_status[$j]}"
    done

    local elapsed=$(( SECONDS - start_time ))
    command rm -rf "$tmp_dir" 2>/dev/null || true
    print ""

    # Show errors if any.
    local has_errors=false
    for i in {1..$total}; do
        [[ -n "${error_msgs[$i]}" ]] && has_errors=true && break
    done
    if $has_errors; then
        print "Errors:"
        for i in {1..$total}; do
            [[ -n "${error_msgs[$i]}" ]] && print "  ✗ ${repos[$i]##*/}: ${error_msgs[$i]}"
        done
        print ""
    fi

    print "✓ Scanned $total repo(s) in ${elapsed}s, checked $total_checked worktree(s), found $total_stale stale"

    if (( ${#stale_rows[@]} == 0 )); then
        return 0
    fi

    print "\nStale worktrees (clean + no unique patches vs origin/main):"
    local row
    for row in "${stale_rows[@]}"; do
        local rp="${row%%$'\t'*}"
        local rest="${row#*$'\t'}"
        local wt="${rest%%$'\t'*}"
        local branch="${rest##*$'\t'}"
        print "  - ${rp##*/}: ${branch} (${wt##*/})"
    done

    if [[ "$apply_mode" == "dry-run" ]]; then
        return 0
    fi

    if [[ "$apply_mode" != "yes" ]]; then
        local response=""
        print ""
        read "?Delete these worktrees and local branches? (y/N): " response
        [[ "$response" =~ ^[Yy]$ ]] || return 0
    fi

    print ""
    for row in "${stale_rows[@]}"; do
        local rp="${row%%$'\t'*}"
        local rest="${row#*$'\t'}"
        local wt="${rest%%$'\t'*}"
        local branch="${rest##*$'\t'}"

        print "Removing ${rp##*/}: $branch"
        command git -C "$rp" worktree remove "$wt" >/dev/null 2>&1 || true
        command git -C "$rp" branch -d "$branch" >/dev/null 2>&1 || true
        command git -C "$rp" worktree prune >/dev/null 2>&1 || true
    done

        return 0
    } always {
        XTRACEFD=$saved_xtracefd
        exec {xtrace_devnull_fd}>&- 2>/dev/null
    }
}

###############################################################################
# git wkm: create worktrees across multiple repos and open in Cursor workspace
#
# Usage:
#   git wkm <branch> [repo1 repo2 ...]
#   git wkm <branch>                      # uses GIT_WKM_REPOS if set
#   git wkm <branch> --add [repo1 ...]     # merge into existing workspace
#   git wkm <branch> --replace [repo1 ...] # overwrite workspace folders list
#   git wkm --cleanup [<branch>]           # delete stale worktrees (prompted)
#   git wkm --cleanup --dry-run [<branch>] # list stale worktrees (no fetch)
#   git wkm --cleanup --yes [<branch>]     # delete stale worktrees (no prompt)
#   git wkm --cleanup --fetch [<branch>]   # fetch origin/main before checking
#   git wkm --cleanup --jobs 8 [<branch>]  # parallel repo scan (default 4)
#
# Config:
#   GIT_WKM_REPOS=(repo1 repo2 repo3)     # default repos (names or paths)
#   GIT_WKM_WORKSPACE_DIR=~/.workspaces   # where to store workspace files
###############################################################################
function _git_wkm() {
    emulate -L zsh
    setopt localoptions no_unset

    local mode="create"
    local merge_workspace=true
    local -a repos
    repos=()

    local branch_name=""
    local cleanup_branch=""
    local cleanup_apply="prompt"
    local cleanup_fetch="auto"
    local cleanup_jobs="${GIT_WKM_CLEANUP_JOBS:-4}"

    while (( $# > 0 )); do
        case "$1" in
            --replace)
                merge_workspace=false
                shift
                ;;
            --add|--merge)
                merge_workspace=true
                shift
                ;;
            --cleanup)
                mode="cleanup"
                shift
                ;;
            --dry-run)
                cleanup_apply="dry-run"
                shift
                ;;
            --yes|-y)
                cleanup_apply="yes"
                shift
                ;;
            --fetch)
                cleanup_fetch="yes"
                shift
                ;;
            --no-fetch)
                cleanup_fetch="no"
                shift
                ;;
            --jobs|-j)
                cleanup_jobs="${2:-}"
                shift 2 || break
                ;;
            --help|-h)
                echo "Usage: git wkm <branch> [--add|--replace] [repo...]" >&2
                echo "       git wkm --cleanup [opts] [branch] [repo...]" >&2
                echo "  opts: --dry-run --yes --fetch --no-fetch --jobs N" >&2
                return 1
                ;;
            *)
                if [[ -z "$branch_name" ]]; then
                    branch_name="$1"
                else
                    repos+=("$1")
                fi
                shift
                ;;
        esac
    done

    # Fall back to GIT_WKM_REPOS if no repos specified.
    if (( ${#repos[@]} == 0 )) && (( ${+GIT_WKM_REPOS} )); then
        repos=("${(@)GIT_WKM_REPOS}")
    fi

    if [[ "$mode" == "cleanup" ]]; then
        cleanup_branch="$branch_name"
        local do_fetch="false"
        if [[ "$cleanup_fetch" == "yes" ]]; then
            do_fetch="true"
        elif [[ "$cleanup_fetch" == "no" ]]; then
            do_fetch="false"
        else
            if [[ "$cleanup_apply" != "dry-run" ]]; then
                do_fetch="true"
            fi
        fi

        _git_wkm_cleanup "$cleanup_branch" "$cleanup_apply" \
            "${GIT_WTK_PARENT_DIR:-}" "$cleanup_jobs" "$do_fetch" "${repos[@]}"
        return $?
    fi

    if [[ -z "$branch_name" ]]; then
        echo "Usage: git wkm <branch> [--add|--replace] [repo1 repo2 ...]" >&2
        return 1
    fi

    local parent_dir="${GIT_WTK_PARENT_DIR:-}"
    local workspace_dir="${GIT_WKM_WORKSPACE_DIR:-${HOME}/.workspaces}"
    command mkdir -p "$workspace_dir" 2>/dev/null || true

    # If workspace file exists, use it as source of truth for repos.
    # CLI repos (with or without --add) are appended and deduplicated.
    local ws_name="${branch_name//\//-}"
    local ws_file="$workspace_dir/${ws_name}.code-workspace"

    if [[ -f "$ws_file" ]]; then
        local -a ws_repos
        ws_repos=()
        local dir_name="${branch_name//\//-}"
        local suffix="-worktrees/$dir_name"

        # Extract repo paths from workspace folders.
        local folder_path
        while IFS= read -r folder_path; do
            [[ -z "$folder_path" ]] && continue
            # Strip the -worktrees/branch suffix to get repo path.
            if [[ "$folder_path" == *"$suffix" ]]; then
                ws_repos+=("${folder_path%$suffix}")
            fi
        done < <(jq -r '.folders[].path // empty' "$ws_file" 2>/dev/null)

        # Merge: workspace repos first, then CLI repos (deduplicated by resolved path).
        local -a merged_repos
        merged_repos=("${ws_repos[@]}")
        local r rp found existing
        for r in "${repos[@]}"; do
            # Resolve CLI repo to absolute path for comparison.
            rp="$r"
            [[ ! -d "$rp" && -n "$parent_dir" && -d "$parent_dir/$rp" ]] && rp="$parent_dir/$rp"
            [[ "$rp" != /* ]] && rp="$PWD/$rp"
            rp="${rp:A}"

            # Add if not already in merged_repos.
            found=false
            for existing in "${merged_repos[@]}"; do
                [[ "$existing" == "$rp" ]] && { found=true; break; }
            done
            $found || merged_repos+=("$rp")
        done

        repos=("${merged_repos[@]}")
    fi

    if (( ${#repos[@]} == 0 )); then
        echo "git wkm: no repos specified and GIT_WKM_REPOS not set" >&2
        return 1
    fi

    local total=${#repos[@]}

    # Check if async-cmd is available, fall back to native if not
    local use_async_cmd=false
    if command -v async >/dev/null 2>&1; then
        use_async_cmd=true
    fi

    # Temp dir for job output (and socket if using async-cmd).
    local tmp_dir="${TMPDIR:-/tmp}/git-wkm-$$"
    command mkdir -p "$tmp_dir"
    local socket="$tmp_dir/async.sock"

    if [[ "$use_async_cmd" == "true" ]]; then
        # Start async-cmd server
        async -s="$socket" server --start >/dev/null 2>&1
    else
        # Disable job notifications for native background jobs
        setopt local_options no_notify no_monitor
    fi

    # Print header with parallel indicator.
    if [[ "$use_async_cmd" == "true" ]]; then
        print "Creating worktrees for '$branch_name' in $total repos (async-cmd)..."
    else
        print "Creating worktrees for '$branch_name' in $total repos..."
    fi
    print "  ⚡ Spawning $total parallel workers...\n"

    # Track state for each repo.
    local -a last_status last_line_count wt_paths done_flags pids error_msgs
    for i in {1..$total}; do
        last_status[$i]="spawning"
        last_line_count[$i]=0
        wt_paths[$i]=""
        done_flags[$i]=0
        pids[$i]=""
        error_msgs[$i]=""
    done

    # Print initial status lines (will be updated in place).
    for i in {1..$total}; do
        print "  ⏳ ${repos[$i]} (spawning)"
    done

    local widx=0
    local start_time=$SECONDS
    for repo in "${repos[@]}"; do
        (( widx++ )) || true
        : > "$tmp_dir/$widx"  # create empty file

        if [[ "$use_async_cmd" == "true" ]]; then
            # Submit job to async-cmd (non-blocking)
            # Resolve path now - DOTDOTFILES may not be exported to child shells
            # Use semicolons so variables persist (VAR=x cmd only sets for that cmd)
            local dotfiles_path="${DOTDOTFILES:-$HOME/.dotfiles}"
            async -s="$socket" cmd -- zsh -c \
                "_WKM_REPO='$repo'; \
_WKM_PARENT='$parent_dir'; \
_WKM_BRANCH='$branch_name'; \
_WKM_OUT='$tmp_dir/$widx'; \
_WKM_PWD='$PWD'; \
source \"$dotfiles_path/lib/shell/zsh/git.zsh\" && \
_git_wkm_worker" >/dev/null 2>&1
            pids[$widx]="async"
        else
            # Use native background jobs
            (
                _git_wkm_worker_async "$repo" "$parent_dir" "$branch_name" \
                    "$tmp_dir/$widx" "$PWD"
            ) &!
            pids[$widx]=$!
        fi
    done

    # Move cursor back up to update status lines.
    printf "\033[%dA" "$total"
    for i in {1..$total}; do
        last_status[$i]="pending (pid ${pids[$i]})"
        printf "\r\033[K  ⏳ ${repos[$i]} (pid ${pids[$i]})\n"
    done

    # Poll and update display in real-time.
    local all_done=0
    local poll_count=0
    while (( ! all_done )); do
        all_done=1
        local completed=0
        for i in {1..$total}; do
            if (( done_flags[$i] )); then
                (( completed++ ))
                continue
            fi
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
                    error)  error_msgs[$i]="$val" ;;
                    done)   done_flags[$i]=1; (( completed++ )) ;;
                esac
            done < "$out_file"
            last_line_count[$i]=$line_num
        done

        # Update progress display every few polls.
        if (( poll_count % 3 == 0 )); then
            printf "\033[%dA" "$total"  # move up
            for i in {1..$total}; do
                local icon="⏳"
                (( done_flags[$i] )) && icon="✓"
                printf "\r\033[K  $icon ${repos[$i]} (${last_status[$i]})\n"
            done
        fi
        (( poll_count++ ))
        sleep 0.1
    done

    # Stop async-cmd server if we used it
    if [[ "$use_async_cmd" == "true" ]]; then
        async -s="$socket" server --stop >/dev/null 2>&1
    fi

    local elapsed=$(( SECONDS - start_time ))

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

    # Show full error messages for failed repos.
    local has_errors=false
    for i in {1..$total}; do
        if [[ -n "${error_msgs[$i]}" ]]; then
            has_errors=true
            break
        fi
    done
    if $has_errors; then
        print "Errors:"
        for i in {1..$total}; do
            if [[ -n "${error_msgs[$i]}" ]]; then
                print "  ${repos[$i]}: ${error_msgs[$i]}"
            fi
        done
        print ""
    fi

    if (( ${#worktree_paths[@]} == 0 )); then
        print "git wkm: no worktrees created or found" >&2
        return 1
    fi

    # Create Cursor workspace file with repo names as display names.
    # (ws_name and ws_file already defined above)

    # Build workspace JSON with jq.
    local folders_json="[]" j
    for j in {1..${#worktree_paths[@]}}; do
        folders_json=$(print -r -- "$folders_json" | jq \
            --arg name "${worktree_names[$j]}" \
            --arg path "${worktree_paths[$j]}" \
            '. + [{"name": $name, "path": $path}]') || true
    done

    # Merge in existing workspace folders so "add a repo" doesn't drop others.
    if [[ "$merge_workspace" == "true" && -f "$ws_file" ]]; then
        local existing_folders_json="[]"
        existing_folders_json="$(jq -c '.folders // []' "$ws_file" 2>/dev/null)" \
            || existing_folders_json="[]"

        folders_json="$(jq -n \
            --argjson existing "$existing_folders_json" \
            --argjson new "$folders_json" \
            '
            def add_unique_by_path($arr):
              reduce $arr[] as $item (.;
                if any(.[]; .path == $item.path) then . else . + [$item] end);
            ([] | add_unique_by_path($existing) | add_unique_by_path($new))
            ' 2>/dev/null)" || true
    fi

    # Base settings for git detection
    local base_settings='{
        "git.enabled": true,
        "git.autoRepositoryDetection": "subFolders",
        "git.showCursorWorktrees": true,
        "scm.alwaysShowRepositories": true
    }'

    # Merge custom settings from file or variable
    local custom_settings="{}"
    if [[ -n "${GIT_WKM_SETTINGS_FILE:-}" && -f "$GIT_WKM_SETTINGS_FILE" ]]; then
        custom_settings="$(cat "$GIT_WKM_SETTINGS_FILE")"
    elif [[ -n "${GIT_WKM_SETTINGS:-}" ]]; then
        custom_settings="$GIT_WKM_SETTINGS"
    fi

    local merged_settings
    merged_settings=$(jq -n \
        --argjson base "$base_settings" \
        --argjson custom "$custom_settings" \
        '$base * $custom')

    jq -n --argjson folders "$folders_json" \
        --argjson settings "$merged_settings" \
        '{"folders": $folders, "settings": $settings}' > "$ws_file"

    # Clear Cursor workspace state to prevent repos from being hidden (Bug workaround).
    # Cursor stores SCM visibility state in SQLite; stale state can hide repos.
    _git_wkm_clear_cursor_state "$ws_file"

    # Open in Cursor BEFORE linking so IDE is ready while services sync.
    # Prefer `open` on macOS for reliable window focus, fall back to CLI.
    if [[ "$OSTYPE" == darwin* ]]; then
        open -a "Cursor" "$ws_file" &>/dev/null &!
    elif command -v cursor >/dev/null 2>&1; then
        cursor --reuse-window "$ws_file" &>/dev/null &!
    fi

    # Call post-hook if defined (e.g., for halo integration in .zshrc.local)
    # Hook receives: branch_name, worktree_names array, worktree_paths array
    if (( ${+functions[_git_wkm_post_hook]} )); then
        _git_wkm_post_hook "$branch_name" "${worktree_names[@]}" -- "${worktree_paths[@]}"
    fi

    print ""
    print "✓ ${#worktree_paths[@]} worktree(s) ready in ${elapsed}s"
    print "  Workspace: $ws_file"
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
        if command grep -q "$ws_file" "$ws_json" 2>/dev/null; then
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
                    repo_path="$(builtin cd "$repo_path" 2>/dev/null && pwd -P)" \
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

