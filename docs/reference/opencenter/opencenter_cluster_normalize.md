---
id: "opencenter-cluster-normalize"
title: "Opencenter_Cluster_Normalize"
sidebar_label: "Opencenter_Cluster_Normalize"
description: "Add missing default fields to a cluster configuration"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster normalize

**Purpose:** For operators, developers, documents `opencenter cluster normalize`: what it does, its flags, and its examples.

Add missing default fields to a cluster configuration

### Synopsis

Add missing default fields to an existing cluster configuration.

This command loads the current cluster configuration, merges it with the default
configuration template to add any missing keys, and writes the updated configuration
back to the file.

To set specific configuration values, use 'opencenter cluster set' with native v2
dot notation instead (e.g., opencenter.gitops.repository.url=...).

A timestamped backup is automatically created before modification:
  <config-file>.backup.<timestamp>

The backup allows you to review changes and revert if needed. Delete the backup
once you're satisfied with the updated configuration.

Missing keys are added with their default values based on:
  • Provider-specific defaults (if provider is configured)
  • Global schema defaults
  • Empty/zero values for required fields

Existing values are preserved - only missing keys are added.

If no cluster name is provided, normalizes the currently active cluster.

```
opencenter cluster normalize [name] [flags]
```

### Examples

```
  # Normalize active cluster configuration
  opencenter cluster normalize

  # Normalize specific cluster
  opencenter cluster normalize my-cluster

  # Normalize with organization prefix
  opencenter cluster normalize myorg/my-cluster

  # Dry run to preview changes
  opencenter cluster normalize my-cluster --dry-run

  # Update without creating backup (not recommended)
  opencenter cluster normalize my-cluster --no-backup
```

### Options

```
  -h, --help        help for normalize
      --no-backup   skip creating a backup before normalizing
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
