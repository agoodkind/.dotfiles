#!/bin/zsh
export DOTDOTFILES=$(pwd)
ZSHRC_HOME=$HOME/.zshrc
VIMRC_HOME=$HOME/.vimrc

# Check for silent mode flag
USE_DEFAULTS=0
while getopts "d" opt; do
    case $opt in
        d)
            USE_DEFAULTS=1
            ;;
    esac
done

if [ $USE_DEFAULTS -eq 0 ]; then
    vared -p "Enter path for Dotfiles: " DOTDOTFILES 
    echo "export DOTDOTFILES=$DOTDOTFILES" > "$DOTDOTFILES/dotfiles_path.zsh"
    vared -p "Enter path for ZSH config: " ZSHRC_HOME 
    vared -p "Enter path for VIM config: " VIMRC_HOME 
else 
    echo "Using default paths"
    echo "Dotfiles: $DOTDOTFILES"
    echo "ZSH config: $ZSHRC_HOME"
    echo "VIM config: $VIMRC_HOME"
fi

echo "Updating plugins and submodules"
git submodule update --init --recursive
git submodule sync
git pull --recurse-submodules

timestamp=$(date +"%Y%m%d_%H%M%S")

BACKUPS_PATH="$DOTDOTFILES/backups"
mkdir -p $BACKUPS_PATH

OMZ_SUBMODULE_PATH="$DOTDOTFILES/lib/.oh-my-zsh"
OMZ_SUBMODULE_CUSTOM_PATH="$OMZ_SUBMODULE_PATH/custom"
OMZ_CUSTOM_PATH="$DOTDOTFILES/lib/omz-custom"
if [ -f "$OMZ_SUBMODULE_PATH/custom" ]; then
    echo "Backing up $OMZ_SUBMODULE_CUSTOM_PATH to $BACKUPS_PATH/custom.bak-$timestamp"
    mv "$OMZ_SUBMODULE_CUSTOM_PATH" "$BACKUPS_PATH/custom.bak-$timestamp"
fi

echo "Creating symlink from $OMZ_CUSTOM_PATH to $OMZ_SUBMODULE_CUSTOM_PATH"
ln -sF "$OMZ_CUSTOM_PATH" "$OMZ_SUBMODULE_CUSTOM_PATH"

if [ -f "$ZSHRC_HOME" ]; then
    echo "Backing up $ZSHRC_HOME to $BACKUPS_PATH/.zshrc.bak-$timestamp"
    mv "$ZSHRC_HOME" "$BACKUPS_PATH/.zshrc.bak-$timestamp"
fi
echo "Creating symlink from $DOTDOTFILES/.zshrc to $ZSHRC_HOME"
ln -sF "$DOTDOTFILES/.zshrc" "$ZSHRC_HOME"

if [ -f "$VIMRC_HOME" ]; then
    echo "Backing up $VIMRC_HOME to $BACKUPS_PATH/.vimrc.bak-$timestamp"
    mv "$VIMRC_HOME" "$BACKUPS_PATH/.vimrc.bak-$timestamp"
fi

echo "Creating symlink from $DOTDOTFILES/.vimrc to $VIMRC_HOME"
ln -sF "$DOTDOTFILES/.vimrc" "$VIMRC_HOME"

# if [[ $(uname) == "Darwin" ]]; then
#     echo "Cleaning up Homebrew before sourcing $VIMRC_HOME"
#     brew cleanup
#     rm -f ~/.zcompdump
#     chmod go-w '/opt/homebrew/share'
#     chmod -R go-w '/opt/homebrew/share/zsh'
# fi

echo "Sourcing $ZSHRC_HOME"
source "$ZSHRC_HOME"