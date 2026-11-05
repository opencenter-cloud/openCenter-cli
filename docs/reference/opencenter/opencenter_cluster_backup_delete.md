---
id: "opencenter-cluster-backup-delete"
title: "Opencenter_Cluster_Backup_Delete"
sidebar_label: "Opencenter_Cluster_Backup_Delete"
description: "Delete a backup"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster backup delete

**Purpose:** For operators, developers, documents `opencenter cluster backup delete`: what it does, its flags, and its examples.

Delete a backup

### Synopsis

Delete a backup by its ID.

This operation is irreversible. Use with caution.

```
opencenter cluster backup delete <backup-id> [flags]
```

### Examples

```
  # Delete a backup
  opencenter cluster backup delete my-cluster-20260118-143000

  # Delete without confirmation
  opencenter cluster backup delete my-cluster-20260118-143000 --force
```

### Options

```
      --force   Delete without confirmation
  -h, --help    help for delete
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
