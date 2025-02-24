#!/bin/zsh

if [ -f "$HOME/.zshrc" ]; then
    timestamp=$(date +"%Y%m%d_%H%M%S")
    echo "Backing up $HOME/.zshrc to $HOME/.zshrc.bak-$timestamp"\n
    mv "$HOME/.zshrc" "$HOME/.zshrc.bak-$timestamp"
fi

if [ -f "$HOME/.vimrc" ]; then
    timestamp=$(date +"%Y%m%d_%H%M%S")
    echo "Backing up $HOME/.vimrc to $HOME/.vimrc.bak-$timestamp"\n
    mv "$HOME/.vimrc" "$HOME/.vimrc.bak-$timestamp"
fi

echo "Creating symlink from $HOME/.dotfiles/.zshrc to $HOME/.zshrc"\n
ln -sF "$HOME/.dotfiles/.zshrc" "$HOME/.zshrc"

echo "Creating symlink from $HOME/.dotfiles/.vimrc to $HOME/.vimrc"\n
ln -sF "$HOME/.dotfiles/.vimrc" "$HOME/.vimrc"

echo "Sourcing $HOME/.zshrc"
source "$HOME/.zshrc"