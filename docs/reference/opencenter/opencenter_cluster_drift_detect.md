---
id: "opencenter-cluster-drift-detect"
title: "Opencenter_Cluster_Drift_Detect"
sidebar_label: "Opencenter_Cluster_Drift_Detect"
description: "Detect infrastructure drift for a cluster"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster drift detect

**Purpose:** For operators, developers, documents `opencenter cluster drift detect`: what it does, its flags, and its examples.

Detect infrastructure drift for a cluster

### Synopsis

Detect differences between desired configuration and actual infrastructure state.

This command queries cloud provider APIs to retrieve the actual state of resources
(VMs, networks, security groups, load balancers) and compares them with the desired
configuration. It generates a drift report showing:
  - Resource type and ID
  - Field that has drifted
  - Expected vs actual values
  - Severity (critical, warning, info)
  - Whether the drift is reconcilable

If no cluster name is provided, uses the currently active cluster.

```
opencenter cluster drift detect [cluster] [flags]
```

### Examples

```
  # Detect drift for active cluster
  opencenter cluster drift detect

  # Detect drift for a specific cluster
  opencenter cluster drift detect my-cluster

  # Output drift report as JSON
  opencenter cluster drift detect my-cluster --output=json

  # Show only critical drift
  opencenter cluster drift detect my-cluster --severity=critical
```

### Options

```
  -h, --help              help for detect
      --severity string   Filter by severity (critical, warning, info)
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

* [opencenter cluster drift](opencenter_cluster_drift.md)	 - Detect and reconcile infrastructure drift
