# Design: End-to-end baremetal cluster deploy on a local libvirt host

Date: 2026-10-01
Status: Approved
Repo: openCenter-cli

## Goal

Turn a single Linux libvirt host into a working baremetal Kubernetes cluster
using the repo's own tooling, validating every step and proving the cluster is
healthy at the end. The workflow is the six steps the user specified:

1. Build the VMs
2. Build the CLI
3. Create a new cluster using type baremetal
4. Set any settings using the built CLI
5. Deploy the cluster via the CLI
6. Validate a fully working k8s cluster

## Host

- Fedora, 12 cores / 62 GB RAM / ~123 GB free disk, `/dev/kvm` present.
- Running as user `opencenter` (uid 1002), in the `wheel` group, **passwordless
  sudo**.
- `libvirtd` was **inactive** and the user was **not** in the `libvirt` group;
  both were fixed (idempotently) as step 0. After fix: `libvirtd` active,
  `opencenter` in `libvirt`, libvirt reachable at `qemu:///system`.
- The `opencenter` group membership only applies to *new* sessions, so the VM
  script is run under `newgrp libvirt` (the wrapper bakes this in).

## VM topology

- 3 control-plane (masters) + 3 workers.
- Masters: 4 vCPU / 4 GB. Workers: 4 vCPU / 8 GB. (User choice.)
- Total ~28 GB RAM — comfortable on a 62 GB host.
- Provisioned by `hack/scripts/baremetal-test-vms.sh up` on a dedicated libvirt
  NAT network `192.168.123.0/24`. Node IPs are pinned by MAC:
  - masters: `192.168.123.101..103`
  - workers: `192.168.123.111..113`
  - gateway `192.168.123.1`, VIP `192.168.123.10`, DHCP pool `192.168.123.150..200`.
- VMs are Ubuntu 22.04 cloud images with the host's SSH key and passwordless
  sudo — exactly what the baremetal provider needs (it SSHes in and runs
  kubespray).

## CLI

- Go, module `github.com/opencenter-cloud/opencenter-cli`, toolchain Go 1.27.1
  via mise.
- Built with `mise run build` (→ `bin/opencenter`) and installed with
  `mise run local-install` (→ `~/.local/bin/opencenter` + local plugin). The
  preinstalled copy was stale (Sep 10); a fresh build `0.0.1-8174b69` was used.

## Baremetal config model (evidence-backed)

- `cluster init <name> --type baremetal` writes a complete config to
  `~/.config/opencenter/clusters/blueprints/<org>/<name>/<name>-config.yaml`.
  It pre-populates `storage`, `networking.dns_zone_name`/`dns_nameservers`/
  `ntp_servers`, `cluster.kubernetes.subnet_pods`/`subnet_services`/`version`,
  `cloud: {}`, `deployment.method: kubespray`, `opentofu.backend.type: local`,
  and auto-generates SSH + SOPS/Age keys.
- What `init` does NOT populate for baremetal: `compute.master_nodes` /
  `worker_nodes` (empty slices) and the real network node IPs (still the
  10.2.128.0/22 default). `os_version` is `"24"` by default — our VMs are 22.04.
- The baremetal deploy template
  (`internal/gitops/templates/infrastructure-cluster-template/main-baremetal.tf.tpl`)
  reads node lists, ssh user/key, subnet_nodes, vrrp_ip, kube_vip settings,
  and `dns_zone_name`. It does NOT read `storage.*` (validation-only for
  baremetal), `dns_nameservers`, or `ntp_servers`.
- `storage` and `cloud` are `validate:"required"` struct fields; `cloud: {}`
  satisfies it. `storage` needs its 5 required sub-fields (already present from
  init).

## The two real defects that block a clean validate/deploy

1. **`print-config` in `baremetal-test-vms.sh` emits a fragment** (no `storage`,
   no `dns_zone_name`/`dns_nameservers`/`ntp_servers`, no `cluster.kubernetes`)
   that cannot pass `cluster validate`. Decision: rather than "fixing" the
   script, the wrapper **edits the init-generated config** (the complete,
   schema-valid shape) with the real VM values. `print-config` is kept only as
   a node-list helper.
2. **init enables a heavy service stack** (keycloak, postgres-operator, olm,
   loki, tempo, kube-prometheus-stack, kyverno, headlamp, gateway, gateway-api,
   rbac-manager, fluxcd, sources, cert-manager, calico) plus an HTTPS GitOps
   token requirement (`gitops.auth.token.token = CHANGEME`). On a local VM
   cluster:
   - loki + tempo resolve to the S3 backend by default and would fail at
     runtime with no S3 → set their storage to local/none.
   - the HTTPS GitOps token requirement is cleared by switching GitOps auth to
     **SSH** (init already generates the SSH keys) with a placeholder
     `ssh://` URL.
   - The Flux bootstrap deploy step is skipped when the GitOps URL is the
     default placeholder, so a local cluster never clones/pushes a remote.

