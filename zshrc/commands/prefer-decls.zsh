# shellcheck shell=bash

# cd: zoxide smart jump in TTY, plain builtin otherwise
prefer_tty cd __zoxide_z

# ls
prefer ll eza -lah --icons --group-directories-first
prefer la eza -a --icons --group-directories-first
prefer lt eza --tree --level=2 --icons
prefer llt eza -lah --tree --level=2 --icons
prefer_tty ls ll

# cat / find / grep
prefer bat batcat --style=auto
prefer catt bat --style=auto
prefer rgi rg -i
prefer rgl rg -l

# disk + process tools
prefer top btop
prefer htop btop

prefer cp /bin/cp

# helper CLIs
prefer help tldr
prefer lg lazygit

# npm wrapper prefers pnpm implementation
prefer npm pnpm

prefer docker podman

# ssh helper
prefer sshrm ssh-keygen -R

# Editor: nvim > vim > vi (with sudoedit wrappers)
if isinstalled nvim; then
    export EDITOR=nvim SUDO_EDITOR="nvim -u $HOME/.config/nvim/init.lua"
    export MANPAGER='nvim +Man!' PAGER="$DOTDOTFILES/bin/nvim-pager" MANWIDTH=999
elif isinstalled vim; then
    export EDITOR=vim SUDO_EDITOR=vim
    export MANPAGER="vim -M +MANPAGER --not-a-term -" PAGER=$MANPAGER
else
    export EDITOR=vi SUDO_EDITOR=vi
fi

prefer edit "$EDITOR"
prefer nano "$EDITOR"
prefer emacs "$EDITOR"
prefer vim _edit_maybe_sudoedit "$EDITOR"
prefer vi _edit_maybe_sudoedit "$EDITOR"
prefer nvim _edit_maybe_sudoedit nvim

prefer disable-macos-resume "${DOTFILES_DIR:-$HOME/.dotfiles}/bin/disable-macos-resume"

prefer cursor open -a "/Applications/Cursor.app"
prefer code cursor
