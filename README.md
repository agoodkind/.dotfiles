## .dotdotfiles

### Directory layout

- `home/*` everything in this directory is linked recursively into `$HOME`. If a file exists, it will be backed up to `backups/<timestamp>/*`.
- `lib/include` various includes for the final `.zshrc` that is linked into `$HOME`.
- `lib/install` platform-specific scripts run by `install.sh` (`apt.sh`, `brew.sh`, `mac.sh`, `git.sh`).
- `bin/` and `lib/scripts/` custom scripts and utilities.
- `backups/` timestamped backups of replaced files.
-

## Setup

### Automatic (preferred)

```sh
chmod +x install.sh
./install.sh
```
