#!/usr/bin/env bash
# Apply performance patches to macOS system zsh startup files.
#
# Patches applied:
#   /etc/zprofile  – adds _perf_push instrumentation and a _PATH_HELPER_DONE
#                    bypass guard so .zshenv can serve a cached PATH.
#   /etc/zshrc     – adds _perf_push instrumentation and a _LOCALE_DONE bypass
#                    guard so .zshenv can skip the locale(1) subprocess fork.
#
# Both patches are idempotent: a sentinel comment is checked first.
# Originals are backed up to $DOTDOTFILES/backups/<timestamp>/ before writing.
# This script must be run as root (or with sudo) on macOS only.

set -euo pipefail

SENTINEL="# DOTFILES_PERF_PATCH_V4"
DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
BACKUPS_PATH=""

log() { echo "[patch-etc-zsh] $*"; }

[[ "$(uname -s)" == "Darwin" ]] || { log "Not macOS — skipping"; exit 0; }
[[ $EUID -eq 0 ]] || { log "Must run as root (use sudo)"; exit 1; }

ensure_backup_dir() {
    if [[ -z "$BACKUPS_PATH" ]]; then
        BACKUPS_PATH="$DOTDOTFILES/backups/$(date +"%Y%m%d_%H%M%S")"
        mkdir -p "$BACKUPS_PATH"
    fi
}

patch_file() {
    local target="$1"
    local content="$2"

    if grep -qF "$SENTINEL" "$target" 2>/dev/null; then
        log "$target already patched — skipping"
        return 0
    fi

    if [[ -f "$target" ]]; then
        ensure_backup_dir
        local backup_file="$BACKUPS_PATH/$(basename "$target").bak"
        cp "$target" "$backup_file"
        log "backed up $target → $backup_file"
    fi

    printf '%s\n' "$content" > "$target"
    log "patched $target"
}

# ---------------------------------------------------------------------------
# /etc/zprofile patch
# ---------------------------------------------------------------------------
# Stock macOS /etc/zprofile (Ventura/Sonoma/Sequoia):
#
#   # System-wide profile for interactive zsh(1) login shells.
#   # Setup user specific overrides for this in ~/.zprofile.
#   if [ -z "$LANG" ]; then
#       export LANG=C.UTF-8
#   fi
#   if [ -x /usr/libexec/path_helper ]; then
#       eval `/usr/libexec/path_helper -s`
#   fi
#
# We wrap path_helper with timing probes and a bypass guard so that .zshenv
# can pre-load a cached PATH and skip the path_helper fork entirely.
# ---------------------------------------------------------------------------
ZPROFILE_CONTENT="${SENTINEL}
# System-wide profile for interactive zsh(1) login shells.

# Setup user specific overrides for this in ~/.zprofile. See zshbuiltins(1)
# and zshoptions(1) for more details.

# No-op fallback when .zshenv didn't define _perf_push (e.g. zsh -c '...')
(( \${+functions[_perf_push]} )) || function _perf_push() { : }

_perf_push 2 '[gap .zshenv→zprofile]'
_perf_push 2 /etc/zprofile

