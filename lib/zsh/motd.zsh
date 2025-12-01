# shellcheck shell=bash
###############################################################################
# MOTD
MOTD_INTERVAL_HOURS="${MOTD_INTERVAL_HOURS:-12}"
MOTD_CACHE_FILE="$HOME/.cache/motd_last_shown"
MOTD_FORCE_FILE="$HOME/.cache/motd_show_next"

should_show_motd=false

# Force flag (highest priority)
if [[ -f "$MOTD_FORCE_FILE" ]]; then
  should_show_motd=true
  rm -f "$MOTD_FORCE_FILE"
# SSH connections
elif [[ -n "$SSH_CONNECTION$SSH_CLIENT" ]] && [[ ! -f "$HOME/.cache/motd_disable_ssh" ]]; then
  should_show_motd=true
# First run or periodic check
elif [[ ! -f "$MOTD_CACHE_FILE" ]]; then
  mkdir -p "$HOME/.cache"
  should_show_motd=true
else
  # Use zsh builtins for timestamps
  zmodload -F zsh/stat b:zstat 2>/dev/null
  if (( $+builtins[zstat] )); then
    zstat -A last_shown +mtime "$MOTD_CACHE_FILE"
    now=$EPOCHSECONDS
    last_date=${(%):-"%D{%Y-%m-%d}"}
    strftime -s last_shown_date "%Y-%m-%d" "$last_shown"
    if [[ "$last_shown_date" != "$last_date" ]] || (( (now - last_shown) / 3600 >= MOTD_INTERVAL_HOURS )); then
      should_show_motd=true
    fi
  else
    should_show_motd=true
  fi
fi



# Login info styling helpers
_li_label() { print -Pn "%B%F{cyan}${1}%f%b"; }
_li_location() { print -Pn "%B%F{green}${1}%f%b"; }
_li_time() { print -Pn "%B%F{yellow}${1}%f%b"; }

# Format relative time
_li_relative_time() {
  local diff=$1 epoch=$2 this_year=$3
  if (( diff < 60 )); then
    echo "just now"
  elif (( diff < 120 )); then
    echo "1 minute ago"
  elif (( diff < 3600 )); then
    echo "$((diff / 60)) minutes ago"
  elif (( diff < 172800 )); then
    local time_str
    strftime -s time_str "%H:%M" $epoch
    (( diff < 86400 )) && echo "at $time_str" || echo "at $time_str yesterday"
  else
    local day weekday month time login_year suffix="th"
    strftime -s day "%d" $epoch
    strftime -s weekday "%A" $epoch
    strftime -s month "%b" $epoch
    strftime -s time "%H:%M" $epoch
    strftime -s login_year "%Y" $epoch
    day=${day#0}
    case $day in
      1|21|31) suffix="st";; 2|22) suffix="nd";; 3|23) suffix="rd";;
    esac
    [[ "$login_year" == "$this_year" ]] \
      && echo "on $weekday $month ${day}${suffix} at $time" \
      || echo "on $weekday $month ${day}${suffix}, $login_year at $time"
  fi
}

# Inline logininfo using zsh builtins (no bash fork)
function _logininfo {
  local CACHE_FILE="$HOME/.cache/logininfo_session"
  local now=$EPOCHSECONDS this_year
  strftime -s this_year "%Y" $now

  # Read cache first
  local last_terminal="" last_remote="" last_epoch=0
  [[ -f "$CACHE_FILE" ]] && IFS='|' read -r last_terminal last_remote last_epoch < "$CACHE_FILE"

  local current_terminal current_remote current_src

  # Rate limit: if cache is < 5 seconds old, skip slow `who` call, just show last login
  if (( last_epoch > 0 && (now - last_epoch) < 5 )); then
    local last_src="${last_remote:-$last_terminal}"
    _li_label "Last logged in from "; _li_location "$last_src"; echo -n " "; _li_time "just now"; echo
    return 0
  fi

  # Get current session (who is slow ~50ms)
  local current_line=$(who am i 2>/dev/null || who -m 2>/dev/null)
  [[ -z "$current_line" ]] && return 0

  # Extract terminal using zsh word splitting
  local -a words=(${=current_line})
  current_terminal=${words[2]}

  # Extract source from parentheses using zsh regex
  current_remote=""
  if [[ "$current_line" =~ '\(([^)]+)\)' ]]; then
    current_remote=${match[1]}
  fi
  current_src="${current_remote:-$current_terminal}"

  local last_src="${last_remote:-$last_terminal}"

  # Background: cache current session
  print -r "$current_terminal|$current_remote|$now" > "$CACHE_FILE" &!

  if [[ $last_epoch -gt 0 ]]; then
    local when=$(_li_relative_time $((now - last_epoch)) $last_epoch $this_year)
    
    _li_label "Currently logged in from "; _li_location "$current_src"
    if [[ "$current_src" == "$last_src" ]]; then
      echo -n ", "; _li_label "last logged in "; _li_time "$when"
    else
      echo -n ", "; _li_label "last logged in from "; _li_location "$last_src"; echo -n " "; _li_time "$when"
    fi
    echo
  else
    _li_label "Currently logged in from "; _li_location "$current_src"; echo
  fi
}

motd() {
  local MOTD_DIR="$DOTDOTFILES/lib/motd"
  [[ -d "$MOTD_DIR" ]] || return 0
  for script in "$MOTD_DIR"/*; do
    [[ -f "$script" && -x "$script" ]] && "$script"
  done
}

if [[ "$should_show_motd" == "true" ]]; then
  MOTD_DIR="$DOTDOTFILES/lib/motd"
  if [[ -d "$MOTD_DIR" ]]; then
    for script in "$MOTD_DIR"/*; do
      [[ -f "$script" && -x "$script" ]] && "$script"
    done
  fi
  touch "$MOTD_CACHE_FILE"
fi

# Show login info (inline, no fork)
_logininfo