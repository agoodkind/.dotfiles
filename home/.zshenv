zmodload zsh/datetime
START_TIME=$EPOCHREALTIME

# AI agent compatibility mode (Cursor, Claude Code):
# keep shell metacharacters literal unless explicitly handled by the command.
# Only applies in a non-interactive shell; an interactive shell (zsh -i) needs
# normal glob/history behavior for zshrc to work.
if { [[ -n "$CURSOR_AGENT" ]] || [[ -n "$CLAUDECODE" ]]; } && [[ ! -o interactive ]]; then
    setopt NO_GLOB
    setopt NO_NOMATCH
    unsetopt BANG_HIST
    unsetopt HISTSUBSTPATTERN
fi

# On-demand zprof: `zsh_profile` touches ~/.zsh_profile_next to arm.
# .zshenv loads zprof here (earliest user file); perf.zsh has a fallback.
typeset -gi _ZPROF_ARMED=0 _ZPROF_LOADED=0
if [[ -f ~/.zsh_profile_next ]]; then
    _ZPROF_ARMED=1
    rm -f ~/.zsh_profile_next
    zmodload zsh/zprof 2>/dev/null && _ZPROF_LOADED=1
fi

# Unified perf tree: all callsites push depth:label:ms[:tag] entries.
# lib/tree.zsh renders the array as a tree; perf.zsh is a thin consumer.
typeset -ga _PERF_TREE=()
typeset -gF _PERF_LAP=$EPOCHREALTIME

# Delta-based probe: computes ms since last probe, pushes into _PERF_TREE.
# Callsites in /etc/zprofile, ~/.zprofile, /etc/zshrc use this.
function _perf_push() {
    local push_depth=$1 push_label=$2 push_tag=${3:-}
    local push_now=$EPOCHREALTIME
    local push_ms=$(( (push_now - _PERF_LAP) * 1000 ))
    _PERF_TREE+=("${push_depth}:${push_label}:${push_ms}${push_tag:+:${push_tag}}")
    _PERF_LAP=$push_now
}

# Bypass /etc/zshrc locale fork: LC_CTYPE is always UTF-8 on modern macOS.
# setopt COMBINING_CHARS here; /etc/zshrc checks _LOCALE_DONE and skips the fork.
typeset -gi _LOCALE_DONE=1
setopt COMBINING_CHARS

# Bypass /etc/zprofile path_helper: source a cached version of its output.
# Cache is invalidated when /etc/paths.d/ is newer than the cache file.
# /etc/zprofile checks _PATH_HELPER_DONE and skips path_helper if set.
typeset -gi _PATH_HELPER_DONE=0
_path_cache="${HOME}/.cache/zsh_startup/path_cache.zsh"
if [[ -f "$_path_cache" && "$_path_cache" -nt /etc/paths.d ]]; then
    source "$_path_cache"
    _PATH_HELPER_DONE=1
fi
unset _path_cache

export PATH="$HOME/.local/bin:$HOME/.local/bin/scripts:$PATH"

if [[ -f "$HOME/.cargo/env" ]]; then
    source "$HOME/.cargo/env"
fi

# .zshenv self-time: everything between START_TIME and now
typeset -gA _PROFILE_TIMES
_PROFILE_TIMES[_zshenv_self]=$(( (EPOCHREALTIME - START_TIME) * 1000 ))

# Anchor: reset lap so the next _perf_push captures the .zshenv→/etc/zprofile gap
_PERF_LAP=$EPOCHREALTIME

# Capture system snapshot at startup for diagnosing pre-zshrc slowness.
# Runs in a subshell so it never blocks the shell startup path.
# Outer { } 2>/dev/null suppresses nice(5) and mkdir errors in sandboxed envs.
{
    (
        snap_file="${HOME}/.cache/zsh_startup/.syssnap_$$"
        mkdir -p "${HOME}/.cache/zsh_startup"
        {
            printf "# syssnap pid=%d time=%s\n" "$$" "$(date '+%Y-%m-%d %H:%M:%S.%3N %Z')"
            printf "\n## top CPU processes\n"
            ps -Ao pid,pcpu,pmem,comm -r 2>/dev/null | head -15
            printf "\n## load average\n"
            sysctl -n vm.loadavg 2>/dev/null || uptime
            printf "\n## disk activity (iostat)\n"
            iostat -d disk0 2>/dev/null | head -6
            printf "\n## path_helper entries\n"
            ls /etc/paths.d/ 2>/dev/null
            printf "\n## TERM_PROGRAM=%s\n" "${TERM_PROGRAM:-unset}"
        } > "$snap_file" 2>/dev/null
    ) &!
} 2>/dev/null