## "Keep more defaults" decision (user)

Rather than disabling everything, keep the identity trio (keycloak +
postgres-operator + olm), the monitoring stack (kube-prometheus-stack, loki,
tempo), kyverno, headlamp, gateway/gateway-api, rbac-manager, fluxcd, sources,
cert-manager, calico — and make the S3-dependent ones work locally (loki/tempo
storage_type none). This keeps the platform realistic while still being fully
self-contained on the VMs.

## Step 4 — the exact config reconciliation (wrapper `set-cluster`)

Applied to the init-generated config for `oc-baremetal` (org `opencenter`):

- `opencenter.infrastructure.os_version=22.04`
- `opencenter.infrastructure.networking.subnet_nodes=192.168.123.0/24`
- `opencenter.infrastructure.networking.allocation_pool_start=192.168.123.150`
- `opencenter.infrastructure.networking.allocation_pool_end=192.168.123.200`
- `opencenter.infrastructure.networking.gateway=192.168.123.1`
- `opencenter.infrastructure.networking.vrrp_ip=192.168.123.10`
- `opencenter.infrastructure.networking.vip_interface=eth0`
- `opencenter.infrastructure.networking.loadbalancer_provider=metallb`
- `opencenter.infrastructure.ssh.user`/`username`=ubuntu,
  `key_path`=<generated private key>, `authorized_keys[0]`=<host public key>
- `opencenter.infrastructure.compute.master_nodes=[{name,access_ip_v4} x3]`
  (set as a whole JSON list — `set` supports slice-of-struct JSON; per-index
  does not work on the empty init slices)
- `opencenter.infrastructure.compute.worker_nodes=[{name,access_ip_v4} x3]`
- GitOps → SSH auth: `gitops.repository.url=ssh://git@local/oc/cluster-config.git`,
  `gitops.auth.token=null`, `gitops.auth.ssh.private_key`/`public_key`=<init keys>
- loki + tempo → local storage: `services.loki.storage_type=none`,
  `services.tempo.storage_type=none` (clears the S3 secret/endpoint errors)
- keycloak admin password → set to a generated non-placeholder value (or
  disable keycloak+postgres+olm if keeping them is too heavy; decision: set a
  value so the stack stays on).

Then `opencenter cluster validate <org>/<name> --validation offline` must exit 0
with no errors.

## Step 5 — deploy

`opencenter cluster deploy <org>/<name>`. Steps: preflight (static-node
check) → opentofu-init/apply (local render) → kubespray-prepare (clones public
`kubernetes-sigs/kubespray` v2.31.0, python venv) → wait-cloudinit →
kubespray-deploy (`ansible-playbook cluster.yml -f 10`) → export-kubeconfig →
normalize → network-plugin install. Resumable if a step fails (re-run
`deploy`). This is the long pole (~30–60 min).

## Step 6 — verification

- `kubectl get nodes` → 3 masters + 3 workers, all `Ready`.
- `kubectl get pods -A` → core system pods (kube-apiserver, etcd,
  scheduler, controller-manager, kubelet, calico, coredns) `Running`.
- `kubectl get svc` → API reachable via the kube-vip (`192.168.123.10`).
- Pod round-trip: `kubectl run` a busybox/nanosleep pod, confirm it schedules
  on a worker and runs.

## Teardown

- `hack/scripts/baremetal-test-vms.sh cleanup --all -y`
- (optionally) `opencenter cluster destroy <org>/<name>`

## Deliverables

- `hack/scripts/baremetal-test-vms.sh` — VMs + node-list helper (unchanged logic).
- `hack/scripts/baremetal-deploy.sh` — thin reusable wrapper (steps 0–6),
  `set -euo pipefail`, driven by the same `OC_*` env vars; runs under
  `newgrp libvirt`; logs each step; resumable.
- This spec doc.
- Repo stays otherwise clean; the cluster config lives in
  `~/.config/opencenter/clusters/...`, not the repo.

## `baremetal-test-vms.sh` enhancements (2026-10-02)

After the first end-to-end run, the script was hardened so the three biggest
time sinks (interface naming, silent cloud-init, silent seed-ISO) fail fast and
loudly instead of 30 minutes into kubespray. New subcommands and behavior:

