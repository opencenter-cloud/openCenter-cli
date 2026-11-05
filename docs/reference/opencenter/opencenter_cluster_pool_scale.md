---
id: "opencenter-cluster-pool-scale"
title: "Opencenter_Cluster_Pool_Scale"
sidebar_label: "Opencenter_Cluster_Pool_Scale"
description: "Scale a worker pool to a specific count"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster pool scale

**Purpose:** For operators, developers, documents `opencenter cluster pool scale`: what it does, its flags, and its examples.

Scale a worker pool to a specific count

```
opencenter cluster pool scale <pool-name> [flags]
```

### Options

```
      --cluster string   Cluster name
      --count int        Target node count
  -h, --help             help for scale
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
