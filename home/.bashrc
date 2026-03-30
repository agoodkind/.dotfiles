export PATH="$HOME/.local/bin:$HOME/.local/bin/scripts:$PATH"
export PATH="$PATH:/opt/scripts"
export PATH="$PATH:$HOME/go/bin"

if [[ -f "$HOME/.cargo/env" ]]; then
    source "$HOME/.cargo/env"
fi
