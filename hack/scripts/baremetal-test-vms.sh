#!/usr/bin/env bash
# Provision disposable KVM/libvirt VMs to test the baremetal cluster deploy.
#
# Creates a set of cloud-init Ubuntu VMs (default: 3 control-plane + 3 workers)
# on a dedicated libvirt NAT network, with per-node IPs pinned by MAC. The VMs
# are plain reachable Linux hosts with your SSH key and passwordless sudo --
# exactly what `opencenter cluster deploy` needs for the baremetal provider,
# which SSHes in and runs kubespray against pre-provisioned StaticNodes.
#
# Subcommands:
#   up            Create the network and all VMs, wait for SSH+cloud-init, print a table.
#   doctor        Print the resolved layout (IPs, MACs, workdir, image, keys) -- no action.
#   check/verify  Assert deploy preconditions (ssh, cloud-init, interface, reachability, VIP).
#   reboot [node] Reboot all VMs (or one named node).
#   cleanup       Remove the VMs and their disks (reverse of up). See flags.
#   down          Alias for cleanup.
#   status        Show the network and VM state plus resolved IPs.
#   ssh <node>    SSH into a node by short name (e.g. master-1, worker-2).
#   print-config  Legacy YAML fragment (NOT schema-valid) -- prefer print-merge.
#   print-merge   Emit the opencenter.infrastructure.* overlay fields to merge in.
#
# cleanup flags:
#   --purge-net    also destroy/undefine the libvirt network
#   --purge-cache  also delete the cached base cloud image
#   --all          VMs + disks + network + cache (everything `up` created)
#   --dry-run      list what would be removed, change nothing
#   -y, --yes      skip the confirmation prompt (for CI)
#
# Configuration (environment variables; all have defaults):
#   OC_VM_COUNT          total VM count                      (default 6)
#   OC_VM_MASTERS        first N are control-plane           (default 3)
#   OC_VM_VCPUS          vCPUs per VM                        (default 2)
#   OC_VM_MEMORY_MB      RAM per VM in MiB                   (default 4096)
#   OC_VM_DISK_GB        root disk per VM in GiB             (default 40)
#   OC_VM_MASTERS_VCPUS      master vCPU override            (default OC_VM_VCPUS)
#   OC_VM_MASTERS_MEMORY_MB  master RAM override             (default OC_VM_MEMORY_MB)
#   OC_VM_WORKERS_VCPUS      worker vCPU override            (default OC_VM_VCPUS)
#   OC_VM_WORKERS_MEMORY_MB  worker RAM override             (default OC_VM_MEMORY_MB)
#   OC_VM_PREFIX         name/hostname prefix                (default oc-bm)
#   OC_NET_NAME          libvirt network name                (default oc-baremetal)
#   OC_NET_CIDR          node subnet CIDR                    (default 192.168.123.0/24)
#   OC_NET_VIP           reserved k8s API VIP                (default 192.168.123.10)
#   OC_NET_DHCP_START    DHCP/allocation pool start          (default 192.168.123.100)
#   OC_NET_DHCP_END      DHCP/allocation pool end            (default 192.168.123.200)
#   OC_NODE_IP_BASE      first pinned node IP (last octet)   (default 101)
#   OC_IMAGE_URL         base cloud image URL                (default Ubuntu 22.04)
#   OC_SSH_PUBKEY        public key path injected via cloud-init (auto-detected if unset)
#   OC_SSH_USER          cloud-init login user               (default ubuntu)
#   OC_SSH_PRIVATE_KEY   private key used for ssh/check      (default ${OC_SSH_PUBKEY%.pub})
#   OC_VM_ETHERNAME      in-VM interface name                (default eth0)
#   OC_VM_SSH_TIMEOUT    seconds to wait for each node       (default 300)
#   OC_WORKDIR           state/overlay/cache directory        (default ~/.cache/opencenter/baremetal-vms)
#   OC_LIBVIRT_URI       libvirt connection URI              (default qemu:///system)
#
# Subcommand `check` (aka `verify`) asserts the invariants a baremetal deploy
# depends on -- SSH reachable, cloud-init done, interface named/assigned the
# pinned IP, node-to-node reachability, and that the VIP is free -- so a broken
# environment fails fast instead of 30 minutes into kubespray. `doctor` prints
# the resolved layout (IPs, MACs, workdir, image, seed tool) before any action.
#
# Node IP layout (defaults): masters get OC_NODE_IP_BASE..+N, workers continue
# after a 10-address gap. Keep the pinned range OUTSIDE the DHCP pool.
#
# Requires (Linux host): libvirtd, virsh, virt-install, qemu-img, and one of
# cloud-localds (cloud-image-utils) or genisoimage/mkisofs; plus curl.
#
# Usage:
#   hack/scripts/baremetal-test-vms.sh up
#   hack/scripts/baremetal-test-vms.sh print-config > infra.yaml
#   hack/scripts/baremetal-test-vms.sh cleanup --all -y

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------
OC_VM_COUNT="${OC_VM_COUNT:-6}"
OC_VM_MASTERS="${OC_VM_MASTERS:-3}"
OC_VM_VCPUS="${OC_VM_VCPUS:-2}"
OC_VM_MEMORY_MB="${OC_VM_MEMORY_MB:-4096}"
OC_VM_DISK_GB="${OC_VM_DISK_GB:-40}"
OC_VM_MASTERS_VCPUS="${OC_VM_MASTERS_VCPUS:-$OC_VM_VCPUS}"
OC_VM_MASTERS_MEMORY_MB="${OC_VM_MASTERS_MEMORY_MB:-$OC_VM_MEMORY_MB}"
OC_VM_WORKERS_VCPUS="${OC_VM_WORKERS_VCPUS:-$OC_VM_VCPUS}"
OC_VM_WORKERS_MEMORY_MB="${OC_VM_WORKERS_MEMORY_MB:-$OC_VM_MEMORY_MB}"
OC_VM_PREFIX="${OC_VM_PREFIX:-oc-bm}"
OC_NET_NAME="${OC_NET_NAME:-oc-baremetal}"
OC_NET_CIDR="${OC_NET_CIDR:-192.168.123.0/24}"
OC_NET_VIP="${OC_NET_VIP:-192.168.123.10}"
OC_NET_DHCP_START="${OC_NET_DHCP_START:-192.168.123.150}"
OC_NET_DHCP_END="${OC_NET_DHCP_END:-192.168.123.200}"
OC_NODE_IP_BASE="${OC_NODE_IP_BASE:-101}"
OC_IMAGE_URL="${OC_IMAGE_URL:-https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img}"
# Auto-detect the SSH public key if the caller did not set one: prefer an
# ed25519 key (most hosts generate that now), then rsa.
if [[ -z "${OC_SSH_PUBKEY:-}" ]]; then
    for cand in \
        "${HOME:?HOME must be set}/.ssh/id_ed25519.pub" \
        "${HOME}/.ssh/id_rsa.pub" \
        "${HOME}/.ssh/id_ecdsa.pub"; do
        if [[ -r "$cand" ]]; then OC_SSH_PUBKEY="$cand"; break; fi
    done
