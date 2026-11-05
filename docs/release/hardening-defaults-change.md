---
id: hardening-defaults-change
title: Kubernetes and OS hardening now default to enabled
sidebar_label: Hardening defaults changed
description: What changes for existing clusters when k8s_hardening and os_hardening start defaulting to true, and how to opt out.
doc_type: reference
audience: openCenter operators
tags: [migration, security, kubespray]
last_updated: 2026-10-23
---

# Hardening defaults changed

**Purpose:** For openCenter operators, documents what changes for existing clusters when k8s_hardening and os_hardening start defaulting to true, and how to opt out.

`opencenter.cluster.kubernetes.security.k8s_hardening` and
`opencenter.infrastructure.networking.security.os_hardening` now default to
`true` in generated configurations. Configurations created before this change
that never mentioned either field are affected on the next render.

## What changes

- `cluster init` writes both fields as `true`, so a newly generated
  configuration carries them explicitly rather than inheriting an implicit
  `false`.
- The rendered Terraform emits `k8s_hardening_enabled = true` and
  `os_hardening_enabled = true`, and the per-pool values
  (`k8s_hardening_enabled = local.k8s_hardening_enabled`,
  `os_hardening_enabled = local.os_hardening_enabled`) follow.
- For clusters deployed with `deployment.method: kubespray`, the bootstrap plan
  gains a `kubespray-os-hardening` step between `kubespray-wait-cloudinit` and
  `kubespray-deploy`. It is scheduled by `os_hardening` alone; `k8s_hardening`
  affects only the Terraform variables. Providers that do not use Kubespray (for
  example `kind`) never list the step.

The hardening actions themselves (sysctl tuning, SSH policy, kernel parameters,
etcd and kubelet file permissions) are implemented in the Kubespray/Ansible
layer, not in this CLI. See
[Reference architecture](../concepts/reference-architecture.md) for where each
flag is passed.

## Opting out

Set either field to `false` in the cluster configuration:

```yaml
opencenter:
  cluster:
    kubernetes:
      security:
        k8s_hardening: false
  infrastructure:
    networking:
      security:
        os_hardening: false
```

Then validate and regenerate:

```bash
opencenter cluster validate ORG/CLUSTER
opencenter cluster generate ORG/CLUSTER
```

With `os_hardening: false` the `kubespray-os-hardening` step disappears from the
plan.

## Checking an existing cluster

Render, then confirm what the generated Terraform asserts:

```bash
opencenter cluster generate ORG/CLUSTER
grep -n 'hardening_enabled' <git-dir>/infrastructure/clusters/CLUSTER/main.tf
```

The dry-run deploy plan shows whether the hardening step is scheduled:

```bash
opencenter --dry-run cluster deploy ORG/CLUSTER
```
