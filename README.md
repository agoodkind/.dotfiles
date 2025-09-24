## .dotdotfiles

### Directory layout

- `home/*` everything in this directory is linked recursively into `$HOME`, if exists will be backed up to `backups/<timestamp>/*`
- `lib/include` various includes for the final `.zshrc` that is linked into `$HOME`
- `lib/install` platform specific scripts that will only be run when `install.sh` is executed


## Setup

### Automatic (preferred)

```sh
chmod +x install.sh
./install.sh
```