fi
OC_SSH_USER="${OC_SSH_USER:-ubuntu}"
# Private key defaults to the pub key path without the trailing .pub.
OC_SSH_PRIVATE_KEY="${OC_SSH_PRIVATE_KEY:-${OC_SSH_PUBKEY:-${HOME}/.ssh/id_ed25519}.pub%.pub}"
OC_VM_ETHERNAME="${OC_VM_ETHERNAME:-eth0}"
OC_VM_SSH_TIMEOUT="${OC_VM_SSH_TIMEOUT:-300}"
OC_WORKDIR="${OC_WORKDIR:-${HOME}/.cache/opencenter/baremetal-vms}"
OC_LIBVIRT_URI="${OC_LIBVIRT_URI:-qemu:///system}"

VIRSH=(virsh --connect "$OC_LIBVIRT_URI")
CACHE_IMG="${OC_WORKDIR}/base-image.img"
NET_PREFIX=""   # first three octets of OC_NET_CIDR, filled by compute_layout

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
log()  { printf '>> %s\n' "$*"; }
warn() { printf 'warn: %s\n' "$*" >&2; }
fail() { printf 'error: %s\n' "$*" >&2; exit 1; }

usage() {
    sed -n '2,71p' "$0" | sed 's/^# \{0,1\}//'
    exit "${1:-0}"
}

