---
id: "opencenter-cluster-drift"
title: "Opencenter_Cluster_Drift"
sidebar_label: "Opencenter_Cluster_Drift"
description: "Detect and reconcile infrastructure drift"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster drift

**Purpose:** For operators, developers, documents `opencenter cluster drift`: what it does, its flags, and its examples.

Detect and reconcile infrastructure drift

### Synopsis

Detect and reconcile differences between desired configuration and actual infrastructure state.

Drift detection compares the cluster configuration with the actual state of cloud resources
(VMs, networks, security groups, load balancers) and reports any differences. Drift can be
classified by severity (critical, warning, info) and reconcilability.

```
opencenter cluster drift [flags]
```

### Examples

```
  # Detect drift for a cluster
  opencenter cluster drift detect my-cluster

  # Reconcile detected drift (dry-run)
  opencenter cluster drift reconcile my-cluster --dry-run

  # Reconcile detected drift (apply changes)
  opencenter cluster drift reconcile my-cluster

  # Schedule periodic drift detection
  opencenter cluster drift schedule my-cluster --interval=24h
```

### Options

```
  -h, --help   help for drift
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

* [opencenter cluster](opencenter_cluster.md)	 - Manage cluster configurations
* [opencenter cluster drift detect](opencenter_cluster_drift_detect.md)	 - Detect infrastructure drift for a cluster
* [opencenter cluster drift reconcile](opencenter_cluster_drift_reconcile.md)	 - Reconcile detected infrastructure drift
* [opencenter cluster drift schedule](opencenter_cluster_drift_schedule.md)	 - Schedule periodic drift detection
