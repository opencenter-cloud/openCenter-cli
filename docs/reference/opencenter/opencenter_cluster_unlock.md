---
id: "opencenter-cluster-unlock"
title: "Opencenter_Cluster_Unlock"
sidebar_label: "Opencenter_Cluster_Unlock"
description: "Unlock a cluster to allow modifications"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster unlock

**Purpose:** For operators, developers, documents `opencenter cluster unlock`: what it does, its flags, and its examples.

Unlock a cluster to allow modifications

### Synopsis

Unlock a previously locked cluster to allow modifications.

This command removes the lock from a cluster, allowing it to be modified again.
A reason must be provided to document why the cluster is being unlocked.

Examples:
  # Unlock the currently selected cluster
  opencenter cluster unlock --reason "Maintenance completed"

  # Unlock a specific cluster
  opencenter cluster unlock my-cluster --reason "Emergency fix applied"

  # Unlock a cluster in a specific organization
  opencenter cluster unlock myorg/my-cluster --reason "Approved by ops team"

```
opencenter cluster unlock [name] [flags]
```

### Options

```
  -h, --help            help for unlock
  -r, --reason string   Reason for unlocking the cluster (required)
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
