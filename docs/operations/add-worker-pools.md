---
id: add-worker-pools
title: Add a worker pool
sidebar_label: Add a worker pool
description: Add a worker pool definition to an openCenter cluster configuration.
doc_type: how-to
audience: openCenter operators
tags: [worker-pools, configuration]
last_updated: 2026-10-21
---
# Add a worker pool

Worker pools are configuration entries. The pool command writes the v2
configuration; generation and deployment are separate operations.

## Add a pool

```bash
opencenter cluster pool add batch \
  --cluster ORG/CLUSTER \
  --count 2 \
  --flavor WORKER_FLAVOR
```

Supported add flags include `--image`, `--os linux|windows`,
`--boot-volume-size`, `--boot-volume-type`, repeatable `--label key=value`,
repeatable `--taint key=value:effect`, and `--pf9-onboard`. `--flavor` is
required. Linux pools default their boot volume to 256 GB `HA-Performance`
(destination `volume`, source `image`, delete on termination) when those
fields are not passed. `cluster pool update` accepts the same `--label`/
`--taint` flags; there they replace the pool's existing labels/taints
wholesale -- see [Manage worker pools](manage-worker-pools.md).

The command stores Linux pools under
`opencenter.infrastructure.compute.additional_server_pools_worker` and Windows
pools under the corresponding `_windows` collection. These names are the
structural fields used by the implementation; provider-specific flavor, image,
and volume values are not invented by openCenter.

## Validate and apply the configuration

```bash
opencenter cluster validate ORG/CLUSTER
opencenter cluster generate ORG/CLUSTER
opencenter cluster deploy ORG/CLUSTER
```

Use global `--dry-run` on the mutating command when you want a preview.

## Evidence and limits

The command edits configuration and prints a reminder to generate. It does not
by itself prove that a provider has capacity or that a node joined Kubernetes.
Those checks require the provider lifecycle and a live cluster.

Generation wires the pool into the cluster: the rendered
`infrastructure/clusters/<cluster>/main.tf` passes
`additional_server_pools_worker` to the OpenStack Nova module and concatenates
the resulting `additional_worker_pools_nodes` into the Kubespray module's
`worker_nodes`, so pool nodes land in the Ansible inventory group
`oc_worker_nodes` (child of `kube_node`). Windows pools concatenate into
`windows_nodes` the same way. Additional pools are an OpenStack feature: the
baremetal and VMware templates do not render them.

- Worker-pool command: `cmd/cluster_pool.go`
- Pool fields: `internal/config/v2/infrastructure.go`
- Pool command tests: `cmd/cluster_pool_test.go`
