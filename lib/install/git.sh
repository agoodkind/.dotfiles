#!/usr/bin/env bash

# include the global gitconfig
git --git-dir="$DOTDOTFILES"/.git \
    --work-tree="$DOTDOTFILES" \
    config --global include.path "$DOTDOTFILES/lib/.gitconfig_incl"

# Set git user name and email
if [[ -z "$(git config --global user.name)" ]]; then
    read -r -p "Enter your git user name: " git_user_name
    git config --global user.name "$git_user_name"
fi

if [[ -z "$(git config --global user.email)" ]]; then
    read -r -p "Enter your git email: " git_user_email
    git config --global user.email "$git_user_email"
fi

if [[ -z "$(git config --global gpg.ssh.defaultKeyCommand)" ]]; then
    if ssh-add -L 2>/dev/null | grep -q "ed25519"; then
        git_ssh_key_full=$(ssh-add -L | grep "ed25519" | head -n 1)
        git_ssh_key_id=$(echo "$git_ssh_key_full" | awk '{print $2}')
        git config --global gpg.ssh.defaultKeyCommand "ssh-add -L | grep '$git_ssh_key_id'"
        git config --global user.signingKey "key::$git_ssh_key_full"
    elif [[ -f "$HOME/.ssh/id_ed25519.pub" ]]; then
        git_ssh_key_full=$(cat "$HOME/.ssh/id_ed25519.pub")
        git config --global user.signingKey "key::$git_ssh_key_full"
    else
        read -r -p "Enter path to your SSH public key (or leave empty to skip): " git_ssh_key_path
        if [[ -n "$git_ssh_key_path" && -f "$git_ssh_key_path" ]]; then
            git_ssh_key_full=$(cat "$git_ssh_key_path")
            git config --global user.signingKey "key::$git_ssh_key_full"
        fi
    fi
fi