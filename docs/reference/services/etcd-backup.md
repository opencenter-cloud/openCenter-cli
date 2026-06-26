---
id: service-etcd-backup
title: "etcd Backup"
sidebar_label: etcd Backup
description: Scheduled etcd snapshot backups to S3-compatible storage.
doc_type: reference
audience: "platform engineers, operators"
tags: [etcd, backup, disaster-recovery]
---

> **Purpose:** For platform engineers, documents the etcd backup service for scheduled cluster state snapshots to S3-compatible storage.

## Overview

The etcd backup service provides scheduled snapshots of the etcd datastore to S3-compatible object storage. This enables point-in-time recovery of the Kubernetes control plane state in disaster recovery scenarios. The service uses global AWS credentials for S3 access—no service-specific secrets are required.

## Configuration

```yaml
opencenter:
  services:
    etcd-backup:
      enabled: true
      s3_host: s3.us-east-1.amazonaws.com
      s3_region: us-east-1
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable etcd snapshot backups |
| `s3_host` | string | — | S3-compatible endpoint hostname |
| `s3_region` | string | — | S3 bucket region |

### Secrets

None (uses global AWS credentials).

## Dependencies

None.

## CLI Commands

```bash
opencenter cluster service enable etcd-backup
opencenter cluster service disable etcd-backup
opencenter cluster service status etcd-backup
opencenter cluster service options etcd-backup
```
