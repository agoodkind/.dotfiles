# shellcheck shell=bash
###############################################################################
# MOTD (Message of the Day) and Login Info
#
# This module handles:
# 1. Displaying MOTD scripts on shell startup (with smart caching)
# 2. Showing login session information
#
# MOTD CACHING STRATEGY:
# - MOTD scripts can be slow, so we don't run them every shell spawn
# - Cache file tracks last display time; we show MOTD when:
#     a) Force file exists (user explicitly requested via `motd` command)
#     b) SSH connection detected (always, unless explicitly disabled)
#     c) First run (no cache file exists)
#     d) New day OR interval hours have passed since last shown
#
# LOGIN INFO CACHING STRATEGY:
# - The `who` command is slow (~50ms), so we cache session info
# - If cache is < 5 seconds old, skip the `who` call entirely
# - Cache stores: terminal|remote|epoch
###############################################################################

# ==============================================================================
# CONFIGURATION
# ==============================================================================

# How often (in hours) to re-show MOTD within the same day
MOTD_INTERVAL_HOURS="${MOTD_INTERVAL_HOURS:-12}"

# Cache file paths
MOTD_CACHE_FILE="$HOME/.cache/motd_last_shown"   # Tracks last MOTD display time
MOTD_FORCE_FILE="$HOME/.cache/motd_show_next"    # Trigger file for forced display
MOTD_DISABLE_SSH="$HOME/.cache/motd_disable_ssh" # If exists, skip MOTD on SSH
LOGININFO_CACHE="$HOME/.cache/logininfo_session" # Caches last login session

# ==============================================================================
# MOTD DISPLAY DECISION
# ==============================================================================

# Determine if we should show MOTD this session
# Returns: sets should_show_motd to "true" or "false"
_motd_should_show() {
  # Priority 1: Force file exists (user requested via `motd` command)
  if [[ -f "$MOTD_FORCE_FILE" ]]; then
    rm -f "$MOTD_FORCE_FILE"
    echo "true"
    return
  fi

  # Priority 2: SSH connection (always show on SSH, bypassing all cache checks)
  if [[ -n "$SSH_CONNECTION$SSH_CLIENT" ]] && [[ ! -f "$MOTD_DISABLE_SSH" ]]; then
    echo "true"
    return
  fi

  # Priority 3: First run (no cache file yet)
  if [[ ! -f "$MOTD_CACHE_FILE" ]]; then
    mkdir -p "$HOME/.cache"
    echo "true"
    return
  fi

  # Priority 4: Check if enough time has passed
  if _motd_cache_expired; then
    echo "true"
    return
  fi

  echo "false"
}

# Check if MOTD cache has expired (new day or interval hours passed)
# Uses zsh builtins for performance; falls back to showing if unavailable
_motd_cache_expired() {
  zmodload -F zsh/stat b:zstat 2>/dev/null
  if (( ! $+builtins[zstat] )); then
    # Can't check timestamps, default to showing
    return 0
  fi

  local -a last_shown
  zstat -A last_shown +mtime "$MOTD_CACHE_FILE"
  local now=$EPOCHSECONDS

  # Get dates for comparison
  local today=${(%):-"%D{%Y-%m-%d}"}
  local cache_date
  strftime -s cache_date "%Y-%m-%d" "$last_shown"

  # Show if: different day OR interval hours have passed
  if [[ "$cache_date" != "$today" ]]; then
    return 0
  fi

  local hours_since=$(( (now - last_shown) / 3600 ))
  if (( hours_since >= MOTD_INTERVAL_HOURS )); then
    return 0
  fi

  return 1
}

# ==============================================================================
# MOTD EXECUTION
# ==============================================================================