if [ -z \"\$LANG\" ]; then
	export LANG=C.UTF-8
fi
_perf_push 3 preamble

if [ -x /usr/libexec/path_helper ]; then
	if (( ! \${_PATH_HELPER_DONE:-0} )); then
		eval \`/usr/libexec/path_helper -s\`
		_perf_push 3 path_helper \"forked\"
	else
		_perf_push 3 path_helper \"cached\"
	fi
fi"

# ---------------------------------------------------------------------------
# /etc/zshrc patch
# ---------------------------------------------------------------------------
# Stock macOS /etc/zshrc (Ventura/Sonoma/Sequoia) runs:
#   - locale(1) subprocess to detect UTF-8 and conditionally setopt COMBINING_CHARS
#   - history options, terminfo key bindings, PS1, and /etc/zshrc_$TERM_PROGRAM
#
# We add _perf_push calls around each section and guard the locale fork with
# _LOCALE_DONE so .zshenv can skip it unconditionally on modern macOS.
# ---------------------------------------------------------------------------
ZSHRC_CONTENT="${SENTINEL}
# System-wide profile for interactive zsh(1) shells.

# Setup user specific overrides for this in ~/.zshrc. See zshbuiltins(1)
# and zshoptions(1) for more details.

(( \${+functions[_perf_push]} )) || function _perf_push() { : }

_perf_push 2 '[gap .zprofile→zshrc]'
_perf_push 2 /etc/zshrc

if (( ! \${_LOCALE_DONE:-0} )); then
    if [[ ! -x /usr/bin/locale ]] || [[ \"\$(locale LC_CTYPE)\" == \"UTF-8\" ]]; then
        setopt COMBINING_CHARS
    fi
fi
if (( \${_LOCALE_DONE:-0} )); then
    _perf_push 3 combining/locale \"bypassed\"
else
    _perf_push 3 combining/locale
fi

disable log

HISTFILE=\${ZDOTDIR:-\$HOME}/.zsh_history
HISTSIZE=2000
SAVEHIST=1000
setopt BEEP
_perf_push 3 history/opts

if [[ -r \${ZDOTDIR:-\$HOME}/.zkbd/\${TERM}-\${VENDOR} ]] ; then
    source \${ZDOTDIR:-\$HOME}/.zkbd/\${TERM}-\${VENDOR}
else
    typeset -g -A key
    [[ -n \"\$terminfo[kf1]\" ]] && key[F1]=\$terminfo[kf1]
    [[ -n \"\$terminfo[kf2]\" ]] && key[F2]=\$terminfo[kf2]
    [[ -n \"\$terminfo[kf3]\" ]] && key[F3]=\$terminfo[kf3]
    [[ -n \"\$terminfo[kf4]\" ]] && key[F4]=\$terminfo[kf4]
    [[ -n \"\$terminfo[kf5]\" ]] && key[F5]=\$terminfo[kf5]
    [[ -n \"\$terminfo[kf6]\" ]] && key[F6]=\$terminfo[kf6]
    [[ -n \"\$terminfo[kf7]\" ]] && key[F7]=\$terminfo[kf7]
    [[ -n \"\$terminfo[kf8]\" ]] && key[F8]=\$terminfo[kf8]
    [[ -n \"\$terminfo[kf9]\" ]] && key[F9]=\$terminfo[kf9]
    [[ -n \"\$terminfo[kf10]\" ]] && key[F10]=\$terminfo[kf10]
    [[ -n \"\$terminfo[kf11]\" ]] && key[F11]=\$terminfo[kf11]
    [[ -n \"\$terminfo[kf12]\" ]] && key[F12]=\$terminfo[kf12]
    [[ -n \"\$terminfo[kf13]\" ]] && key[F13]=\$terminfo[kf13]
    [[ -n \"\$terminfo[kf14]\" ]] && key[F14]=\$terminfo[kf14]
    [[ -n \"\$terminfo[kf15]\" ]] && key[F15]=\$terminfo[kf15]
    [[ -n \"\$terminfo[kf16]\" ]] && key[F16]=\$terminfo[kf16]
    [[ -n \"\$terminfo[kf17]\" ]] && key[F17]=\$terminfo[kf17]
    [[ -n \"\$terminfo[kf18]\" ]] && key[F18]=\$terminfo[kf18]
    [[ -n \"\$terminfo[kf19]\" ]] && key[F19]=\$terminfo[kf19]
    [[ -n \"\$terminfo[kf20]\" ]] && key[F20]=\$terminfo[kf20]
    [[ -n \"\$terminfo[kbs]\" ]] && key[Backspace]=\$terminfo[kbs]
    [[ -n \"\$terminfo[kich1]\" ]] && key[Insert]=\$terminfo[kich1]
    [[ -n \"\$terminfo[kdch1]\" ]] && key[Delete]=\$terminfo[kdch1]
    [[ -n \"\$terminfo[khome]\" ]] && key[Home]=\$terminfo[khome]
    [[ -n \"\$terminfo[kend]\" ]] && key[End]=\$terminfo[kend]
    [[ -n \"\$terminfo[kpp]\" ]] && key[PageUp]=\$terminfo[kpp]
    [[ -n \"\$terminfo[knp]\" ]] && key[PageDown]=\$terminfo[knp]
    [[ -n \"\$terminfo[kcuu1]\" ]] && key[Up]=\$terminfo[kcuu1]
    [[ -n \"\$terminfo[kcub1]\" ]] && key[Left]=\$terminfo[kcub1]
    [[ -n \"\$terminfo[kcud1]\" ]] && key[Down]=\$terminfo[kcud1]
    [[ -n \"\$terminfo[kcuf1]\" ]] && key[Right]=\$terminfo[kcuf1]
fi
_perf_push 3 terminfo

[[ -n \${key[Delete]} ]] && bindkey \"\${key[Delete]}\" delete-char
[[ -n \${key[Home]} ]] && bindkey \"\${key[Home]}\" beginning-of-line
[[ -n \${key[End]} ]] && bindkey \"\${key[End]}\" end-of-line
[[ -n \${key[Up]} ]] && bindkey \"\${key[Up]}\" up-line-or-search
[[ -n \${key[Down]} ]] && bindkey \"\${key[Down]}\" down-line-or-search
_perf_push 3 bindkey

PS1=\"%n@%m %1~ %# \"

[ -r \"/etc/zshrc_\$TERM_PROGRAM\" ] && . \"/etc/zshrc_\$TERM_PROGRAM\"
_perf_push 3 zshrc_\${TERM_PROGRAM:-unset}"

patch_file /etc/zprofile "$ZPROFILE_CONTENT"
patch_file /etc/zshrc    "$ZSHRC_CONTENT"

log "done"
