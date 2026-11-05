---
id: "opencenter-cluster-backup-schedule"
title: "Opencenter_Cluster_Backup_Schedule"
sidebar_label: "Opencenter_Cluster_Backup_Schedule"
description: "Schedule periodic backups for a cluster"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster backup schedule

**Purpose:** For operators, developers, documents `opencenter cluster backup schedule`: what it does, its flags, and its examples.

Schedule periodic backups for a cluster

### Synopsis

Schedule periodic backups for a cluster.

This command runs as a foreground interval scheduler. It creates a backup on
each interval tick, prunes backups older than the retention window, and keeps
running until interrupted.

If no cluster name is provided, uses the currently active cluster.

```
opencenter cluster backup schedule [cluster] [flags]
```

### Examples

```
  # Schedule daily backups for active cluster
  opencenter cluster backup schedule --interval=24h

  # Schedule daily backups for specific cluster
  opencenter cluster backup schedule my-cluster --interval=24h

  # Schedule with retention policy
  opencenter cluster backup schedule my-cluster --interval=24h --retention=30d
```

### Options

```
  -h, --help               help for schedule
      --interval string    Backup interval (e.g., 24h, 7d) (default "24h")
      --retention string   Backup retention period (e.g., 30d, 90d) (default "30d")
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
