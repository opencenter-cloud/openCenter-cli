---
id: "opencenter-cluster-describe"
title: "Opencenter_Cluster_Describe"
sidebar_label: "Opencenter_Cluster_Describe"
description: "Describe cluster configuration, paths, locks, and state"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster describe

**Purpose:** For operators, developers, documents `opencenter cluster describe`: what it does, its flags, and its examples.

Describe cluster configuration, paths, locks, and state

### Synopsis

Describe cluster configuration, paths, locks, and state.

The cluster name can be specified in two formats:
  - cluster-name (uses organization from config)
  - organization/cluster-name (explicit organization)

```
opencenter cluster describe [name] [flags]
```

### Options

```
  -h, --help       help for describe
      --validate   validate cluster configuration invariants
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
