---
id: "opencenter-cluster-service-reconcile"
title: "Opencenter_Cluster_Service_Reconcile"
sidebar_label: "Opencenter_Cluster_Service_Reconcile"
description: "Reconcile a specific service in GitOps and Flux"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster service reconcile

**Purpose:** For operators, developers, documents `opencenter cluster service reconcile`: what it does, its flags, and its examples.

Reconcile a specific service in GitOps and Flux

### Synopsis

Reconcile a specific service in the cluster's GitOps repository and Flux.
Stages service-scoped manifests, validates secrets, commits, pushes, and triggers
in-cluster Flux reconciliation for the service's Kustomizations and HelmRelease.

Examples:
  # Reconcile openstack-ccm on active cluster
  opencenter cluster service reconcile openstack-ccm

  # Re-render service values and reconcile
  opencenter cluster service reconcile headlamp --generate

  # Reconcile on a specific cluster
  opencenter cluster service reconcile openstack-ccm --cluster opensnack/doritos

```
opencenter cluster service reconcile <service-name> [flags]
```

### Options

```
  -c, --cluster string      target cluster name (defaults to active cluster)
      --flux-only           execute Flux in-cluster reconciliation only; skip Git commit & push
  -g, --generate            re-render and re-encrypt service overlay before reconciling
  -h, --help                help for reconcile
      --kubeconfig string   path to kubeconfig (defaults to cluster-owned kubeconfig)
  -m, --message string      custom git commit message
      --push-only           execute GitOps commit & push only; skip Flux in-cluster reconciliation
      --remote string       git remote to push to (defaults to tracking remote, upstream, or origin)
      --skip-flux           skip Flux in-cluster reconciliation
      --timeout duration    timeout for Flux reconciliation and rollout (default 5m0s)
  -w, --wait                wait for Flux resources to become Ready (default true)
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --dry-run             preview mutating operations without writing or acting
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter cluster service](opencenter_cluster_service.md)	 - Manage cluster services
