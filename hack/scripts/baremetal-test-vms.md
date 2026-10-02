# Baremetal test VMs — `baremetal-test-vms.sh`

A single-script way to turn one Linux libvirt host into a set of disposable
Ubuntu 22.04 VMs that act as **pre-provisioned baremetal nodes**, so you can
exercise `opencenter cluster deploy` end to end (the baremetal provider SSHes
into each node and runs Kubespray against them). When you're done, the same
script tears everything down.

This is a **test/development** tool for validating the baremetal deploy path on
a laptop or a small dedicated machine — not a production provisioning system.

> Full design + the as-built findings from the first real run (Gitea/RustFS,
> kube-vip interface naming, kubespray module patches, Flux in-cluster CA) live
> in
> [docs/superpowers/specs/2026-10-01-baremetal-local-vm-cluster-design.md](../../docs/superpowers/specs/2026-10-01-baremetal-local-vm-cluster-design.md).

---

## Requirements

A **Linux host** with KVM (`/dev/kvm`) and these tools:

| Tool | Package (Debian/Ubuntu) | Package (Fedora) |
|---|---|---|
| libvirt daemon + `virsh` | `libvirt-daemon-system` | `libvirt-daemon-kvm` |
| `virt-install` | `virtinst` | `virtinst` |
| `qemu-img` | `qemu-utils` | `qemu-utils` |
| `curl` | `curl` | `curl` |
| cloud-init seed ISO | `cloud-image-utils` (**or** `genisoimage`/`mkisofs`) | `cloud-image-utils` (**or** `genisoimage`) |
| SSH key | `openssh-client` | `openssh-clients` |

Notes:

- You must be able to talk to `qemu:///system`. Either run as root, or be in the
  `libvirt` group (group membership only applies to *new* sessions — re-login,
  or wrap the call in `newgrp libvirt` / `sg libvirt -c '…'`).
- A seed-ISO tool is required. `cloud-localds` (cloud-image-utils) is preferred;
  `genisoimage`/`mkisofs` also work (the script names the ISO files correctly
  for them). `isoinfo` (from `genisoimage`) is used for the seed self-check when
  present — without it the check degrades to a read-only loop mount, or a
  warning if neither is possible.
- **Not supported on macOS** (no native KVM).

---

## The 5-minute happy path

```bash
# 0. (optional) sanity-check the resolved layout before touching anything
hack/scripts/baremetal-test-vms.sh doctor

# 1. Create the network + 6 VMs (3 control-plane + 3 workers), wait for SSH
#    + cloud-init on all of them (in parallel).
hack/scripts/baremetal-test-vms.sh up

# 2. Prove the environment is deploy-ready (interface name, reachability, VIP
#    free, node-to-node). Exits non-zero if anything is wrong.
hack/scripts/baremetal-test-vms.sh check

# 3. Build the CLI and point it at these nodes (see "Wiring into a cluster").
# 4. Deploy, verify, then tear down.
hack/scripts/baremetal-test-vms.sh cleanup --all -y
```

`up` is idempotent: domains that already exist are skipped, and an existing
network is reused. If a node doesn't come up, `up` tells you which and to check
`virsh console <domain>`.

---

## Subcommands

| Command | What it does |
|---|---|
| `up` | Create network + all VMs, wait for SSH **and** `cloud-init status --wait` (parallel, `OC_VM_SSH_TIMEOUT` each), print a status table. |
| `doctor` | Print the resolved layout (IPs, MACs, specs, workdir, image, seed tool, SSH keys) **without doing anything**. Run this first to catch config typos. |
| `check` / `verify` | Assert the invariants a baremetal deploy depends on: each node SSH-reachable, cloud-init done, interface named `OC_VM_ETHERNAME` (default `eth0`) holding the pinned IP, master-1 → worker-1 ping, and the VIP free. **Exits non-zero on any failure** — use it as a gate before `deploy`. |
| `reboot [node]` | Reboot all VMs, or one named node (`reboot master-1`). Uses the `--connect` URI. |
| `status` | Network + per-VM state table with resolved IPs. |
| `ssh <node>` | SSH into a node by short name (`ssh master-1`, `ssh worker-2`). |
| `print-merge` | Emit the `opencenter.infrastructure.*` overlay fields (as `cluster set` args + JSON node lists) to merge onto the init-generated config. **This is the intended path.** |
| `print-config` | Legacy YAML fragment. **Not schema-valid on its own** — kept for reference. Prefer `print-merge`. |
| `cleanup` / `down` | Remove the VMs and their disks. See flags below. |

