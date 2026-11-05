---
id: "opencenter-cluster-export"
title: "Opencenter_Cluster_Export"
sidebar_label: "Opencenter_Cluster_Export"
description: "Export effective cluster configuration"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster export

**Purpose:** For operators, developers, documents `opencenter cluster export`: what it does, its flags, and its examples.

Export effective cluster configuration

### Synopsis

Export the effective configuration including all applied defaults.

This command loads a cluster configuration, applies all defaults (provider-region,
provider, and global defaults), and exports the complete configuration with comments
indicating which values came from defaults vs explicit configuration.

The effective configuration shows:
  • All explicitly configured values
  • All applied defaults with their source (provider-region, provider, global)
  • Comments indicating default sources for transparency

This is useful for:
  • Understanding which defaults are being applied
  • Debugging configuration issues
  • Creating explicit configurations from defaults
  • Documentation and auditing

If no cluster name is provided, exports the currently active cluster.

```
opencenter cluster export [name] [flags]
```

### Examples

```
  # Export effective config for active cluster
  opencenter cluster export

  # Export effective config for specific cluster
  opencenter cluster export my-cluster

  # Export to specific file
  opencenter cluster export my-cluster -o /tmp/effective-config.yaml

  # Export with organization prefix
  opencenter cluster export myorg/my-cluster
```

### Options

```
  -h, --help                 help for export
  -o, --output-file string   write effective configuration to this file instead of stdout
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
