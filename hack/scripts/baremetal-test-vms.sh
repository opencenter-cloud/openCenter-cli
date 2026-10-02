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
#   up            Create the network and all VMs, wait for SSH, print a table.
#   cleanup       Remove the VMs and their disks (reverse of up). See flags.
#   down          Alias for cleanup.
#   status        Show the network and VM state plus resolved IPs.
#   ssh <node>    SSH into a node by short name (e.g. master-1, worker-2).
#   print-config  Emit a paste-ready baremetal infrastructure YAML fragment.
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
#   OC_SSH_PUBKEY        public key path injected via cloud-init (default ~/.ssh/id_rsa.pub)
#   OC_SSH_USER          cloud-init login user               (default ubuntu)
#   OC_WORKDIR           state/overlay/cache directory        (default ~/.cache/opencenter/baremetal-vms)
#   OC_LIBVIRT_URI       libvirt connection URI              (default qemu:///system)
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
OC_SSH_PUBKEY="${OC_SSH_PUBKEY:-${HOME:?HOME must be set}/.ssh/id_rsa.pub}"
OC_SSH_USER="${OC_SSH_USER:-ubuntu}"
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
    sed -n '2,70p' "$0" | sed 's/^# \{0,1\}//'
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
    [[ -r "$OC_SSH_PUBKEY" ]] || fail "SSH public key not readable: ${OC_SSH_PUBKEY} (set OC_SSH_PUBKEY)"
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
    local idx="$1" host seed tool pub userdata metadata
    host="$(domain_name_for_index "$idx")"
    seed="${OC_WORKDIR}/${host}-seed.iso"
    pub="$(cat "$OC_SSH_PUBKEY")"
    userdata="$(mktemp)"; metadata="$(mktemp)"
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
runcmd:
  - systemctl enable --now qemu-guest-agent
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
    echo "$seed"
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

wait_for_ssh() {
    local idx="$1" ip; ip="$(ip_for_index "$idx")"
    local tries=60
    while (( tries-- > 0 )); do
        if timeout 3 bash -c ">/dev/tcp/${ip}/22" 2>/dev/null; then
            return 0
        fi
        sleep 5
    done
    return 1
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
    log "waiting for SSH on all nodes..."
    local idx ok=1
    for (( idx = 0; idx < OC_VM_COUNT; idx++ )); do
        if wait_for_ssh "$idx"; then
            log "  $(domain_name_for_index "$idx") ready at $(ip_for_index "$idx")"
        else
            warn "  $(domain_name_for_index "$idx") did not come up on SSH (check 'virsh console')"
            ok=0
        fi
    done
    echo
    cmd_status
    echo
    log "next: ./$(basename "$0") print-config  # paste into your baremetal cluster config"
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

cmd_print_config() {
    compute_layout
    local pub; pub="$(cat "$OC_SSH_PUBKEY")"
    local keypath="${OC_SSH_PUBKEY%.pub}"
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

    # 3. Network (gated).
    if (( purge_net )) && net_exists; then
        "${VIRSH[@]}" net-destroy "$OC_NET_NAME" >/dev/null 2>&1 || true
        "${VIRSH[@]}" net-undefine "$OC_NET_NAME" >/dev/null 2>&1 || true
        log "removed network ${OC_NET_NAME}"
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
        ssh)           cmd_ssh "$@" ;;
        print-config)  cmd_print_config "$@" ;;
        -h|--help|help|"") usage 0 ;;
        *) warn "unknown subcommand: ${sub}"; usage 1 ;;
    esac
}

main "$@"
