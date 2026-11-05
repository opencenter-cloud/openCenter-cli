---
id: "opencenter-cluster-backup-restore"
title: "Opencenter_Cluster_Backup_Restore"
sidebar_label: "Opencenter_Cluster_Backup_Restore"
description: "Restore cluster configuration from backup"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster backup restore

**Purpose:** For operators, developers, documents `opencenter cluster backup restore`: what it does, its flags, and its examples.

Restore cluster configuration from backup

### Synopsis

Restore cluster configuration and related files from a backup.

The backup ID is the filename without extension (e.g., my-cluster-20260118-143000).

Restored files are placed in a "restored" directory to avoid overwriting existing
configurations. You can then manually move them to the appropriate locations.

```
opencenter cluster backup restore <backup-id> [flags]
```

### Examples

```
  # Restore from backup
  opencenter cluster backup restore my-cluster-20260118-143000

  # Restore from encrypted backup
  opencenter cluster backup restore my-cluster-20260118-143000 --passphrase="secret123"
```

### Options

```
  -h, --help                help for restore
      --passphrase string   Passphrase for backup decryption
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