| Subcommand | Purpose |
|---|---|
| `doctor` | Print the resolved layout (IPs, MACs, workdir, image, seed tool, SSH keys) before any action. Cheap way to catch config typos. |
| `check` / `verify` | Assert deploy preconditions: SSH reachable, `cloud-init status --wait` done, interface named `eth0` (or `OC_VM_ETHERNAME`) with the pinned IP, master-1 → worker-1 ping, VIP free. Exits non-zero on any failure. |
| `reboot [node]` | Reboot all VMs (or one named node) via the `--connect` URI. |
| `print-merge` | Emit the `opencenter.infrastructure.*` overlay fields (as `cluster set` args + JSON node lists) to merge onto the init-generated config. The intended path. |
| `print-config` | Legacy YAML fragment (kept for reference; now clearly marked "NOT schema-valid"). |

Behavior changes:
- **Interface naming:** the cloud-init seed now writes a udev rule
  (`/etc/udev/rules.d/70-eth0.rules`) at first boot that names the VM's NIC
  `eth0` (matched by its pinned MAC). This keeps kube-vip / kubespray
  interface assumptions true without a post-boot reboot. Configurable via
  `OC_VM_ETHERNAME` (default `eth0`).
- **Readiness probe:** `up` now waits for SSH **and** `cloud-init status --wait`
  to succeed (not just port 22 open), in parallel across all nodes, with a
  configurable `OC_VM_SSH_TIMEOUT` (default 300 s).
- **Seed self-check:** after building each seed ISO, `make_seed` verifies it
  contains a non-empty `user-data` (via `isoinfo` when available, else a read-only
  loop mount). A silent mis-provision (the mkisofs basename bug) now fails
  loudly at build time. Degrades to a warning when neither tool is available.
- **SSH key auto-detect:** `OC_SSH_PUBKEY` defaults to the first readable of
  `~/.ssh/id_ed25519.pub`, `id_rsa.pub`, `id_ecdsa.pub` (most hosts generate
  ed25519 now). `OC_SSH_PRIVATE_KEY` defaults to the pub path minus `.pub`.
- **Cleanup:** `--purge-net` also removes the stale `virbr-ocbm` host bridge if
  it lingers after the network is gone (a stale bridge can confuse a fresh `up`).

## What we intentionally do NOT do

- Change k8s/kubespray versions from init defaults (k8s 1.35.4, kubespray
  2.31.0).
- Alter anything outside `hack/scripts/` in the repo (the wrapper + this doc).
- Wire up external S3/GitHub — the cluster is fully local.

## Findings & Workarounds (as-built, 2026-10-02)

The deploy succeeded end-to-end (6 nodes Ready, Calico up, Flux reconciling from
local Gitea, app round-trip verified via Service). Several **upstream defects**
had to be worked around; these are load-bearing for any repeat.

### 1. Local Gitea + RustFS replace the SSH/none plan
- `opencenter-local gitea up --runtime podman` → self-hosted Gitea on
  `192.168.123.1:3000-3001` (0.0.0.0). `opencenter-local rustfs up` → S3 on
  `127.0.0.1:9000` (had to be rebound to `0.0.0.0` manually for VM reachability
  at `192.168.123.1:9000`).
- GitOps uses HTTPS + `gitops.auth.token.provider: gitea` (token file
  `~/.config/opencenter/local/tokens/gitea-user.token`). loki + tempo use
  `object_storage_provider: external-s3` pointed at RustFS (creds in
  `~/.config/opencenter/local/rustfs/credentials.json`).
- Gitea must be **rootful podman** via the shim `~/.local/bin/podman`
  (`exec sudo podman "$@"`); run the plugin with that on PATH.

### 2. Kube-vip hardcodes `eth0`; VMs have `enp1s0`
- Ubuntu 24.04 (noble) uses predictable names; the kube-vip static pod
  (kubespray) hardcodes `vip_interface: eth0` → CrashLoopBackOff
  ("Link not found").
- GRUB `net.ifnames=0` did NOT propagate (update-grub not re-read after reboot).
  A udev MAC→eth0 rule also did not take without a real reboot cycle.
- **As-built fix:** patched `/etc/kubernetes/manifests/kube-vip.yml` on all 3
  masters to `vip_interface: enp1s0`. Once fixed, kube-vip bound the VIP
  `192.168.123.10/32` on master-1 and the host reached the API via the VIP.

### 3. Kubespray module: no `loadbalancer_apiserver` for kube-vip + missing CLI contract outputs
- The inventory template emitted `loadbalancer_apiserver.address: 192.168.123.10`
  (the VIP) even for kube-vip clusters, making `kubeadm init` target the VIP
  pre-kube-vip → `upload-config` failed ("context deadline exceeded").
