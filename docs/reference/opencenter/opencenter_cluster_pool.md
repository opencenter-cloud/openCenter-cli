---
id: "opencenter-cluster-pool"
title: "Opencenter_Cluster_Pool"
sidebar_label: "Opencenter_Cluster_Pool"
description: "Manage worker pools"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster pool

**Purpose:** For operators, developers, documents `opencenter cluster pool`: what it does, its flags, and its examples.

Manage worker pools

### Synopsis

Add, update, remove, scale, and list worker pools in a cluster configuration.

### Options

```
  -h, --help   help for pool
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
* [opencenter cluster pool add](opencenter_cluster_pool_add.md)	 - Add a worker pool to the cluster configuration
* [opencenter cluster pool list](opencenter_cluster_pool_list.md)	 - List all worker pools
* [opencenter cluster pool remove](opencenter_cluster_pool_remove.md)	 - Remove a worker pool from the cluster configuration
* [opencenter cluster pool scale](opencenter_cluster_pool_scale.md)	 - Scale a worker pool to a specific count
* [opencenter cluster pool update](opencenter_cluster_pool_update.md)	 - Update an existing worker pool
