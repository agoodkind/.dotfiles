#!/usr/bin/env bash

# exit on error
set -e

# run repair.sh
./repair.sh

# install zoxide
curl -sSfL https://raw.githubusercontent.com/ajeetdsouza/zoxide/main/install.sh | sh

# set some common git configs
git config --global rerere.enabled true
git config --global push.autoSetupRemote true
git config --global pull.rebase true

# if mac, install brew
if [[ "$OSTYPE" == "darwin"* ]]; then
    ./lib/brew.sh
fi

# if linux, install apt
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    ./lib/apt.sh
fi