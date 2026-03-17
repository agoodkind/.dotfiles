#!/usr/bin/env bash
DOTDOTFILES="${DOTDOTFILES:-$HOME/.dotfiles}"
source "$DOTDOTFILES/bash/core/tools.bash"
dotfiles_log_init "zwc-recompile"

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
local_count=0
while IFS= read -r -d '' f; do
    if [[ "$f" -nt "${f}.zwc" ]] \
        || [[ ! -f "${f}.zwc" ]]; then
        dotfiles_run zsh -c "zcompile '$f'"
        local_count=$((local_count + 1))
    fi
done < <(
    find "${dirs[@]}" \
        \( -name '*.zsh' -o -name '.zshrc' -o -name '*.plugin.zsh' \) \
        -print0 2>/dev/null
)
dotfiles_log "compiled $local_count file(s)"
