## .dotfiles

This is a bespoke collection of my dotfiles wrapped up into a self-updating cross platform package (ubuntu + mac).

The canonical Go module path is `goodkind.io/.dotfiles`, while the repository continues to live on GitHub.

## Setup

### Interactive first time set up (preferred)

```sh
./install.sh
```

Or run it without cloning first:

```sh
curl -fsSL https://raw.githubusercontent.com/agoodkind/.dotfiles/main/install.sh | bash
```

This convenience path executes the current `main` branch installer directly, so
review `install.sh` first if you want to inspect it before running it.
