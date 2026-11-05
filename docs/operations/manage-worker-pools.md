---
id: manage-worker-pools
title: Manage worker pools
sidebar_label: Manage worker pools
description: List, update, and scale worker pool definitions in an openCenter configuration.
doc_type: how-to
audience: openCenter operators
tags: [worker-pools, scaling]
last_updated: 2026-10-20
---
# Manage worker pools

**Purpose:** For openCenter operators, shows how to list, update, and scale worker pool definitions in an openCenter configuration.

The `cluster pool` commands update worker-pool definitions in the selected
configuration. They do not directly change provider infrastructure.

```bash
opencenter cluster pool list --cluster ORG/CLUSTER
opencenter cluster pool list --cluster ORG/CLUSTER --output json
opencenter cluster pool list --cluster ORG/CLUSTER --output yaml
```

## Update and scale

```bash
opencenter cluster pool update batch --cluster ORG/CLUSTER \
  --count 3 --flavor NEW_FLAVOR --image IMAGE

opencenter cluster pool update gpu-wg1 --cluster ORG/CLUSTER \
  --label workload=gpu --label gpu=true \
  --taint nvidia.com/gpu=present:NoSchedule

opencenter cluster pool scale batch --cluster ORG/CLUSTER --count 0
```

`update` accepts count, flavor, image, boot-volume size, boot-volume type,
labels, and taints. `--label`/`--taint` are repeatable and **replace** the
pool's existing labels/taints wholesale when provided; omitting them leaves
those fields untouched. Pass an empty value (`--label=` / `--taint=`) to clear.
Taint format is `key=value:effect` with effect one of `NoSchedule`,
`PreferNoSchedule`, `NoExecute`. Linux pools always end up with a complete
boot-volume block: unset fields are filled with the platform defaults (256 GB
`HA-Performance`, destination `volume`, source `image`, delete on termination)
on add and on the first update after they were introduced. `scale` changes
only the target count. Both commands support global `--dry-run`.

## Remove

The implementation blocks removal while a pool has a positive count unless
`--force` is supplied. The safe configuration sequence is:

```bash
opencenter cluster pool scale batch --cluster ORG/CLUSTER --count 0
opencenter cluster generate ORG/CLUSTER
opencenter cluster deploy ORG/CLUSTER
opencenter cluster pool remove batch --cluster ORG/CLUSTER
```

`pool remove --force` bypasses the count check; it does not perform provider
cleanup for you.

## Evidence

- Command behavior and flags: `cmd/cluster_pool.go`
- Pool command tests: `cmd/cluster_pool_test.go`