require_cmds() {
    local missing=()
    for c in "$@"; do
        command -v "$c" >/dev/null 2>&1 || missing+=("$c")
    done
    if ((${#missing[@]})); then
        fail "missing required command(s): ${missing[*]}
Install on Debian/Ubuntu with:
  sudo apt-get install -y libvirt-daemon-system virtinst qemu-utils cloud-image-utils curl"
    fi
}

# Pick the cloud-init seed-ISO tool available on this host.
seed_tool() {
    if command -v cloud-localds >/dev/null 2>&1; then echo "cloud-localds"; return; fi
    if command -v genisoimage  >/dev/null 2>&1; then echo "genisoimage";  return; fi
    if command -v mkisofs      >/dev/null 2>&1; then echo "mkisofs";      return; fi
    echo ""
}

preflight() {
    require_cmds virsh virt-install qemu-img curl
    [[ -n "$(seed_tool)" ]] || fail "need cloud-localds (cloud-image-utils) or genisoimage/mkisofs for cloud-init seeds"
    "${VIRSH[@]}" uri >/dev/null 2>&1 || fail "cannot reach libvirt at ${OC_LIBVIRT_URI}; is libvirtd running and are you in the 'libvirt' group?"
    [[ -n "$OC_SSH_PUBKEY" && -r "$OC_SSH_PUBKEY" ]] || fail "SSH public key not readable: ${OC_SSH_PUBKEY:-<none auto-detected>} (set OC_SSH_PUBKEY)"
    [[ -r "$OC_SSH_PRIVATE_KEY" ]] || warn "SSH private key not readable: ${OC_SSH_PRIVATE_KEY} (ssh/check will not work until it is)"
    (( OC_VM_MASTERS <= OC_VM_COUNT )) || fail "OC_VM_MASTERS (${OC_VM_MASTERS}) cannot exceed OC_VM_COUNT (${OC_VM_COUNT})"
}

# Derive network prefix (first three octets) from the CIDR.
compute_layout() {
    local net="${OC_NET_CIDR%/*}"
    NET_PREFIX="${net%.*}"
    [[ "$NET_PREFIX" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "OC_NET_CIDR must be an IPv4 /24-style CIDR, got: ${OC_NET_CIDR}"

    # Guard: pinned node IPs must not fall inside the DHCP pool, or libvirt may
    # hand a pinned address to a different lease. Compare last octets (/24).
    local pool_start="${OC_NET_DHCP_START##*.}" pool_end="${OC_NET_DHCP_END##*.}" idx oct
    for (( idx = 0; idx < OC_VM_COUNT; idx++ )); do
        oct="$(ip_octet_for_index "$idx")"
        if (( oct >= pool_start && oct <= pool_end )); then
            fail "pinned IP ${NET_PREFIX}.${oct} ($(domain_name_for_index "$idx")) is inside the DHCP pool ${OC_NET_DHCP_START}-${OC_NET_DHCP_END}; adjust OC_NODE_IP_BASE or the pool"
        fi
    done
}

# role for VM index (0-based): first OC_VM_MASTERS are masters.
role_for_index() { (( $1 < OC_VM_MASTERS )) && echo "master" || echo "worker"; }

# short node name for index, e.g. master-1 / worker-2
node_name_for_index() {
    local idx="$1" role; role="$(role_for_index "$idx")"
    if [[ "$role" == "master" ]]; then
        echo "master-$((idx + 1))"
    else
        echo "worker-$((idx - OC_VM_MASTERS + 1))"
    fi
}

domain_name_for_index() { echo "${OC_VM_PREFIX}-$(node_name_for_index "$1")"; }

# pinned last octet for index: masters base..; workers base+masters+10gap..
ip_octet_for_index() {
    local idx="$1"
    if (( idx < OC_VM_MASTERS )); then
        echo $(( OC_NODE_IP_BASE + idx ))
    else
        echo $(( OC_NODE_IP_BASE + OC_VM_MASTERS + 10 + (idx - OC_VM_MASTERS) ))
    fi
}

ip_for_index()  { echo "${NET_PREFIX}.$(ip_octet_for_index "$1")"; }

# Deterministic locally-administered MAC derived from prefix+index (no wall clock).
mac_for_index() {
    local idx="$1"
    printf '52:54:00:%02x:%02x:%02x' \
        $(( (OC_NODE_IP_BASE + idx) & 0xff )) \
        $(( idx & 0xff )) \
        $(( (idx * 7 + 13) & 0xff ))
}

confirm() {
    local prompt="$1"
    [[ "${ASSUME_YES:-0}" == "1" ]] && return 0
    read -r -p "${prompt} [y/N] " ans
    [[ "$ans" == "y" || "$ans" == "Y" ]]
}

# Run a command on a node over SSH with a short timeout. Non-zero on failure.
# Usage: node_ssh <ip> <remote-command...>
node_ssh() {
    local ip="$1"; shift
    ssh -i "$OC_SSH_PRIVATE_KEY" \
        -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
        -o ConnectTimeout=8 -o BatchMode=yes \
        "${OC_SSH_USER}@${ip}" "$@" 2>/dev/null
}

# ---------------------------------------------------------------------------
# Network
# ---------------------------------------------------------------------------
net_exists() { "${VIRSH[@]}" net-info "$OC_NET_NAME" >/dev/null 2>&1; }

ensure_network() {
    if net_exists; then
        log "network ${OC_NET_NAME} already defined; reusing it"
    else
        log "defining network ${OC_NET_NAME} (${OC_NET_CIDR})"
        local xml hosts="" idx
        for (( idx = 0; idx < OC_VM_COUNT; idx++ )); do
            hosts+=$(printf "      <host mac='%s' name='%s' ip='%s'/>\n" \
                "$(mac_for_index "$idx")" "$(domain_name_for_index "$idx")" "$(ip_for_index "$idx")")
        done
        xml="$(mktemp)"
        cat >"$xml" <<EOF
<network>
  <name>${OC_NET_NAME}</name>
  <forward mode='nat'/>
  <bridge name='virbr-ocbm' stp='on' delay='0'/>
  <ip address='${NET_PREFIX}.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='${OC_NET_DHCP_START}' end='${OC_NET_DHCP_END}'/>
${hosts}    </dhcp>
  </ip>
</network>
EOF
        "${VIRSH[@]}" net-define "$xml" >/dev/null
        rm -f "$xml"
    fi
    "${VIRSH[@]}" net-start "$OC_NET_NAME" >/dev/null 2>&1 || true
    "${VIRSH[@]}" net-autostart "$OC_NET_NAME" >/dev/null 2>&1 || true
}

# ---------------------------------------------------------------------------
# Base image + per-VM overlays and seeds
# ---------------------------------------------------------------------------
ensure_base_image() {
    mkdir -p "$OC_WORKDIR"
    if [[ -f "$CACHE_IMG" ]]; then
        log "base image cached at ${CACHE_IMG}"
    else
        log "downloading base image from ${OC_IMAGE_URL}"
        curl -fL --retry 3 -o "${CACHE_IMG}.tmp" "$OC_IMAGE_URL"
        mv "${CACHE_IMG}.tmp" "$CACHE_IMG"
    fi
}

make_overlay() {
    local idx="$1" overlay; overlay="${OC_WORKDIR}/$(domain_name_for_index "$idx").qcow2"
    qemu-img create -f qcow2 -F qcow2 -b "$CACHE_IMG" "$overlay" "${OC_VM_DISK_GB}G" >/dev/null
    echo "$overlay"
}

make_seed() {
    local idx="$1" host seed tool pub mac userdata metadata udev_rule
    host="$(domain_name_for_index "$idx")"
    mac="$(mac_for_index "$idx")"
    seed="${OC_WORKDIR}/${host}-seed.iso"
    pub="$(cat "$OC_SSH_PUBKEY")"
    userdata="$(mktemp)"; metadata="$(mktemp)"
    # cloud-init writes a udev rule that names this VM's NIC ${OC_VM_ETHERNAME}
    # (matched by its pinned MAC) at first boot, before the interface is up.
    # This keeps kubespray/kube-vip interface assumptions (which expect eth0)
    # true across cloud images, without needing a post-boot reboot.
    udev_rule="SUBSYSTEM==\"net\", ACTION==\"add\", ATTR{address}==\"${mac}\", NAME=\"${OC_VM_ETHERNAME}\""
    cat >"$userdata" <<EOF
#cloud-config
hostname: ${host}
fqdn: ${host}.local
manage_etc_hosts: true
users:
  - name: ${OC_SSH_USER}
    sudo: ALL=(ALL) NOPASSWD:ALL
    groups: [sudo]
    shell: /bin/bash
    ssh_authorized_keys:
      - ${pub}
ssh_pwauth: false
package_update: true
packages:
  - qemu-guest-agent
write_files:
  - path: /etc/udev/rules.d/70-eth0.rules
    content: |
      ${udev_rule}
runcmd:
  - systemctl enable --now qemu-guest-agent
  - udevadm control --reload-rules || true
EOF
    cat >"$metadata" <<EOF
instance-id: ${host}
local-hostname: ${host}
EOF
    tool="$(seed_tool)"
    case "$tool" in
        cloud-localds) cloud-localds "$seed" "$userdata" "$metadata" >/dev/null ;;
        genisoimage|mkisofs)
            # cloud-init reads "user-data" and "meta-data" from the cidata volume.
            # mkisofs/genisoimage name ISO files by the source basename, so stage the
            # temp files under those exact names first (mktemp names would be ignored).
            local staged; staged="$(mktemp -d)"
            cp "$userdata" "${staged}/user-data"
            cp "$metadata" "${staged}/meta-data"
            "$tool" -output "$seed" -volid cidata -joliet -rock "${staged}/user-data" "${staged}/meta-data" >/dev/null 2>&1
            rm -rf "$staged"
            ;;
    esac
    rm -f "$userdata" "$metadata"
    # Self-check: the cidata volume must expose a non-empty "user-data" file, or
    # cloud-init will silently no-op and the VM never configures SSH/hostname.
    verify_seed "$seed"
    echo "$seed"
}

# verify_seed confirms a cloud-init seed ISO contains a non-empty user-data.
# Uses isoinfo (genisoimage) when available, else a read-only loop mount.
verify_seed() {
    local seed="$1"
    local list
    if command -v isoinfo >/dev/null 2>&1; then
        list="$(isoinfo -l -i "$seed" 2>/dev/null || true)"
        if ! grep -qiE '(^|/)?user-data$' <<<"$list"; then
            fail "seed ${seed} has no 'user-data' file; cloud-init will not run"
        fi
        return 0
    fi
    # Fallback: read-only loop mount (needs root; skip gracefully if unavailable).
    local mnt; mnt="$(mktemp -d)"
    if mount -o loop,ro "$seed" "$mnt" 2>/dev/null; then
        if [[ ! -s "$mnt/user-data" ]]; then
            umount "$mnt" 2>/dev/null || true
            rmdir "$mnt" 2>/dev/null || true
            fail "seed ${seed} has no non-empty 'user-data'; cloud-init will not run"
        fi
        umount "$mnt" 2>/dev/null || true
        rmdir "$mnt" 2>/dev/null || true
    else
        rm -rf "$mnt" 2>/dev/null || true
        warn "could not verify seed ${seed} (no isoinfo, loop mount failed); skipping check"
    fi
}

# ---------------------------------------------------------------------------
# VM lifecycle
# ---------------------------------------------------------------------------
domain_exists() { "${VIRSH[@]}" dominfo "$1" >/dev/null 2>&1; }

create_vm() {
    local idx="$1" dom role vcpus mem overlay seed mac
    dom="$(domain_name_for_index "$idx")"
    role="$(role_for_index "$idx")"
    if [[ "$role" == "master" ]]; then
        vcpus="$OC_VM_MASTERS_VCPUS"; mem="$OC_VM_MASTERS_MEMORY_MB"
    else
        vcpus="$OC_VM_WORKERS_VCPUS"; mem="$OC_VM_WORKERS_MEMORY_MB"
    fi
    if domain_exists "$dom"; then
        log "domain ${dom} already exists; skipping"
        return 0
    fi
    log "creating ${dom} (${role}, ${vcpus} vCPU, ${mem} MiB, $(ip_for_index "$idx"))"
    overlay="$(make_overlay "$idx")"
    seed="$(make_seed "$idx")"
    mac="$(mac_for_index "$idx")"
    virt-install \
        --connect "$OC_LIBVIRT_URI" \
        --name "$dom" \
        --memory "$mem" \
        --vcpus "$vcpus" \
        --cpu host-passthrough \
        --import \
        --disk "path=${overlay},format=qcow2,bus=virtio" \
        --disk "path=${seed},device=cdrom" \
        --network "network=${OC_NET_NAME},mac=${mac},model=virtio" \
        --os-variant ubuntu22.04 \
        --graphics none \
        --noautoconsole \
        --channel unix,target_type=virtio,name=org.qemu.guest_agent.0 \
        >/dev/null
}

# Wait until a node's SSH port answers AND cloud-init reports success.
# Port-only is not enough: SSH can come up while cloud-init is still mid-flight,
# which masks seed/hostname/interface problems until much later.
# rc: 0 ready, 1 timed out.
wait_for_node() {
    local idx="$1" ip; ip="$(ip_for_index "$idx")"
    local deadline=$(( $(date +%s) + OC_VM_SSH_TIMEOUT ))
    local now
    while (( $(now=$(date +%s)) < deadline )); do
        if timeout 3 bash -c ">/dev/tcp/${ip}/22" 2>/dev/null; then
            # SSH is up; confirm cloud-init finished (rc 0 = done, 2 = still
            # processing but accepted; anything else is a failure).
            if node_ssh "$ip" "cloud-init status --wait" >/dev/null 2>&1; then
                return 0
            fi
            # cloud-init not yet done (or ssh key not yet active) -- keep waiting.
        fi
        sleep 5
    done
    return 1
}

# Wait for all nodes, in parallel, with per-node status reporting.
wait_for_all_nodes() {
    local idx pids=() ready=()
    for (( idx = 0; idx < OC_VM_COUNT; idx++ )); do
        ( wait_for_node "$idx" && echo "$idx:ok" || echo "$idx:fail" ) &
        pids+=("$!")
    done
    local ok=1
    for (( idx = 0; idx < OC_VM_COUNT; idx++ )); do
        local r; r="$(wait "${pids[idx]}")"
        case "$r" in
            "$idx:ok")   log "  $(domain_name_for_index "$idx") ready at $(ip_for_index "$idx")" ;;
            *)           warn "  $(domain_name_for_index "$idx") not ready (check 'virsh console ${domain_name_for_index "$idx"}')"; ok=0 ;;
        esac
    done
    (( ok == 1 ))
}

# ---------------------------------------------------------------------------
# Commands
# ---------------------------------------------------------------------------
cmd_up() {
    preflight
    compute_layout
    ensure_network
    ensure_base_image
    local idx
    for (( idx = 0; idx < OC_VM_COUNT; idx++ )); do
        create_vm "$idx"
    done
    log "waiting for SSH + cloud-init on all nodes (timeout ${OC_VM_SSH_TIMEOUT}s each)..."
    local ok=1
    wait_for_all_nodes || ok=0
    echo
    cmd_status
    echo
    log "next: ./$(basename "$0") check   # verify the deploy preconditions"
    log "      ./$(basename "$0") print-merge  # emit the overlay fields for your config"
    (( ok == 1 ))
}

cmd_status() {
    compute_layout
    printf 'Network: %s  (%s, VIP %s)\n' "$OC_NET_NAME" "$OC_NET_CIDR" "$OC_NET_VIP"
    net_exists && "${VIRSH[@]}" net-info "$OC_NET_NAME" | sed 's/^/  /' || echo "  (not defined)"
    printf '\n%-22s %-8s %-16s %s\n' "DOMAIN" "ROLE" "IP" "STATE"
    local idx dom state
    for (( idx = 0; idx < OC_VM_COUNT; idx++ )); do
        dom="$(domain_name_for_index "$idx")"
        if domain_exists "$dom"; then
            state="$("${VIRSH[@]}" domstate "$dom" 2>/dev/null || echo unknown)"
        else
            state="absent"
        fi
        printf '%-22s %-8s %-16s %s\n' "$dom" "$(role_for_index "$idx")" "$(ip_for_index "$idx")" "$state"
    done
}

# Reboot all (or a single named) VM via the --connect URI.
# doctor: print the resolved layout and environment before any action. Cheap way
# to catch config typos (IP/base/pool collisions, missing key, wrong image).
cmd_doctor() {
    compute_layout
    printf 'libvirt URI:     %s  (reachable: %s)\n' "$OC_LIBVIRT_URI" "$("${VIRSH[@]}" uri >/dev/null 2>&1 && echo yes || echo NO)"
    printf 'workdir:         %s\n' "$OC_WORKDIR"
    printf 'base image:      %s  (%s)\n' "$OC_IMAGE_URL" "$([[ -f "$CACHE_IMG" ]] && echo cached || echo not-downloaded)"
    printf 'seed tool:       %s\n' "$(seed_tool)"
    printf 'ssh user:        %s\n' "$OC_SSH_USER"
    printf 'ssh pubkey:      %s  (%s)\n' "$OC_SSH_PUBKEY" "$([[ -r "$OC_SSH_PUBKEY" ]] && echo ok || echo MISSING)"
    printf 'ssh privkey:     %s  (%s)\n' "$OC_SSH_PRIVATE_KEY" "$([[ -r "$OC_SSH_PRIVATE_KEY" ]] && echo ok || echo MISSING)"
    printf 'interface name:  %s\n' "$OC_VM_ETHERNAME"
    printf 'ssh timeout:     %ss\n' "$OC_VM_SSH_TIMEOUT"
    printf 'network:         %s  %s  (gateway %s.1, VIP %s, DHCP %s-%s)\n' \
        "$OC_NET_NAME" "$OC_NET_CIDR" "$NET_PREFIX" "$OC_NET_VIP" "$OC_NET_DHCP_START" "$OC_NET_DHCP_END"
    printf '\n%-22s %-8s %-18s %-18s %s\n' "DOMAIN" "ROLE" "IP" "MAC" "SPECS"
    local idx vcpus mem
    for (( idx = 0; idx < OC_VM_COUNT; idx++ )); do
        if (( idx < OC_VM_MASTERS )); then
            vcpus="$OC_VM_MASTERS_VCPUS"; mem="$OC_VM_MASTERS_MEMORY_MB"
        else
            vcpus="$OC_VM_WORKERS_VCPUS"; mem="$OC_VM_WORKERS_MEMORY_MB"
        fi
        printf '%-22s %-8s %-18s %-18s %s vCPU / %s MiB\n' \
            "$(domain_name_for_index "$idx")" "$(role_for_index "$idx")" \
            "$(ip_for_index "$idx")" "$(mac_for_index "$idx")" "$vcpus" "$mem"
    done
}

cmd_reboot() {
    compute_layout
    local target="${1:-}" idx dom
    for (( idx = 0; idx < OC_VM_COUNT; idx++ )); do
        dom="$(domain_name_for_index "$idx")"
        if [[ -n "$target" && "$dom" != "$target" && "$(node_name_for_index "$idx")" != "$target" ]]; then
            continue
        fi
        if domain_exists "$dom"; then
            "${VIRSH[@]}" reboot "$dom" >/dev/null 2>&1 && log "rebooted ${dom}" || warn "could not reboot ${dom}"
        else
            warn "${dom} not present; skipping"
        fi
    done
}

# check/verify: assert the invariants a baremetal deploy depends on. Fails (rc 1)
# if any node is unreachable, cloud-init incomplete, the interface is not the
# expected name/IP, nodes can't reach each other, or the VIP is already held.
cmd_check() {
    compute_layout
    local idx ip dom status detail ci ifline m1 w1
    local vip_held=0 failures=0
    log "checking ${OC_VM_COUNT} nodes on ${OC_NET_CIDR} (VIP ${OC_NET_VIP})..."
    for (( idx = 0; idx < OC_VM_COUNT; idx++ )); do
        ip="$(ip_for_index "$idx")"
        dom="$(domain_name_for_index "$idx")"
        status="OK"; detail=""
        if ! node_ssh "$ip" "true"; then
            status="FAIL"; detail="ssh unreachable"
        else
            ci="$(node_ssh "$ip" "cloud-init status 2>/dev/null" || echo unknown)"
            ifline="$(node_ssh "$ip" "ip -4 -br addr show 2>/dev/null | grep -F ${ip} | awk '{print \$1}' | head -1")"
            if [[ "$ifline" != "$OC_VM_ETHERNAME" ]]; then
                status="FAIL"; detail="interface is '${ifline:-<none>}' (expected ${OC_VM_ETHERNAME}) on ${ip}"
            elif ! node_ssh "$ip" "cloud-init status --wait" >/dev/null 2>&1; then
                status="FAIL"; detail="cloud-init not done (${ci})"
            else
                detail="iface=${OC_VM_ETHERNAME} @ ${ip}; cloud-init done"
            fi
        fi
        if [[ "$status" == "FAIL" ]]; then
            failures=$((failures + 1))
        fi
        printf '  [%s] %-22s %s\n' "$status" "$dom" "$detail"
    done

    # Node-to-node reachability: master-1 -> worker-1 (if workers exist).
    if (( OC_VM_COUNT > OC_VM_MASTERS )); then
        m1="$(ip_for_index 0)"
        w1="$(ip_for_index "$OC_VM_MASTERS")"
        if node_ssh "$m1" "ping -c1 -W2 ${w1}" >/dev/null 2>&1; then
            log "  [OK] node-to-node: master-1 -> worker-1"
        else
            log "  [FAIL] node-to-node: master-1 -> worker-1 unreachable"
            failures=$((failures + 1))
        fi
    fi

    # VIP should be free (no VM holds it) before kube-vip claims it.
    for (( idx = 0; idx < OC_VM_COUNT; idx++ )); do
        ip="$(ip_for_index "$idx")"
        if node_ssh "$ip" "true" 2>/dev/null; then
            if node_ssh "$ip" "ip -4 addr show 2>/dev/null | grep -F '${OC_NET_VIP}/'" >/dev/null 2>&1; then
                log "  [WARN] VIP ${OC_NET_VIP} is already held by $(domain_name_for_index "$idx")"
                vip_held=1
            fi
        fi
    done
    if (( vip_held == 0 )); then
        log "  [OK] VIP ${OC_NET_VIP} is free"
    fi

    echo
    if (( failures > 0 )); then
        fail "check found ${failures} problem(s)"
    fi
    log "all checks passed"
}

cmd_ssh() {
    compute_layout
    local short="${1:?usage: ssh <node> (e.g. master-1)}" idx dom
    for (( idx = 0; idx < OC_VM_COUNT; idx++ )); do
        if [[ "$(node_name_for_index "$idx")" == "$short" || "$(domain_name_for_index "$idx")" == "$short" ]]; then
            exec ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
                "${OC_SSH_USER}@$(ip_for_index "$idx")"
        fi
    done
    fail "unknown node: ${short}"
}

# print-merge: emit ONLY the fields that must be overlaid onto the
# init-generated config (opencenter.infrastructure.*), as a paste-ready
# `opencenter cluster set` batch. This is the intended path -- `cluster init
# --type baremetal` writes the complete schema-valid shape; you merge these in.
cmd_print_merge() {
    compute_layout
    local keypath="$OC_SSH_PRIVATE_KEY" idx
    echo "# Apply these to an existing baremetal cluster config, e.g.:"
    echo "#   opencenter cluster set opencenter/oc-baremetal --set '<key>=<value>' ..."
    echo "# or paste into the 'opencenter.infrastructure' block of your config YAML."
    echo "opencenter.infrastructure.os_version=22.04"
    echo "opencenter.infrastructure.networking.subnet_nodes=${OC_NET_CIDR}"
    echo "opencenter.infrastructure.networking.allocation_pool_start=${OC_NET_DHCP_START}"
    echo "opencenter.infrastructure.networking.allocation_pool_end=${OC_NET_DHCP_END}"
    echo "opencenter.infrastructure.networking.gateway=${NET_PREFIX}.1"
    echo "opencenter.infrastructure.networking.vrrp_ip=${OC_NET_VIP}"
    echo "opencenter.infrastructure.networking.vip_interface=${OC_VM_ETHERNAME}"
    echo "opencenter.infrastructure.networking.loadbalancer_provider=metallb"
    echo "opencenter.infrastructure.ssh.user=${OC_SSH_USER}"
    echo "opencenter.infrastructure.ssh.username=${OC_SSH_USER}"
    echo "opencenter.infrastructure.ssh.key_path=${keypath}"
    # authorized_keys[0] cannot be set by index via `cluster set`; edit the YAML.
    echo "# edit YAML: opencenter.infrastructure.ssh.authorized_keys[0]=\"$(cat "$OC_SSH_PUBKEY")\""
    echo "opencenter.infrastructure.compute.master_nodes=$(master_nodes_json)"
    echo "opencenter.infrastructure.compute.worker_nodes=$(worker_nodes_json)"
    echo ""
    echo "# note: 'set' cannot address authorized_keys by index; edit the YAML for that one field."
}

# JSON list of master nodes for the compute.master_nodes field.
master_nodes_json() {
    local idx out=""
    for (( idx = 0; idx < OC_VM_MASTERS; idx++ )); do
        [[ -n "$out" ]] && out+=","
        out+="{\"name\":\"$(domain_name_for_index "$idx")\",\"access_ip_v4\":\"$(ip_for_index "$idx")\"}"
    done
    echo "[${out}]"
}

# JSON list of worker nodes for the compute.worker_nodes field.
worker_nodes_json() {
    local idx out=""
    for (( idx = OC_VM_MASTERS; idx < OC_VM_COUNT; idx++ )); do
        [[ -n "$out" ]] && out+=","
        out+="{\"name\":\"$(domain_name_for_index "$idx")\",\"access_ip_v4\":\"$(ip_for_index "$idx")\"}"
    done
    echo "[${out}]"
}

# print-config: legacy fragment (NOT schema-valid on its own). Kept for
# reference; prefer print-merge, which overlays onto the init-generated config.
cmd_print_config() {
    compute_layout
    local pub; pub="$(cat "$OC_SSH_PUBKEY")"
    local keypath="${OC_SSH_PUBKEY%.pub}"
    echo "# NOTE: this is a FRAGMENT, not a schema-valid config. Prefer print-merge"
    echo "#       (overlay onto the output of 'opencenter cluster init --type baremetal')."
    echo "infrastructure:"
    echo "  provider: baremetal"
    echo "  os_version: \"22.04\""
    echo "  ssh:"
    echo "    user: ${OC_SSH_USER}"
    echo "    username: ${OC_SSH_USER}"
    echo "    key_path: ${keypath}"
    echo "    authorized_keys:"
    echo "      - \"${pub}\""
    echo "  networking:"
    echo "    subnet_nodes: ${OC_NET_CIDR}"
    echo "    allocation_pool_start: ${OC_NET_DHCP_START}"
    echo "    allocation_pool_end: ${OC_NET_DHCP_END}"
    echo "    gateway: ${NET_PREFIX}.1"
    echo "    vrrp_enabled: true"
    echo "    vrrp_ip: ${OC_NET_VIP}"
    echo "    loadbalancer_provider: metallb"
    echo "  compute:"
    echo "    master_count: ${OC_VM_MASTERS}"
    echo "    worker_count: $(( OC_VM_COUNT - OC_VM_MASTERS ))"
    echo "    master_nodes:"
    local idx
    for (( idx = 0; idx < OC_VM_MASTERS; idx++ )); do
        echo "      - { name: $(domain_name_for_index "$idx"), access_ip_v4: $(ip_for_index "$idx") }"
    done
    echo "    worker_nodes:"
    for (( idx = OC_VM_MASTERS; idx < OC_VM_COUNT; idx++ )); do
        echo "      - { name: $(domain_name_for_index "$idx"), access_ip_v4: $(ip_for_index "$idx") }"
    done
}

cmd_cleanup() {
    local purge_net=0 purge_cache=0 dry=0
    while (($#)); do
        case "$1" in
            --purge-net)   purge_net=1 ;;
            --purge-cache) purge_cache=1 ;;
            --all)         purge_net=1; purge_cache=1 ;;
            --dry-run)     dry=1 ;;
            -y|--yes)      ASSUME_YES=1 ;;
            *) fail "unknown cleanup flag: $1" ;;
        esac
        shift
    done
    compute_layout
    require_cmds virsh

    # Discover resources by querying libvirt (prefix-scoped), not a state file,
    # so a half-finished `up` is still fully cleaned.
    local all_doms doms=() d
    all_doms="$("${VIRSH[@]}" list --all --name 2>/dev/null || true)"
    while IFS= read -r d; do
        [[ -n "$d" && "$d" == "${OC_VM_PREFIX}-"* ]] && doms+=("$d")
    done <<<"$all_doms"

    local overlays=() seeds=()
    if [[ -d "$OC_WORKDIR" ]]; then
        while IFS= read -r f; do [[ -n "$f" ]] && overlays+=("$f"); done \
            < <(find "$OC_WORKDIR" -maxdepth 1 -name "${OC_VM_PREFIX}-*.qcow2" 2>/dev/null)
        while IFS= read -r f; do [[ -n "$f" ]] && seeds+=("$f"); done \
            < <(find "$OC_WORKDIR" -maxdepth 1 -name "${OC_VM_PREFIX}-*-seed.iso" 2>/dev/null)
    fi

    log "cleanup scope (prefix '${OC_VM_PREFIX}'):"
    printf '  VMs:      %s\n' "${doms[*]:-<none>}"
    printf '  overlays: %s\n' "${overlays[*]:-<none>}"
    printf '  seeds:    %s\n' "${seeds[*]:-<none>}"
    if (( purge_net )); then
        printf '  network:  %s%s\n' "$OC_NET_NAME" "$(net_exists || echo ' (absent)')"
    else
        printf '  network:  kept (pass --purge-net to remove)\n'
    fi
    if (( purge_cache )); then
        printf '  cache:    %s\n' "$CACHE_IMG"
    else
        printf '  cache:    kept (pass --purge-cache to remove)\n'
    fi

    if (( dry )); then
        log "dry-run: nothing removed"
        return 0
    fi
    if ! confirm "Remove the above resources?"; then
        log "aborted"
        return 0
    fi

    # 1. VMs (destroy if running, then undefine).
    for d in "${doms[@]:-}"; do
        [[ -z "$d" ]] && continue
        "${VIRSH[@]}" destroy "$d" >/dev/null 2>&1 || true
        if ! "${VIRSH[@]}" undefine "$d" --nvram --snapshots-metadata >/dev/null 2>&1; then
            "${VIRSH[@]}" undefine "$d" >/dev/null 2>&1 || true
        fi
        log "removed VM ${d}"
    done

    # 2. Overlays + seeds.
    for f in "${overlays[@]:-}" "${seeds[@]:-}"; do
        [[ -n "$f" && -e "$f" ]] && rm -f "$f" && log "removed $(basename "$f")"
    done

    # 3. Network (gated). Also remove the host bridge if it lingers after the
    #    network is gone (a stale virbr-ocbm can confuse a fresh `up`).
    if (( purge_net )); then
        if net_exists; then
            "${VIRSH[@]}" net-destroy "$OC_NET_NAME" >/dev/null 2>&1 || true
            "${VIRSH[@]}" net-undefine "$OC_NET_NAME" >/dev/null 2>&1 || true
            log "removed network ${OC_NET_NAME}"
        fi
        if ip link show virbr-ocbm >/dev/null 2>&1; then
            if ip link set virbr-ocbm down 2>/dev/null && ip link delete virbr-ocbm 2>/dev/null; then
                log "removed stale bridge virbr-ocbm"
            else
                warn "could not remove bridge virbr-ocbm (still in use?); left in place"
            fi
        fi
    fi

    # 4. Cached base image (gated).
    if (( purge_cache )) && [[ -f "$CACHE_IMG" ]]; then
        rm -f "$CACHE_IMG" && log "removed cached base image"
    fi

    log "cleanup complete"
}

# ---------------------------------------------------------------------------
# Dispatch
# ---------------------------------------------------------------------------
main() {
    local sub="${1:-}"
    [[ $# -gt 0 ]] && shift || true
    case "$sub" in
        up)            cmd_up "$@" ;;
        cleanup|down)  cmd_cleanup "$@" ;;
        status)        cmd_status "$@" ;;
        doctor)        cmd_doctor "$@" ;;
        check|verify)  cmd_check "$@" ;;
        reboot|reset)  cmd_reboot "$@" ;;
        ssh)           cmd_ssh "$@" ;;
        print-config)  cmd_print_config "$@" ;;
        print-merge)   cmd_print_merge "$@" ;;
        -h|--help|help|"") usage 0 ;;
        *) warn "unknown subcommand: ${sub}"; usage 1 ;;
    esac
}

main "$@"
