zmodload zsh/datetime
START_TIME=$EPOCHREALTIME

typeset -gA _ZSHENV_TIMES
_ZSHENV_TIMES[start]=$EPOCHREALTIME

[[ -f "$HOME/.cargo/env" ]] && . "$HOME/.cargo/env"

_ZSHENV_TIMES[cargo]=$(( (EPOCHREALTIME - _ZSHENV_TIMES[start]) * 1000 ))
_ZSHENV_TIMES[end]=$EPOCHREALTIME

# Capture system snapshot at startup for diagnosing pre-zshrc slowness.
# Runs in a subshell so it never blocks the shell startup path.
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
