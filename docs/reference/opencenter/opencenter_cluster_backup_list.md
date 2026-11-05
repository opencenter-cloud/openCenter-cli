---
id: "opencenter-cluster-backup-list"
title: "Opencenter_Cluster_Backup_List"
sidebar_label: "Opencenter_Cluster_Backup_List"
description: "List backups for a cluster"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster backup list

**Purpose:** For operators, developers, documents `opencenter cluster backup list`: what it does, its flags, and its examples.

List backups for a cluster

### Synopsis

List all backups for a cluster or all backups if no cluster is specified.

Displays backup ID, creation time, size, and storage location.

```
opencenter cluster backup list [cluster] [flags]
```

### Examples

```
  # List all backups
  opencenter cluster backup list

  # List backups for a specific cluster
  opencenter cluster backup list my-cluster
```

### Options

```
  -h, --help   help for list
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

* [opencenter cluster backup](opencenter_cluster_backup.md)	 - Manage cluster backups
