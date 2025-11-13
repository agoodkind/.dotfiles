#!/usr/bin/env bash

# include the global gitconfig
echo "Including global gitconfig from $DOTDOTFILES/lib/.gitconfig_incl"
git --git-dir="$DOTDOTFILES"/.git \
    --work-tree="$DOTDOTFILES" \
    config --global include.path "$DOTDOTFILES/lib/.gitconfig_incl"

# Set git user name and email
echo "Setting up git user name and email"
if [[ -z "$(git config --global user.name)" ]]; then
    echo "No git user name found"
    read -r -p "Enter your git user name (First Last): " git_user_name
    git config --global user.name "$git_user_name"
fi

if [[ -z "$(git config --global user.email)" ]]; then
    echo "No git email found"
    read -r -p "Enter your git email: " git_user_email
    git config --global user.email "$git_user_email"
fi

# set up gpg ssh key
if [[ -z "$(git config --global gpg.ssh.defaultKeyCommand)" ]]; then
    echo "Setting up git gpg ssh key for signing commits"
    # check if ed25519 key is in ssh-agent
    if ssh-add -L 2>/dev/null | grep -q "ed25519"; then
        git_ssh_key_full=$(ssh-add -L | grep "ed25519" | head -n 1)
        git_ssh_key_id=$(echo "$git_ssh_key_full" | awk '{print $2}')
        git config --global gpg.ssh.defaultKeyCommand "ssh-add -L | grep '$git_ssh_key_id'"
        git config --global user.signingKey "key::$git_ssh_key_full"
    # check if ed25519 key is in $HOME/.ssh/id_ed25519.pub
    elif [[ -f "$HOME/.ssh/id_ed25519.pub" ]]; then
        git_ssh_key_full=$(cat "$HOME/.ssh/id_ed25519.pub")
        git config --global user.signingKey "key::$git_ssh_key_full"
    else
    # if not, prompt for path to key
        read -r -p "Enter path to your SSH public key (or leave empty to skip): " git_ssh_key_path
        if [[ -n "$git_ssh_key_path" && -f "$git_ssh_key_path" ]]; then
            git_ssh_key_full=$(cat "$git_ssh_key_path")
            git config --global user.signingKey "key::$git_ssh_key_full"
        fi
    fi
fi