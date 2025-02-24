if [ -f "$HOME/.zshrc" ]; then
    timestamp=$(date +"%Y%m%d_%H%M%S")
    mv "$HOME/.zshrc" "$HOME/.zshrc.bak-$timestamp"
fi

if [ -f "$HOME/.vimrc" ]; then
    timestamp=$(date +"%Y%m%d_%H%M%S")
    mv "$HOME/.vimrc" "$HOME/.vimrc.bak-$timestamp"
fi

echo "Creating symlink from $HOME/.dotfiles/.zshrc to $HOME/.zshrc"
ln -s "$HOME/.dotfiles/.zshrc" "$HOME/.zshrc"

echo "Creating symlink from $HOME/.dotfiles/.vimrc to $HOME/.vimrc"
ln -s "$HOME/.dotfiles/.vimrc" "$HOME/.vimrc"