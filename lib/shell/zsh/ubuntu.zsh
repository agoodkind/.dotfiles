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

flush_dns() {
    echo "Flushing DNS..."
    local did_any=false

    if command -v resolvectl >/dev/null 2>&1; then
        sudo resolvectl flush-caches >/dev/null 2>&1 && did_any=true
    fi

    if command -v systemd-resolve >/dev/null 2>&1; then
        sudo systemd-resolve --flush-caches >/dev/null 2>&1 && \
            did_any=true
    fi

    if command -v systemctl >/dev/null 2>&1; then
        local svc
        for svc in systemd-resolved NetworkManager network-manager \
            nscd dnsmasq unbound bind9 named; do
            if systemctl cat "$svc" >/dev/null 2>&1; then
                sudo systemctl restart "$svc" >/dev/null 2>&1 && \
                    did_any=true
            fi
        done
    elif command -v service >/dev/null 2>&1; then
        local svc
        for svc in network-manager nscd dnsmasq unbound bind9 named; do
            sudo service "$svc" restart >/dev/null 2>&1 && did_any=true
        done
    fi

    if [[ "$did_any" != "true" ]]; then
        echo "No DNS cache service detected" >&2
        return 0
    fi
}

change_hostname() {
    if [[ -z "$1" ]]; then
        echo "Usage: change_hostname <new_name>"
        return 1
    fi

    local new_name="$1"
    local old_name=$(hostname)

    # Set the system hostname
    if command -v hostnamectl >/dev/null 2>&1; then
        sudo hostnamectl set-hostname "$new_name"
    else
        # Fallback for non-systemd environments
        sudo hostname "$new_name"
        echo "$new_name" | sudo tee /etc/hostname > /dev/null
    fi

    # Update /etc/hosts to preserve sudo resolution speed
    if [[ -f /etc/hosts ]]; then
        sudo sed -i "s/\b${old_name}\b/${new_name}/g" /etc/hosts
    fi

    echo "Hostname changed from '$old_name' to '$new_name'." \
        "You may need to restart your shell."
}
