---
id: "opencenter-cluster-migrate-layout"
title: "Opencenter_Cluster_Migrate Layout"
sidebar_label: "Opencenter_Cluster_Migrate Layout"
description: "Migrate legacy cluster files into secure GitOps, state, and secrets zones"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster migrate-layout

**Purpose:** For operators, developers, documents `opencenter cluster migrate-layout`: what it does, its flags, and its examples.

Migrate legacy cluster files into secure GitOps, state, and secrets zones

### Synopsis

Migrate the legacy mixed org-root layout into the secure layout.

This is the only command allowed to read the old layout where a Git repository,
cluster state files, and private secrets share the same organization directory.
Normal cluster commands reject that layout.

The command moves GitOps content to the configured GitOps root, cluster config
and local state files to the cluster state root, and private keys to the secrets
root. Use --dry-run to print the move diff without changing files.

Use --custom with --cluster to migrate unknown hand-authored overlay files into
service custom/ directories. Custom migration is a dry run by default; use
--apply explicitly to change files.

```
opencenter cluster migrate-layout --org <organization> [flags]
```

### Examples

```
  # Preview migration for the acme organization
  opencenter cluster migrate-layout --org acme --dry-run

  # Perform migration, refusing to overwrite destinations
  opencenter cluster migrate-layout --org acme

  # Perform migration and replace existing destinations
  opencenter cluster migrate-layout --org acme --force

  # Preview hand-authored overlay migration for one cluster
  opencenter cluster migrate-layout --custom --org acme --cluster prod

  # Apply the hand-authored overlay migration
  opencenter cluster migrate-layout --custom --org acme --cluster prod --apply
```

### Options

```
      --apply            Apply --custom migration; without this flag it is a dry run
      --cluster string   Cluster name for --custom migration
      --custom           Migrate hand-authored overlay files into custom/ (dry run by default)
      --dry-run          Print planned moves without changing files
      --force            Overwrite existing destination files
  -h, --help             help for migrate-layout
      --org string       Organization name to migrate
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter cluster](opencenter_cluster.md)	 - Manage cluster configurations