### `cleanup` flags

- `--purge-net` — also destroy/undefine the libvirt network **and** remove the
  stale `virbr-ocbm` host bridge if it lingers.
- `--purge-cache` — also delete the cached base cloud image.
- `--all` — VMs + disks + network + cache (everything `up` created).
- `--dry-run` — list what would be removed, change nothing.
- `-y` / `--yes` — skip the confirmation prompt (for CI).

`cleanup` is **prefix-scoped** (only touches domains named `${OC_VM_PREFIX}-*`),
**idempotent**, and discovers resources from libvirt rather than a state file —
so a half-finished `up` is still fully cleaned. By default it removes the VMs
and their disk overlays but **keeps** the network and cached base image for a
fast re-`up`.

---

## Configuration (environment variables)

Every knob has a sane default; override any of them inline, e.g.
`OC_VM_COUNT=9 OC_VM_MASTERS=3 hack/scripts/baremetal-test-vms.sh up`.

| Variable | Default | Meaning |
|---|---|---|
| `OC_VM_COUNT` | `6` | Total VMs. |
| `OC_VM_MASTERS` | `3` | First N are control-plane; the rest are workers. |
| `OC_VM_VCPUS` | `2` | vCPUs per VM (used by both roles unless overridden). |
| `OC_VM_MEMORY_MB` | `4096` | RAM per VM in MiB (used by both roles unless overridden). |
| `OC_VM_DISK_GB` | `40` | Root disk per VM in GiB. |
| `OC_VM_MASTERS_VCPUS` | `OC_VM_VCPUS` | Master vCPU override. |
| `OC_VM_MASTERS_MEMORY_MB` | `OC_VM_MEMORY_MB` | Master RAM override. |
| `OC_VM_WORKERS_VCPUS` | `OC_VM_VCPUS` | Worker vCPU override. |
| `OC_VM_WORKERS_MEMORY_MB` | `OC_VM_MEMORY_MB` | Worker RAM override. |
| `OC_VM_PREFIX` | `oc-bm` | Domain/hostname prefix. |
| `OC_NET_NAME` | `oc-baremetal` | libvirt network name. |
| `OC_NET_CIDR` | `192.168.123.0/24` | Node subnet (must be /24-style). |
| `OC_NET_VIP` | `192.168.123.10` | Reserved k8s API VIP (kube-vip). |
| `OC_NET_DHCP_START` | `192.168.123.150` | DHCP/allocation pool start. |
| `OC_NET_DHCP_END` | `192.168.123.200` | DHCP/allocation pool end. |
| `OC_NODE_IP_BASE` | `101` | Last octet of the first pinned node IP. |
| `OC_IMAGE_URL` | Ubuntu 22.04 jammy cloudimg | Base cloud image URL. |
| `OC_SSH_PUBKEY` | auto-detected | Public key injected via cloud-init. |
| `OC_SSH_PRIVATE_KEY` | `${OC_SSH_PUBKEY%.pub}` | Private key used by `ssh`/`check`. |
| `OC_SSH_USER` | `ubuntu` | cloud-init login user. |
| `OC_VM_ETHERNAME` | `eth0` | In-VM interface name (forced by a udev rule in the seed). |
| `OC_VM_SSH_TIMEOUT` | `300` | Seconds to wait for each node during `up`. |
| `OC_WORKDIR` | `~/.cache/opencenter/baremetal-vms` | State/overlay/cache dir. |
| `OC_LIBVIRT_URI` | `qemu:///system` | libvirt connection URI. |