# Run all executable scripts in the MOTD directory
_motd_run_scripts() {
  local motd_dir="$DOTDOTFILES/lib/motd"
  [[ -d "$motd_dir" ]] || return 0

  local script
  for script in "$motd_dir"/*; do
    [[ -f "$script" && -x "$script" ]] && "$script"
  done
}

# Public function: Force MOTD display (also available for manual invocation)
motd() {
  _motd_run_scripts
}

# ==============================================================================
# LOGIN INFO DISPLAY
# ==============================================================================

# Styling helpers for colored output
_li_label()    { print -Pn "%B%F{cyan}${1}%f%b"; }
_li_location() { print -Pn "%B%F{green}${1}%f%b"; }
_li_time()     { print -Pn "%B%F{yellow}${1}%f%b"; }

# Format epoch timestamp as human-readable relative time
# Args: $1=seconds_ago, $2=epoch_timestamp, $3=current_year
_li_relative_time() {
  local diff=$1 epoch=$2 this_year=$3

  # Just now (< 1 minute)
  if (( diff < 60 )); then
    echo "just now"
    return
  fi

  # 1 minute ago
  if (( diff < 120 )); then
    echo "1 minute ago"
    return
  fi

  # N minutes ago (< 1 hour)
  if (( diff < 3600 )); then
    echo "$((diff / 60)) minutes ago"
    return
  fi

  # Today or yesterday (< 2 days) - show time
  if (( diff < 172800 )); then
    local time_str
    strftime -s time_str "%H:%M" $epoch
    if (( diff < 86400 )); then
      echo "at $time_str"
    else
      echo "at $time_str yesterday"
    fi
    return
  fi

  # Older - show full date with ordinal suffix
  local day weekday month time login_year
  strftime -s day "%d" $epoch
  strftime -s weekday "%A" $epoch
  strftime -s month "%b" $epoch
  strftime -s time "%H:%M" $epoch
  strftime -s login_year "%Y" $epoch

  # Remove leading zero and add ordinal suffix
  day=${day#0}
  local suffix="th"
  case $day in
    1|21|31) suffix="st" ;;
    2|22)    suffix="nd" ;;
    3|23)    suffix="rd" ;;
  esac

  if [[ "$login_year" == "$this_year" ]]; then
    echo "on $weekday $month ${day}${suffix} at $time"
  else
    echo "on $weekday $month ${day}${suffix}, $login_year at $time"
  fi
}

# Display current and last login information
# Uses caching to avoid expensive `who` calls on rapid shell spawns
_logininfo() {
  local now=$EPOCHSECONDS
  local this_year
  strftime -s this_year "%Y" $now

  # --- Read cached last login info ---
  local last_terminal="" last_remote="" last_epoch=0
  if [[ -f "$LOGININFO_CACHE" ]]; then
    IFS='|' read -r last_terminal last_remote last_epoch < "$LOGININFO_CACHE"
  fi

  # --- Rate limiting: skip slow `who` call if cache is fresh (<5s) ---
  if (( last_epoch > 0 && (now - last_epoch) < 5 )); then
    local last_src="${last_remote:-$last_terminal}"
    _li_label "Last logged in from "
    _li_location "$last_src"
    echo -n " "
    _li_time "just now"
    echo
    return 0
  fi

  # --- Get current session info (slow: ~50ms) ---
  local current_line
  current_line=$(who am i 2>/dev/null || who -m 2>/dev/null)
  [[ -z "$current_line" ]] && return 0

  # Parse terminal from second field
  local -a words=(${=current_line})
  local current_terminal=${words[2]}

  # Parse remote host from parentheses (e.g., "(192.168.1.1)")
  local current_remote=""
  if [[ "$current_line" =~ '\(([^)]+)\)' ]]; then
    current_remote=${match[1]}
  fi

  local current_src="${current_remote:-$current_terminal}"
  local last_src="${last_remote:-$last_terminal}"

  # --- Update cache asynchronously (won't block shell startup) ---
  async_run print -r "$current_terminal|$current_remote|$now" >"$LOGININFO_CACHE"

  # --- Display login info ---
  if (( last_epoch > 0 )); then
    # Have previous login to compare
    local when=$(_li_relative_time $((now - last_epoch)) $last_epoch $this_year)

    _li_label "Currently logged in from "
    _li_location "$current_src"

    if [[ "$current_src" == "$last_src" ]]; then
      # Same location - shorter message
      echo -n ", "
      _li_label "last logged in "
      _li_time "$when"
    else
      # Different location - show both
      echo -n ", "
      _li_label "last logged in from "
      _li_location "$last_src"
      echo -n " "
      _li_time "$when"
    fi
    echo
  else
    # First login (no previous record)
    _li_label "Currently logged in from "
    _li_location "$current_src"
    echo
  fi
}

# ==============================================================================
# STARTUP EXECUTION
# ==============================================================================

# Show MOTD if conditions are met
should_show_motd=$(_motd_should_show)
if [[ "$should_show_motd" == "true" ]]; then
  _motd_run_scripts
  touch "$MOTD_CACHE_FILE"
fi

# Always show login info
_logininfo
