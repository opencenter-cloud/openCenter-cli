---
id: service-velero
title: "Velero"
sidebar_label: Velero
description: Cluster backup and disaster recovery with multi-cloud storage backends.
doc_type: reference
audience: "operators, platform engineers"
tags: [velero, backup, disaster-recovery, s3, swift, gcs, azure]
---

# Velero

> **Purpose:** For operators and platform engineers, documents Velero configuration, storage backends, and secrets.

## Overview

Velero provides backup and disaster recovery for Kubernetes cluster resources and persistent volumes. Supports scheduled backups, on-demand snapshots, and cross-cluster migration.

## Configuration

```yaml
opencenter:
  services:
    velero:
      enabled: true
      backup_bucket: ""        # bucket name (defaults to "<cluster_name>-velero")
      region: ""               # storage region
      storage_type: ""         # "s3", "swift", "gcs", or "azure" (auto-detected from provider)
```

## Secrets

Secrets depend on `storage_type`:

### Swift (`storage_type: swift`)

```yaml
secrets:
  service_secrets:
    velero:
      swift_password: ""       # Swift password or app credential secret
```

### S3 (`storage_type: s3`)

```yaml
secrets:
  service_secrets:
    velero:
      s3_access_key: ""
      s3_secret_key: ""
```

### GCS (`storage_type: gcs`)

```yaml
secrets:
  service_secrets:
    velero:
      gcp_service_account_key: ""   # GCP service account JSON key
```

### Azure (`storage_type: azure`)

```yaml
secrets:
  service_secrets:
    velero:
      azure_storage_account_key: ""
```

## Dependencies

- **external-snapshotter** — CSI volume snapshot support

## Storage Provider Defaults

| Infrastructure | Default `storage_type` |
|---|---|
| OpenStack | swift |
| AWS | s3 |
| GCP | gcs |
| Azure | azure |

## Architecture

Velero deploys with:
- Velero server pod with BackupStorageLocation configured
- OpenStack plugin (`velero-plugin-for-openstack`) as init container
- VolumeSnapshotClass for CSI snapshots
- No node agent (DaemonSet) by default — uses CSI snapshots

Features enabled: CSI support, snapshot move data disabled, filesystem backup disabled.

## Verification

```bash
# Check Velero pods
kubectl get pods -n velero

# List backups
kubectl get backups -n velero

# Create a test backup
velero backup create test-backup --include-namespaces default

# Check backup storage location
velero backup-location get
```

## CLI Commands

```bash
opencenter cluster service enable velero
opencenter cluster service disable velero
opencenter cluster service options velero
opencenter cluster backup create my-cluster
opencenter cluster backup restore <backup-id>
```