- **Fix (patched module copy at `~/.local/share/opencenter/kubespray-module`):**
  - Suppress `loadbalancer_apiserver` when `use_octavia != true` (guard uses only
    vars passed to `templatefile`; do NOT reference `kube_vip_enabled` in the
    template — it isn't in the vars map; we added it to the map too).
  - The published base module has **zero outputs**, but the CLI's "cli" lifecycle
    mode requires `lifecycle_contract_version=="1"`, `inventory_path`,
    `k8s_api_address`, `k8s_api_port`. Added these 4 outputs (inventory_path uses
    `abspath(...)`).
  - `null_resource.wait_cloudinit` + `null_resource.copy_and_update_kubeconfig`
    needed `count = var.deploy_cluster ? 1 : 0` (they ran in cli mode with no
    ansible venv → infinite "command not found" loop).
- Config points `deployment.kubespray.kubespray_cluster.source` +
  `deployment.kubespray.modules.kubespray.source` at the patched module.

### 4. Flux `bootstrap gitea` — CLI bug (fixed in-repo) + flux IP-panic
- **CLI bug (fixed):** `internal/cluster/openstack_flux_bootstrap.go` did not
  pass `--hostname` to `flux bootstrap gitea`, so it defaulted to `gitea.com`.
  Added `fluxBootstrapParams.Hostname` derived from the repo URL and threaded it
  into both the run and the plan-commands path via a shared `fluxBootstrapArgs`
  helper. Tests added.
- **Flux limitation:** `flux bootstrap gitea` **panics** (nil-deref in
  `go-git-providers.GetDomainURL`) when `--hostname` is a bare IP:port
  (`192.168.123.1:3001`). **Workaround:** use a resolvable hostname. Added
  `gitea.oc-baremetal → 192.168.123.1` to `/etc/hosts` (host) + a CoreDNS `hosts`
  stub zone (in-cluster) + the VMs' `/etc/hosts`, and pointed the repo URL at
  `https://gitea.oc-baremetal:3001/...`.

### 5. Local Gitea TLS — self-signed cert not trusted in-cluster
- Gitea serves HTTPS with a cert SANned only for `127.0.0.1`/`localhost`.
  Regenerated a self-signed cert (SANs: localhost, gitea, gitea.oc-baremetal,
  127.0.0.1, ::1, 192.168.123.1) and installed it as `cert.pem`/`key.pem`/`ca.pem`
  (certs dir is `devx:devx`-owned → needed `sudo cp`).
- Host trust: added `ca.pem` to `/etc/pki/ca-trust/source/anchors/` +
  `update-ca-trust` (the `flux bootstrap` on the host then validated).
- **In-cluster (the hard part):** flux source-controller v1.7.1 has **no
  `certSecret` field** on `GitRepository` and its Go TLS does not honor
  `SSL_CERT_FILE` by default — but a test pod proved the image's Go **does**
  honor `SSL_CERT_FILE`. The reliable fix is a kustomize **patch** in the git
  repo (`clusters/oc-baremetal/flux-system/patches/*-ca.yaml`) that adds an
  initContainer (appends the Gitea CA to the bundle, `SSL_CERT_FILE=/ca-bundle/ca.crt`)
  to source-controller + kustomize-controller, with the CA as a **ConfigMap**
  (a Secret `stringData` trips the repo's SOPS plaintext-secret hook).
- **Bootstrap deadlock:** flux self-reconciles the controller deployments from
  the repo, reverting manual patches. To break the deadlock, patch the
  source-controller in-cluster first (GitRepository→ready), then the repo-based
  patch takes over durably.

### 6. SOPS age key
- The `sops-age` Secret in `flux-system` is required to decrypt SOPS-encrypted
  service secrets. Key at
  `~/.config/opencenter/clusters/secrets/opencenter/oc-baremetal/age/keys/oc-baremetal-key.txt`
  (referenced by `secrets.sops_age_key_file`). Created with
  `kubectl create secret generic sops-age --from-file=age.agekey=<key>`.

### 7. Operational notes
- Clear `~/.local/state/opencenter/locks/opencenter/oc-baremetal.lock` before each
  deploy; `--from-step <id>` to resume, `--restart` to force fresh `tofu apply`.
- The gitops repo is on branch `master` locally but the config targets `main`;
  push `master:main` to the Gitea remote. The remote URL in the local clone must
  be credential-free (the CLI injects the token at push time) or preflight
  rejects the mismatch.
- `opencenter cluster set` uses top-level `deployment.*` (not
  `opencenter.deployment.*`); `ssh.authorized_keys[0]` must be edited in YAML
  (`set` can't address `[]string` by index).
