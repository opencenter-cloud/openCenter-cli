---
id: "opencenter-cluster-lock"
title: "Opencenter_Cluster_Lock"
sidebar_label: "Opencenter_Cluster_Lock"
description: "Lock a cluster to prevent modifications"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster lock

**Purpose:** For operators, developers, documents `opencenter cluster lock`: what it does, its flags, and its examples.

Lock a cluster to prevent modifications

### Synopsis

Lock a cluster to prevent accidental modifications.

A locked cluster cannot be modified until it is explicitly unlocked.
This is useful for protecting production clusters or clusters undergoing maintenance.

Examples:
  # Lock the currently selected cluster
  opencenter cluster lock --reason "Production cluster - do not modify"

  # Lock a specific cluster
  opencenter cluster lock my-cluster --reason "Under maintenance"

  # Lock a cluster in a specific organization
  opencenter cluster lock myorg/my-cluster --reason "Critical infrastructure"

```
opencenter cluster lock [name] [flags]
```

### Options

```
  -h, --help            help for lock
  -r, --reason string   Reason for locking the cluster (required)
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
