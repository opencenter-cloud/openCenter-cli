---
id: "opencenter-cluster-backup-create"
title: "Opencenter_Cluster_Backup_Create"
sidebar_label: "Opencenter_Cluster_Backup_Create"
description: "Create a backup of cluster configuration"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster backup create

**Purpose:** For operators, developers, documents `opencenter cluster backup create`: what it does, its flags, and its examples.

Create a backup of cluster configuration

### Synopsis

Create a backup of cluster configuration and related files.

The backup includes:
  - Cluster configuration YAML
  - SOPS Age encryption keys
  - SSH keys
  - GitOps repository state
  - Terraform state files

Backups are compressed with gzip and can be encrypted with a passphrase.

If no cluster name is provided, uses the currently active cluster.

```
opencenter cluster backup create [cluster] [flags]
```

### Examples

```
  # Create a backup of active cluster
  opencenter cluster backup create

  # Create a backup of specific cluster
  opencenter cluster backup create my-cluster

  # Create an encrypted backup (will prompt for passphrase)
  opencenter cluster backup create my-cluster --encrypt

  # Create an encrypted backup with passphrase
  opencenter cluster backup create my-cluster --passphrase="secret123"
```

### Options

```
      --encrypt             Encrypt backup (will prompt for passphrase)
  -h, --help                help for create
      --passphrase string   Passphrase for backup encryption
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
