# shellcheck shell=bash

# -----------------------------------------------------------------------------
# Mosh / ET wrappers: rewrite sshpiper-style syntax to two-hop via proxy
# -----------------------------------------------------------------------------

_sshpiper_proxy_ipv6="3d06:bad:b01::110"
_sshpiper_proxy_ssh_port="2222"
_sshpiper_proxy_inner_key="/root/.ssh/sshpiper_upstream"
_sshpiper_dest_regex='^(.+)@(.+)@ssh\.home\.goodkind\.io$'
_sshpiper_user_svc_regex='^(.+)@(.+)$'

_sshpiper_two_hop_inner() {
    local dest="$1"

    if [[ "$dest" =~ $_sshpiper_dest_regex ]]; then
        printf '%s@%s@localhost' "${match[1]}" "${match[2]}"
        return 0
    fi

    local expanded hostname user
    expanded=$(ssh -G "$dest" 2>/dev/null) || return 1
    hostname=$(awk '/^hostname / {print $2}' <<< "$expanded")
    user=$(awk '/^user / {print $2}' <<< "$expanded")

    [[ "$hostname" == "ssh.home.goodkind.io" ]] || return 1
    [[ "$user" =~ $_sshpiper_user_svc_regex ]] || return 1

    printf '%s@%s@localhost' "${match[1]}" "${match[2]}"
}

_sshpiper_run_two_hop() {
    local cmd="$1"
    local inner_flag="$2"
    shift 2

    (( $# == 0 )) && { command "$cmd"; return; }

    local dest="${@: -1}"
    local -a args=()
    (( $# > 1 )) && args=("${@:1:$#-1}")

    local inner
    inner=$(_sshpiper_two_hop_inner "$dest") || { command "$cmd" "$@"; return; }

    # Use proxy's upstream key for inner SSH (agent forwarding doesn't work through mosh)
    # -tt forces TTY allocation, UpdateHostKeys=no suppresses RSA signature warning
    local inner_ssh_opts="-tt -i $_sshpiper_proxy_inner_key"
    inner_ssh_opts+=" -o StrictHostKeyChecking=accept-new -o UpdateHostKeys=no"

    local inner_cmd="ssh $inner_ssh_opts $inner"

    if [[ "$inner_flag" == '--' ]]; then
        command "$cmd" --ssh="ssh -p $_sshpiper_proxy_ssh_port" \
            "${args[@]}" "root@$_sshpiper_proxy_ipv6" -- \
            bash -c "$inner_cmd"
    else
        command "$cmd" -p "$_sshpiper_proxy_ssh_port" \
            "${args[@]}" "root@$_sshpiper_proxy_ipv6" \
            -c "$inner_cmd"
    fi
}

mosh() { _sshpiper_run_two_hop mosh '--' "$@"; }
et() { _sshpiper_run_two_hop et '-c' "$@"; }
