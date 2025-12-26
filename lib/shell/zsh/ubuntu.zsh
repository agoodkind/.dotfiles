# shellcheck shell=bash

# Use systemd-managed SSH agent
export SSH_AUTH_SOCK="$XDG_RUNTIME_DIR/ssh-agent.socket"

########################
# SSH key persistence  #
# Ensure SSH key is loaded in agent after reboot
# Run in background to not block shell startup
_ubuntu_load_ssh_key() {
    if ! ssh-add -l 2>/dev/null | grep -q "id_ed25519"; then
        ssh-add ~/.ssh/id_ed25519
    fi
} >> ~/.cache/ssh-add.log 2>&1
async_run _ubuntu_load_ssh_key

####################################
# Platform-specific customizations #
# alias pbcopy="ssh alexs-mba pbcopy"
alias brew="sudo apt install"
alias bat="batcat"
alias ip="ip --color=always"
alias ifconfig="ip a"
####################################
