---
id: backup-and-restore
title: Back up and restore configuration
sidebar_label: Back up and restore configuration
description: Create, list, and restore openCenter configuration backup archives.
doc_type: how-to
audience: openCenter operators
tags: [backups, configuration]
last_updated: 2026-09-25
---
# Back up and restore configuration

**Purpose:** For openCenter operators, shows how to create, list, and restore openCenter configuration backup archives.

`cluster backup` manages openCenter configuration-related backup archives. It
is separate from any Kubernetes or provider-native backup service.

## Create and list backups

```bash
opencenter cluster backup create ORG/CLUSTER
opencenter cluster backup create ORG/CLUSTER --encrypt
opencenter cluster backup list ORG/CLUSTER
```

Create can accept `--passphrase`; encryption prompts when no passphrase is
provided. The implementation stores backups under the configured openCenter
config directory's `backups` directory and reports the resulting location.

## Restore

```bash
opencenter cluster backup restore BACKUP_ID
opencenter cluster backup restore BACKUP_ID --passphrase VALUE
```

The command restores into a `restored` area and prints the paths for review; it
does not silently replace an existing cluster file.

## Delete or run the foreground scheduler

```bash
opencenter cluster backup delete BACKUP_ID
opencenter cluster backup delete BACKUP_ID --force
opencenter cluster backup schedule ORG/CLUSTER --interval VALUE --retention VALUE
```

The scheduler runs in the foreground until interrupted. The implementation
accepts duration values understood by its duration parser; no operational
frequency or retention policy is implied here.

## Evidence and boundary

This command family backs up local openCenter artifacts. It does not document
etcd restore procedures, Velero behavior, cloud-object-storage setup, or a
successful disaster-recovery outcome because those are not established by the
CLI implementation in this checkout.

- Command and paths: `cmd/cluster_backup.go`
- Backup implementation: `internal/operations/`
