---
id: service-longhorn
title: "Longhorn"
sidebar_label: Longhorn
description: Cloud-native distributed block storage for Kubernetes.
doc_type: reference
audience: "platform engineers, storage administrators"
tags: [storage, block-storage, distributed]
---

> **Purpose:** For platform engineers, documents Longhorn distributed storage configuration including replica settings, backup targets, and provisioning thresholds.

## Overview

Longhorn provides highly available distributed block storage for Kubernetes. It replicates data across multiple nodes for fault tolerance, supports S3 and NFS backup targets for disaster recovery, and exposes storage as standard Kubernetes PersistentVolumes. Longhorn manages volume lifecycle including snapshots, backups, and restoration without external storage infrastructure.

## Configuration

```yaml
opencenter:
  services:
    longhorn:
      enabled: true
      hostname: longhorn.example.com
      default_replica_count: 3
      default_data_path: /var/lib/longhorn
      storage_over_provisioning_percentage: 200
      storage_minimal_available_percentage: 25
      backup_target: s3://my-bucket@us-east-1/backups
      backup_target_credential_secret: longhorn-backup-secret
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable Longhorn storage |
| `hostname` | string | — | UI ingress hostname |
| `default_replica_count` | int | `3` | Number of replicas per volume |
| `default_data_path` | string | `/var/lib/longhorn` | Node storage path for volume data |
| `storage_over_provisioning_percentage` | int | `200` | Allowed over-provisioning percentage |
| `storage_minimal_available_percentage` | int | `25` | Minimum available storage before scheduling stops |
| `backup_target` | string | — | Backup destination (`s3://` or `nfs://`) |
| `backup_target_credential_secret` | string | — | Secret name with backup target credentials |

### Secrets

None.

## Dependencies

None.

## CLI Commands

```bash
opencenter cluster service enable longhorn
opencenter cluster service disable longhorn
opencenter cluster service status longhorn
opencenter cluster service options longhorn
```
