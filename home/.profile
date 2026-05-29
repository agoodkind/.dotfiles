# ~/.profile: sourced by Bourne-compatible login shells (notably `bash -l`).
# zsh does not read this file, and rust/cargo and atuin PATH setup already lives
# in ~/.zshenv and ~/.bashrc, so this file deliberately does not source the tool
# env shims that the rustup and atuin installers append unguarded. Those bare
# `. "$HOME/.cargo/env"` lines error out whenever a login bash starts with $HOME
# unset, which is the only thing they ever did here.
if [ -n "$BASH_VERSION" ] && [ -f "$HOME/.bashrc" ]; then
    . "$HOME/.bashrc"
fi
