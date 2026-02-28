#!/usr/bin/env bash
LOCK=~/.cache/zwc-recompile.lock
if [[ -f "$LOCK" ]] && kill -0 "$(< "$LOCK")" 2>/dev/null; then
    exit 0
fi
mkdir -p ~/.cache
echo $$ > "$LOCK"
trap 'rm -f "$LOCK"' EXIT

DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
ZINIT_HOME="${ZINIT_HOME:-$HOME/.local/share/zinit}"
dirs=(
    "$DOTDOTFILES/zshrc"
    "$DOTDOTFILES/lib/zinit"
    "$DOTDOTFILES/lib/zsh-defer"
    "$DOTDOTFILES/home"
    "$DOTDOTFILES/bin"
    "$ZINIT_HOME/plugins"
    "$ZINIT_HOME/snippets"
)
while IFS= read -r -d '' f; do
    if [[ "$f" -nt "${f}.zwc" ]] \
        || [[ ! -f "${f}.zwc" ]]; then
        zsh -c "zcompile '$f'" 2>/dev/null
    fi
done < <(
    find "${dirs[@]}" \
        \( -name '*.zsh' -o -name '.zshrc' -o -name '*.plugin.zsh' \) \
        -print0 2>/dev/null
)
