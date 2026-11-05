---
id: "opencenter-cluster-active"
title: "Opencenter_Cluster_Active"
sidebar_label: "Opencenter_Cluster_Active"
description: "Show the active cluster"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster active

**Purpose:** For operators, developers, documents `opencenter cluster active`: what it does, its flags, and its examples.

Show the active cluster

### Synopsis

Show the current active cluster with its selection source.

The active cluster follows this precedence:
  1. OPENCENTER_CLUSTER environment variable (session-scoped)
  2. Session file (if shell integration is active)
  3. Persistent selection from marker file

Use --quiet to output only the cluster name without source information.

```
opencenter cluster active [flags]
```

### Examples

```
  # Show current cluster with source
  opencenter cluster active

  # Show only cluster name (for scripting)
  opencenter cluster active --quiet
```

### Options

```
  -h, --help    help for active
  -q, --quiet   quiet output (just the name)
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --dry-run             preview mutating operations without writing or acting
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter cluster](opencenter_cluster.md)	 - Manage cluster configurations
