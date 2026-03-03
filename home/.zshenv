zmodload zsh/datetime
START_TIME=$EPOCHREALTIME

# On-demand zprof: `zsh_profile` touches ~/.zsh_profile_next to arm.
# .zshenv loads zprof here (earliest user file); perf.zsh has a fallback.
typeset -gi _ZPROF_ARMED=0 _ZPROF_LOADED=0
if [[ -f ~/.zsh_profile_next ]]; then
    _ZPROF_ARMED=1
    rm -f ~/.zsh_profile_next
    zmodload zsh/zprof 2>/dev/null && _ZPROF_LOADED=1
fi

typeset -gA _ZSHENV_TIMES
_ZSHENV_TIMES[start]=$EPOCHREALTIME

# System timeline: ordered probes of depth:label:epoch[:tag] populated by _sys_probe.
# Callsites in /etc/zprofile, ~/.zprofile, /etc/zshrc push entries; perf.zsh
# computes deltas between consecutive probes and renders the tree.
typeset -ga _SYSTEM_ORDER=()

function _sys_probe() {
    local probe_depth=$1 probe_label=$2 probe_tag=${3:-}
    _SYSTEM_ORDER+=("${probe_depth}:${probe_label}:${EPOCHREALTIME}${probe_tag:+:${probe_tag}}")
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

[[ -f "$HOME/.cargo/env" ]] && . "$HOME/.cargo/env"

_ZSHENV_TIMES[end]=$EPOCHREALTIME
# Anchor for the system timeline: the next _sys_probe captures .zshenv→/etc/zprofile gap
_sys_probe 0 .zshenv_end

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
