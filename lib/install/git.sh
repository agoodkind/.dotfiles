#!/usr/bin/env bash

# Source utilities
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/../include/defaults.sh"
source "${SCRIPT_DIR}/../include/colors.sh"

# include the global gitconfig
echo "Including global gitconfig from $DOTDOTFILES/lib/.gitconfig_incl"
git config --global include.path "$DOTDOTFILES/lib/.gitconfig_incl"

# Set git user name and email
echo "Setting up git user name and email"
if [[ -z "$(git config --global user.name)" ]]; then
    if [[ "$USE_DEFAULTS" == "true" ]]; then
        color_echo YELLOW "Skipping git user name (use defaults mode)"
    else
        read_with_default_multiline "Enter your name for git (First Last): " ""
        git_user_name="$REPLY"
        if [[ -n "$git_user_name" ]]; then
            git config --global user.name "$git_user_name"
        fi
    fi
fi

if [[ -z "$(git config --global user.email)" ]]; then
    if [[ "$USE_DEFAULTS" == "true" ]]; then
        color_echo YELLOW "Skipping git user email (use defaults mode)"
    else
        read_with_default_multiline "Enter your git email: " ""
        git_user_email="$REPLY"
        if [[ -n "$git_user_email" ]]; then
            git config --global user.email "$git_user_email"
        fi
    fi
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
        if [[ "$USE_DEFAULTS" == "true" ]]; then
            color_echo YELLOW "Skipping SSH key setup (use defaults mode)"
        else
            read_with_default_multiline "Enter path to your SSH public key (or leave empty to skip): " ""
            git_ssh_key_path="$REPLY"
            if [[ -n "$git_ssh_key_path" && -f "$git_ssh_key_path" ]]; then
                git_ssh_key_full=$(cat "$git_ssh_key_path")
                git config --global user.signingKey "key::$git_ssh_key_full"
            fi
        fi
    fi
fi