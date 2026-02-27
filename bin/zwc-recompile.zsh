#!/usr/bin/env zsh
LOCK=~/.cache/zwc-recompile.lock
if [[ -f "$LOCK" ]] && kill -0 "$(<"$LOCK")" 2>/dev/null; then
    exit 0
fi
mkdir -p ~/.cache
echo $$ > "$LOCK"
trap 'rm -f "$LOCK"' EXIT

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
setopt nullglob
for f in \
    "$DOTDOTFILES"/zshrc/**/*.zsh \
    "$DOTDOTFILES"/lib/zinit/*.zsh \
    "$DOTDOTFILES"/home/.zshrc; do
    [[ -f "$f" ]] || continue
    if [[ "$f" -nt "${f}.zwc" ]] || [[ ! -f "${f}.zwc" ]]; then
        zcompile "$f" 2>/dev/null
    fi
done