### Node IP + MAC layout

Masters get `OC_NODE_IP_BASE..+N`; workers continue after a 10-address gap.
`compute_layout` (called by every subcommand) **fails fast** if a pinned IP
falls inside the DHCP pool. MACs are deterministic (no wall clock), derived from
prefix + index.

Defaults: masters `192.168.123.101-103`, workers `192.168.123.114-116`,
gateway `192.168.123.1`, VIP `192.168.123.10`, DHCP `192.168.123.150-200`.

### SSH key auto-detection

If `OC_SSH_PUBKEY` is unset, the script picks the first readable of
`~/.ssh/id_ed25519.pub`, `~/.ssh/id_rsa.pub`, `~/.ssh/id_ecdsa.pub`.
`OC_SSH_PRIVATE_KEY` defaults to that path minus the `.pub`. Override either if
you use a non-standard key (e.g. `~/.ssh/oc-baremetal` / `oc-baremetal.pub`).

---

## Why the script does what it does (the load-bearing parts)

These behaviors exist because the first end-to-end run hit each of them as a
silent, expensive failure. Reading this before your first `up` saves the most
time.

### 1. The seed forces the interface to `eth0`

Kube-vip (and Kubespray's interface assumptions) expect `eth0`, but modern cloud
images default to predictable names like `enp1s0`. The cloud-init seed writes a
udev rule at **first boot** —

```
/etc/udev/rules.d/70-eth0.rules
SUBSYSTEM=="net", ACTION=="add", ATTR{address}=="<pinned-mac>", NAME="eth0"
```

— that renames the VM's NIC to `eth0` *before* the interface comes up. This
takes effect immediately (no post-boot reboot), which is what a GRUB
`net.ifnames=0` or a hand-added udev rule did **not** do on the first run.
Set `OC_VM_ETHERNAME` to a different name if your provider needs one.

> If you ever `check` VMs that were created with an older seed (no udev rule),
> you'll see `[FAIL] … interface is 'enp1s0' (expected eth0)`. Recreate them
> (`cleanup --all -y` then `up`) to pick up the fixed seed.

### 2. `up` waits for cloud-init, not just port 22

SSH can open while cloud-init is still mid-flight, which masks seed/hostname/
interface problems until much later. `wait_for_node` requires **both** the port
to answer and `cloud-init status --wait` to succeed, and `up` runs all nodes in
parallel with a per-node `OC_VM_SSH_TIMEOUT`.

### 3. Each seed ISO is self-checked

After building a seed, `make_seed` verifies the `cidata` volume exposes a
non-empty `user-data` (via `isoinfo`, else a read-only loop mount). A silent
mis-provision (e.g. the mkisofs basename bug where temp files are ignored) now
fails loudly at build time instead of producing a VM that never configures SSH.

---

## Wiring into a cluster deploy

The script produces reachable nodes; the **CLI** does the deploy. The clean path
is to let `cluster init` write a complete, schema-valid config and then overlay
only the VM-specific fields with `print-merge`.

```bash
# 1. Build + install the CLI (repo root).
mise run build
mise run local-install        # -> ~/.local/bin/opencenter

# 2. Init a baremetal cluster (writes a full config + generates SSH/SOPS keys).
~/.local/bin/opencenter cluster init oc-baremetal --type baremetal --org opencenter

# 3. Overlay the VM-specific fields (from the script).
~/.local/bin/opencenter cluster set opencenter/oc-baremetal \
    --set "$(hack/scripts/baremetal-test-vms.sh print-merge \
            | grep -E '^opencenter\.' | tr '\n' ' ')"
```

`print-merge` emits one line per field, e.g.:

```
opencenter.infrastructure.networking.subnet_nodes=192.168.123.0/24
opencenter.infrastructure.networking.vrrp_ip=192.168.123.10
opencenter.infrastructure.networking.vip_interface=eth0
opencenter.infrastructure.ssh.user=ubuntu
opencenter.infrastructure.ssh.key_path=/home/opencenter/.ssh/oc-baremetal
opencenter.infrastructure.compute.master_nodes=[{"name":"oc-bm-master-1",...},...]
opencenter.infrastructure.compute.worker_nodes=[{"name":"oc-bm-worker-1",...},...]
```

**Two fields need a manual YAML edit** (the `set` command can't address them):

- `opencenter.infrastructure.ssh.authorized_keys[0]` — `set` can't target a
  `[]string` by index. Edit the config YAML and paste in
  `$(cat "$OC_SSH_PUBKEY")`. `print-merge` prints the exact line as a comment.
- If `init` enabled services that need S3 (loki/tempo), point them at a local
  RustFS or set their `storage_type=none` (see the design doc's Findings).

Then validate and deploy:

```bash
~/.local/bin/opencenter cluster validate opencenter/oc-baremetal --validation offline
~/.local/bin/opencenter cluster deploy  opencenter/oc-baremetal
```

The design doc's *Findings & Workarounds* section lists the additional local
setup that a fully-offline deploy needs (local Gitea for GitOps, RustFS for S3,
the kubespray module contract outputs, in-cluster Gitea CA). Those are
**environment** concerns, not `baremetal-test-vms.sh` concerns.

### Verifying the cluster

```bash
export KUBECONFIG="$HOME/.config/opencenter/clusters/state/opencenter/oc-baremetal/kubeconfig.yaml"
kubectl get nodes                       # all Ready
kubectl get pods -A | grep -v Running   # no stuck system pods
kubectl run roundtrip --image=nginx --rm -it --restart=Never -- sleep 600
kubectl expose pod roundtrip --port=80
kubectl port-forward svc/roundtrip 8080:80 &
curl -s localhost:8080 | grep -o '<title>.*</title>'   # -> Welcome to nginx!
kubectl delete ns default --ignore-not-found; kubectl delete pod roundtrip --ignore-not-found
```

---

## Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| `up` warns a node didn't come up | `virsh console <domain>` — usually the seed didn't run (see `check` + Finding 3). |
| `check`: `interface is 'enp1s0'` | VMs built with an old seed (no udev rule). `cleanup --all -y` then `up`. |
| `check`: `ssh unreachable` on all | Wrong key or `OC_SSH_USER`. Confirm `OC_SSH_PUBKEY`/`OC_SSH_PRIVATE_KEY` match what you injected; `doctor` shows both. |
| `cannot reach libvirt` | Not in `libvirt` group (re-login / `sg libvirt -c '…'`) or libvirtd down. |
| `pinned IP … inside the DHCP pool` | Adjust `OC_NODE_IP_BASE` or the `OC_NET_DHCP_*` pool so the pinned range is outside it. |
| `seed … has no 'user-data'` | Seed-ISO tool mismatch. Install `cloud-image-utils` or `genisoimage`. |
| VIP already held during `check` | A previous cluster is still running (kube-vip bound it). Expected on a live cluster; clean up first if you want it free. |
| Deploy fails at `upload-config` / API not reachable | kube-vip didn't bind — confirm `vip_interface` matches the real NIC (`check`), and that the kubespray module suppresses `loadbalancer_apiserver` for kube-vip clusters (design doc Finding 3). |

### Useful one-liners

```bash
# Watch the resolved layout (no changes):
hack/scripts/baremetal-test-vms.sh doctor

# Re-check after any change (reboot, manual fix):
hack/scripts/baremetal-test-vms.sh check

# Shell in and look at the interface directly:
hack/scripts/baremetal-test-vms.sh ssh master-1 -- ip -br addr
```

---

## Teardown

```bash
# Remove VMs + disks, keep network + cached base image (fast re-up):
hack/scripts/baremetal-test-vms.sh cleanup -y

# Remove everything (VMs + disks + network + cache):
hack/scripts/baremetal-test-vms.sh cleanup --all -y
```

Optionally also run `opencenter cluster destroy opencenter/oc-baremetal` to
remove the CLI-side cluster state.
