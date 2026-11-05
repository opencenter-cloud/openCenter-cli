---
id: "opencenter-cluster-pool-remove"
title: "Opencenter_Cluster_Pool_Remove"
sidebar_label: "Opencenter_Cluster_Pool_Remove"
description: "Remove a worker pool from the cluster configuration"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster pool remove

**Purpose:** For operators, developers, documents `opencenter cluster pool remove`: what it does, its flags, and its examples.

Remove a worker pool from the cluster configuration

### Synopsis

Remove a worker pool definition. The pool must have count=0 (scaled to zero)
before removal to prevent orphaned infrastructure. Use --force to bypass.

```
opencenter cluster pool remove <pool-name> [flags]
```

### Options

```
      --cluster string   Cluster name
      --force            Bypass scale-to-zero check
  -h, --help             help for remove
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

* [opencenter cluster pool](opencenter_cluster_pool.md)	 - Manage worker pools
