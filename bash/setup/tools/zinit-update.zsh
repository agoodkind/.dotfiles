#!/usr/bin/env zsh
# Runs zinit update and compile, then verifies plugin health.
# Invoked by update_zinit_plugins in sync.bash; output is routed to sync.log.
#
# zinit update --all --quiet terminates the zsh session (exit 1) when it
# successfully pulls new commits for any plugin. Running the update in a
# child process isolates that behavior so compile and verification still run.

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"

source "$DOTDOTFILES/lib/zinit/zinit.zsh"

# --- Update (isolated) ---
# Run in a child zsh so zinit's session-killing exit cannot prevent the
# compile and verification phases from running.
zsh -c "source \"$DOTDOTFILES/lib/zinit/zinit.zsh\"; zinit update --all --quiet" 2>&1
update_rc=$?
printf "[zinit-update-exit: %d]\n" "$update_rc"

# --- Compile ---
zinit compile --all 2>&1
compile_rc=$?
printf "[zinit-compile-exit: %d]\n" "$compile_rc"

# --- Verify plugin health ---
# Confirm every plugin directory (except _local---zinit and custom) is a git
# repo on a real branch, not a detached HEAD left behind by a failed update.
plugins_dir="${ZINIT[PLUGINS_DIR]:-$HOME/.local/share/zinit/plugins}"
verify_rc=0

for plugin_dir in "$plugins_dir"/*(N/); do
    local name="${plugin_dir:t}"
    [[ $name = custom || $name = _local---zinit ]] && continue

    if [[ ! -d "$plugin_dir/.git" ]]; then
        printf "[zinit-verify] %s: not a git repo\n" "$name"
        continue
    fi

    local head_ref
    head_ref=$(command git -C "$plugin_dir" symbolic-ref --quiet HEAD 2>/dev/null)
    if [[ -z "$head_ref" ]]; then
        printf "[zinit-verify] %s: detached HEAD\n" "$name"
        verify_rc=1
    fi
done

if (( verify_rc == 0 )); then
    printf "[zinit-verify: ok]\n"
fi

# compile is the only phase we control; update exit is unreliable
(( compile_rc == 0 && verify_rc == 0 ))
