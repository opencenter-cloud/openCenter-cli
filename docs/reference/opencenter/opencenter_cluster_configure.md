---
id: "opencenter-cluster-configure"
title: "Opencenter_Cluster_Configure"
sidebar_label: "Opencenter_Cluster_Configure"
description: "Guided cluster configuration for supported providers"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster configure

**Purpose:** For operators, developers, documents `opencenter cluster configure`: what it does, its flags, and its examples.

Guided cluster configuration for supported providers

```
opencenter cluster configure [name] [flags]
```

### Options

```
  -h, --help          help for configure
      --org string    organization for new cluster configurations
      --type string   infrastructure provider for new cluster configurations (default "openstack")
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
