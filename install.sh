#!/usr/bin/env bash

# exit on error
set -e

DOTDOTFILES="$HOME/.dotfiles"

chmod +x "$DOTDOTFILES/repair.sh"
chmod +x "$DOTDOTFILES/lib/install/apt.sh"
chmod +x "$DOTDOTFILES/lib/install/mac.sh"
chmod +x "$DOTDOTFILES/lib/install/brew.sh"

# set some common git configs
git config --global rerere.enabled true
git config --global push.autoSetupRemote true
git config --global pull.rebase true
git config --global alias.change-commits '!'"f() { VAR=\$1; OLD=\$2; NEW=\$3; shift 3; git filter-branch --env-filter \"if [[ \\\"\$\`echo \$VAR\`\\\" = '\$OLD' ]]; then export \$VAR='\$NEW'; fi\" \$@; }; f"

# run repair.sh
"$DOTDOTFILES/repair.sh"

# if linux, install apt
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    echo "Linux detected"
    echo "Installing apt packages"
    "$DOTDOTFILES/lib/install/apt.sh"
fi

# run mac last because it calls brew which takes forever
# if mac, install brew
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "macOS detected"
    "$DOTDOTFILES/lib/install/mac.sh"
fi