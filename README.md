## .dotdotfiles

### Directory layout

- `home/*` everything in this directory is linked recursively into `$HOME`, if exists will be backed up to `backups/<timestamp>/*`
- `lib/include` various includes for the final `.zshrc` that is linked into `$HOME`
- `lib/install` platform specific scripts that will only be run when `install.sh` is executed
- `lib/omz-custom/*` this is for `$ZSH_CUSTOM` which is used for themes and plugins. Anagulous to `.oh-my-zsh/custom`

## Setup

### Automatic (preferred)

```sh
chmod +x install.sh
./install.sh
```

### Manual Setup

`ln -s .zshrc ~/.zshrc`
`ln -s ....`

## Adding omz plugins

Plugins that need to be installed into `.oh-my-zsh/custom` should be installed by calling `install_plugin` with the git url as the argument


