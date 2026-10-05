---
last_updated: 2026-10-02
id: destroy-openstack-cluster
title: Destroy an OpenStack cluster
sidebar_label: Destroy an OpenStack cluster
description: How to safely tear down an OpenStack cluster and clean up orphaned CSI Cinder volumes to avoid quota exhaustion.
doc_type: how-to
audience: "platform engineers"
tags: [operations, openstack, destroy, storage]
---

# Destroy an OpenStack cluster

This guide covers tearing down an OpenStack cluster with `opencenter cluster destroy`,
with particular attention to Cinder volumes provisioned by the CSI driver that are
**not** removed by OpenTofu and must be cleaned up separately to avoid quota exhaustion.

## Background: orphaned CSI volumes

When Kubernetes workloads create PersistentVolumeClaims, the OpenStack Cinder CSI
driver (`cinder.csi.openstack.org`) dynamically provisions Cinder volumes for them.
These volumes are managed by Kubernetes, not by OpenTofu, so they are invisible to
`tofu destroy` and survive cluster deletion.

Once the cluster (and its CSI controller) are gone, the `reclaimPolicy: Delete` on
those PersistentVolumes can never be honoured. The volumes remain in `available` state
and continue consuming the project's `gigabytes` quota.

**Symptoms of orphaned volumes exhausting quota:**

```
413 overLimit: VolumeSizeExceedsAvailableQuota: Requested 10G, quota is 10000G
and 10000G has been consumed.
```

Downstream impact: stateful services (Loki, Mimir, Harbor, Prometheus, Keycloak) will
have PVCs stuck in `Pending` with "unbound immediate PersistentVolumeClaims", and Flux
kustomizations never reach `Ready`. This is easy to misdiagnose as a node resource
constraint; it is actually a storage quota issue.

## Destroy with automatic volume cleanup (recommended)

The `--delete-volumes` flag queries the cluster for CSI PV volume handles **before**
infrastructure teardown, then deletes those exact Cinder volumes **after** teardown.

```bash
opencenter cluster destroy <org>/<cluster> --force --delete-volumes
```

What happens step by step:

1. **capture-csi-volumes** — `kubectl get pv -o json` enumerates PersistentVolumes
   backed by `cinder.csi.openstack.org` and records their Cinder volume IDs. This runs
   while the cluster API is still reachable, before any infrastructure is torn down.
2. **opentofu-init** — initialises the OpenTofu working directory.
3. **opentofu-destroy** — tears down compute, network, and storage infrastructure.
4. **cleanup-csi-volumes** — deletes the captured Cinder volume IDs via the Cinder API.
   Only volumes in `available` state are deleted; `in-use` or `reserved` volumes are
   skipped (they belong to a live cluster and are never touched).

## Destroy with volume report (default)

Without `--delete-volumes`, orphaned volumes are **reported** at the end of the destroy
rather than deleted. The output includes the volume IDs, their sizes, and a ready-to-run
`openstack volume delete` command.

```bash
opencenter cluster destroy <org>/<cluster> --force
```

Example output at the end of destroy:

```
Warning: 3 orphaned CSI-provisioned Cinder volume(s) detected (80 GB total).
These volumes are consuming quota but are no longer in use by any cluster.
Re-run with --delete-volumes to remove them automatically, or delete manually:

  openstack volume delete <id1> <id2> <id3>
```

## Check quota before and after

```bash
# Show current volume quota usage
openstack --os-cloud <cloud> quota show --usage | grep -iE "gigabytes|volumes"

# List volumes in 'available' state (all unattached volumes in the project)
openstack --os-cloud <cloud> volume list --status available
```

## Manual cleanup (when the cluster is already gone)

If the cluster was destroyed without `--delete-volumes` and you need to clean up
existing orphans:

```bash
# 1. Identify pvc-* volumes in available state
openstack --os-cloud <cloud> volume list --status available \
  --format value -c ID -c Name | grep '^pvc-'

# 2. Delete them (replace IDs with actual values)
openstack --os-cloud <cloud> volume delete <id1> <id2> ...
```

> **Note:** `openstack volume list` output may omit `created_at` in some CLI builds.
> Use `openstack volume show <id>` per volume if you need creation timestamps.

## Also remove local files

After infrastructure teardown, local GitOps and config files are preserved by default.
Add `--remove-files` to delete them as well:

```bash
opencenter cluster destroy <org>/<cluster> --force --delete-volumes --remove-files
```

## Safety guarantees

- `--delete-volumes` only deletes volumes whose IDs were captured from the cluster's own
  PersistentVolumes **before** destroy. Volumes belonging to other clusters are never
  touched.
- Volumes in `in-use`, `reserved`, or any state other than `available` are always
  skipped, even if their IDs appear in the captured list.
- Capture step is non-fatal. If the cluster API is already unreachable, the capture
  warns and continues with zero handles (cleanup becomes a no-op).
- **In `--delete-volumes` mode:** deletion failures are reported but do NOT fail the
  overall destroy command (exit 0). Always check the cleanup output to verify volumes
  were actually deleted. If cleanup errors appear, re-run the report step or manually
  delete remaining volumes.
- **In report-only mode (default):** failures are logged as warnings and do not affect
  the destroy result.
