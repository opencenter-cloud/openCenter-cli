---
id: "opencenter-cluster-backup"
title: "Opencenter_Cluster_Backup"
sidebar_label: "Opencenter_Cluster_Backup"
description: "Manage cluster backups"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster backup

**Purpose:** For operators, developers, documents `opencenter cluster backup`: what it does, its flags, and its examples.

Manage cluster backups

### Synopsis

Manage cluster configuration backups for disaster recovery.

Backups include:
  - Cluster configuration file
  - SOPS Age encryption keys
  - SSH keys
  - GitOps repository state
  - Terraform state files

Backups are compressed, encrypted with AES-256-GCM, and include SHA-256 checksums
for integrity verification.

```
opencenter cluster backup [flags]
```

### Examples

```
  # Create a backup
  opencenter cluster backup create my-cluster

  # Create an encrypted backup
  opencenter cluster backup create my-cluster --passphrase

  # List backups for a cluster
  opencenter cluster backup list my-cluster

  # Restore from backup
  opencenter cluster backup restore my-cluster-20260118-143000

  # Schedule periodic backups
  opencenter cluster backup schedule my-cluster --interval=24h
```

### Options

```
  -h, --help   help for backup
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
* [opencenter cluster backup create](opencenter_cluster_backup_create.md)	 - Create a backup of cluster configuration
* [opencenter cluster backup delete](opencenter_cluster_backup_delete.md)	 - Delete a backup
* [opencenter cluster backup list](opencenter_cluster_backup_list.md)	 - List backups for a cluster
* [opencenter cluster backup restore](opencenter_cluster_backup_restore.md)	 - Restore cluster configuration from backup
* [opencenter cluster backup schedule](opencenter_cluster_backup_schedule.md)	 - Schedule periodic backups for a cluster
