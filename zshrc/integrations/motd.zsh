# shellcheck shell=bash
###############################################################################
# MOTD (Message of the Day) and Login Info
#
# Startup hot path is at the bottom. zsh/stat loaded once at file top.
#
# MOTD: runs scripts from lib/motd/ based on cache expiry (new day or N hours)
# LOGIN INFO: caches session in ~/.cache/logininfo_session to skip `who`
###############################################################################

zmodload -F zsh/stat b:zstat 2>/dev/null

MOTD_INTERVAL_HOURS="${MOTD_INTERVAL_HOURS:-12}"
MOTD_CACHE_FILE="$HOME/.cache/motd_last_shown"
MOTD_FORCE_FILE="$HOME/.cache/motd_show_next"
MOTD_DISABLE_SSH="$HOME/.cache/motd_disable_ssh"
LOGININFO_CACHE="$HOME/.cache/logininfo_session"

_motd_run_scripts() {
  local motd_dir="$DOTDOTFILES/lib/motd"
  [[ -d "$motd_dir" ]] || return 0
  local script
  for script in "$motd_dir"/*; do
    [[ -f "$script" && -x "$script" ]] && "$script"
  done
}

motd() { _motd_run_scripts; }

_motd_cache_expired() {
  (( $+builtins[zstat] )) || return 0
  local -a last_shown
  zstat -A last_shown +mtime "$MOTD_CACHE_FILE"
  local now=$EPOCHSECONDS
  local today=${(%):-"%D{%Y-%m-%d}"}
  local cache_date
  strftime -s cache_date "%Y-%m-%d" "$last_shown"
  [[ "$cache_date" != "$today" ]] && return 0
  (( (now - last_shown) / 3600 >= MOTD_INTERVAL_HOURS )) && return 0
  return 1
}

# ==============================================================================
# LOGIN INFO (hot path)
# ==============================================================================

_logininfo() {
  local now=$EPOCHSECONDS this_year
  strftime -s this_year "%Y" $now

  local last_terminal="" last_remote="" last_epoch=0
  [[ -f "$LOGININFO_CACHE" ]] && IFS='|' read -r last_terminal last_remote last_epoch < "$LOGININFO_CACHE"

  if (( last_epoch > 0 && (now - last_epoch) < 5 )); then
    print -P "%B%F{cyan}Last logged in from %f%b%B%F{green}${last_remote:-$last_terminal}%f%b %B%F{yellow}just now%f%b"
    return 0
  fi

  local current_terminal current_remote
  if [[ -n "$TTY" ]]; then
    current_terminal="${TTY#/dev/}"
    [[ -n "$SSH_CLIENT" ]] && current_remote="${SSH_CLIENT%% *}"
    [[ -z "$current_remote" && -n "$SSH_CONNECTION" ]] && current_remote="${SSH_CONNECTION%% *}"
  else
    local current_line
    current_line=$(who am i 2>/dev/null || who -m 2>/dev/null)
    [[ -z "$current_line" ]] && return 0
    local -a words=(${=current_line})
    current_terminal=${words[2]}
    [[ "$current_line" =~ '\(([^)]+)\)' ]] && current_remote=${match[1]}
  fi

  local current_src="${current_remote:-$current_terminal}"
  local last_src="${last_remote:-$last_terminal}"

  print -r "$current_terminal|$current_remote|$now" >"$LOGININFO_CACHE" &!

  if (( last_epoch > 0 )); then
    local diff=$((now - last_epoch)) REPLY
    if (( diff < 60 )); then REPLY="just now"
    elif (( diff < 120 )); then REPLY="1 minute ago"
    elif (( diff < 3600 )); then REPLY="$((diff / 60)) minutes ago"
    elif (( diff < 172800 )); then
      local time_str; strftime -s time_str "%H:%M" $last_epoch
      (( diff < 86400 )) && REPLY="at $time_str" || REPLY="at $time_str yesterday"
    else
      local day weekday month time login_year
      strftime -s day "%d" $last_epoch; strftime -s weekday "%A" $last_epoch
      strftime -s month "%b" $last_epoch; strftime -s time "%H:%M" $last_epoch
      strftime -s login_year "%Y" $last_epoch
      day=${day#0}
      local suffix="th"
      case $day in 1|21|31) suffix="st" ;; 2|22) suffix="nd" ;; 3|23) suffix="rd" ;; esac
      [[ "$login_year" == "$this_year" ]] \
        && REPLY="on $weekday $month ${day}${suffix} at $time" \
        || REPLY="on $weekday $month ${day}${suffix}, $login_year at $time"
    fi

    if [[ "$current_src" == "$last_src" ]]; then
      print -P "%B%F{cyan}Currently logged in from %f%b%B%F{green}${current_src}%f%b, %B%F{cyan}last logged in %f%b%B%F{yellow}${REPLY}%f%b"
    else
      print -P "%B%F{cyan}Currently logged in from %f%b%B%F{green}${current_src}%f%b, %B%F{cyan}last logged in from %f%b%B%F{green}${last_src}%f%b %B%F{yellow}${REPLY}%f%b"
    fi
  else
    print -P "%B%F{cyan}Currently logged in from %f%b%B%F{green}${current_src}%f%b"
  fi
}

# ==============================================================================
# STARTUP EXECUTION
# ==============================================================================

if [[ -f "$MOTD_FORCE_FILE" ]]; then
  rm -f "$MOTD_FORCE_FILE"
  _motd_run_scripts
  touch "$MOTD_CACHE_FILE"
elif [[ -n "$SSH_CONNECTION$SSH_CLIENT" ]] && [[ ! -f "$MOTD_DISABLE_SSH" ]]; then
  _motd_run_scripts
  touch "$MOTD_CACHE_FILE"
elif [[ ! -f "$MOTD_CACHE_FILE" ]]; then
  mkdir -p "$HOME/.cache"
  _motd_run_scripts
  touch "$MOTD_CACHE_FILE"
elif _motd_cache_expired; then
  _motd_run_scripts
  touch "$MOTD_CACHE_FILE"
fi

_logininfo
