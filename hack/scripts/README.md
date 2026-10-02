# `hack/scripts`

Small repository-maintenance and test-support utilities. Run commands from the
repository root unless noted otherwise.

## Scripts

- `audit_doc_frontmatter.py` audits maintained pages under `docs/` and can
  update `last_updated` on explicitly supplied maintained pages.
- `add_purpose_line.py` adds the required purpose line to maintained docs; it
  skips generated command references.
- `check_staged_docs.py` checks whether implementation changes include a
  maintained documentation change.
- `test_check_staged_docs.py` tests the staged-documentation checker.
- `check_doc_links.py` checks local Markdown link paths without network access;
  external URLs and anchors are skipped.
- `test_check_doc_links.py` tests the local-link checker.
- `check_mermaid_fences.py` checks Mermaid fence structure without validating
  Mermaid diagram syntax.
- `test_check_mermaid_fences.py` tests the Mermaid fence checker.
- `openstack-reset.sh` removes OpenStack resources used by lifecycle tests.
- `baremetal-test-vms.sh` provisions disposable KVM/libvirt VMs (default 3
  control-plane + 3 workers) on a dedicated NAT network to test the baremetal
  `cluster deploy`, and tears them down again. Linux libvirt host only.
- `tf2yaml.py` converts the supported Terraform locals/module subset used by
  GitOps helpers.

Examples:

```bash
python3 hack/scripts/audit_doc_frontmatter.py
python3 hack/scripts/add_purpose_line.py
python3 hack/scripts/check_staged_docs.py cmd/cluster_generate.go README.md
python3 hack/scripts/check_doc_links.py --repo-root .
python3 hack/scripts/check_mermaid_fences.py --repo-root .
```

The documentation-audit scripts intentionally operate on `docs/`; repository
root and source-adjacent Markdown files are not maintained site pages.

## Baremetal test VMs (`baremetal-test-vms.sh`)

Spins up disposable Ubuntu 22.04 KVM VMs that act as pre-provisioned baremetal
nodes, so you can exercise `opencenter cluster deploy` end to end on a single
Linux libvirt host. The VMs are plain SSH-reachable hosts with your public key
and passwordless sudo; the deploy then SSHes in and runs kubespray.

**Full guide** (subcommands, every env var, how to wire into a cluster deploy,
troubleshooting, teardown): [`baremetal-test-vms.md`](./baremetal-test-vms.md).

Requires a Linux host with `libvirt-daemon-system`, `virtinst`, `qemu-utils`,
`cloud-image-utils` (or `genisoimage`), and `curl`. It does **not** run on
macOS (no native KVM).

Typical loop:

```bash
# 0. Sanity-check the resolved layout before touching anything.
hack/scripts/baremetal-test-vms.sh doctor

# 1. Create the network + 6 VMs (3 control-plane, 3 workers) and wait for SSH
#    + cloud-init (in parallel).
hack/scripts/baremetal-test-vms.sh up

# 2. Prove the environment is deploy-ready (interface name, reachability, VIP).
#    Exits non-zero if anything is wrong.
hack/scripts/baremetal-test-vms.sh check

# 3. Overlay the VM-specific fields onto an init-generated cluster config,
#    then deploy (see the guide for the exact CLI commands).
hack/scripts/baremetal-test-vms.sh print-merge

# 4. Inspect state / shell into a node.
hack/scripts/baremetal-test-vms.sh status
hack/scripts/baremetal-test-vms.sh ssh master-1

# 5. Tear everything down when finished.
hack/scripts/baremetal-test-vms.sh cleanup --all -y
```

Resources are tunable with environment variables (full list in the guide), e.g.:

```bash
OC_VM_COUNT=6 OC_VM_MASTERS=3 \
OC_VM_VCPUS=4 OC_VM_MEMORY_MB=8192 OC_VM_DISK_GB=60 \
  hack/scripts/baremetal-test-vms.sh up

# Masters and workers can differ:
OC_VM_MASTERS_MEMORY_MB=4096 OC_VM_WORKERS_MEMORY_MB=8192 \
  hack/scripts/baremetal-test-vms.sh up
```

`cleanup` is prefix-scoped (only touches domains named `${OC_VM_PREFIX}-*`),
idempotent, and discovers resources from libvirt rather than a state file, so a
half-finished `up` is still fully cleaned. Flags:

- `--purge-net` also removes the libvirt network and the stale `virbr-ocbm` bridge.
- `--purge-cache` also deletes the cached base cloud image.
- `--all` removes everything `up` created (VMs + disks + network + cache).
- `--dry-run` lists what would be removed and changes nothing.
- `-y` / `--yes` skips the confirmation prompt (for CI).

By default `cleanup` removes the VMs and their disk overlays but keeps the
network and cached base image for a fast re-`up`.
