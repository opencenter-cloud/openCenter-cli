---
last_updated: 2026-10-20
id: "opencenter-cluster-pool-update"
title: "Opencenter_Cluster_Pool_Update"
sidebar_label: "Opencenter_Cluster_Pool_Update"
description: "Update an existing worker pool"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
---
## opencenter cluster pool update

Update an existing worker pool

```
opencenter cluster pool update <pool-name> [flags]
```

### Options

```
      --boot-volume-size int      Boot volume size in GB (default 256 for Linux pools)
      --boot-volume-type string   Boot volume type (default HA-Performance for Linux pools)
      --cluster string            Cluster name
      --count int                 Number of nodes
      --flavor string             Instance flavor
  -h, --help                      help for update
      --image string              OS image override
      --label strings             Replace node labels with these (key=value, repeatable)
      --taint strings             Replace node taints with these (key=value:effect, repeatable)
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
