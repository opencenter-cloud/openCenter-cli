---
last_updated: 2026-10-20
id: "opencenter-cluster-pool-add"
title: "Opencenter_Cluster_Pool_Add"
sidebar_label: "Opencenter_Cluster_Pool_Add"
description: "Add a worker pool to the cluster configuration"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
---
## opencenter cluster pool add

Add a worker pool to the cluster configuration

```
opencenter cluster pool add <pool-name> [flags]
```

### Options

```
      --boot-volume-size int      Boot volume size in GB (default 256 for Linux pools)
      --boot-volume-type string   Boot volume type (default HA-Performance for Linux pools)
      --cluster string            Cluster name
      --count int                 Number of nodes in the pool (default 1)
      --flavor string             Instance flavor (required)
  -h, --help                      help for add
      --image string              OS image override
      --label strings             Node label (key=value, repeatable)
      --os string                 Pool OS type (linux|windows) (default "linux")
      --pf9-onboard               Request PF9 onboarding for this Linux pool's nodes
      --taint strings             Node taint (key=value:effect, repeatable)
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
