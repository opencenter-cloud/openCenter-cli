---
last_updated: 2026-09-25
id: service-etcd-backup
title: "etcd Backup"
sidebar_label: etcd Backup
description: etcd-backup configuration and descriptor conditional rendering.
doc_type: reference
audience: "platform engineers, operators"
tags: [etcd, backup, services]
---
# etcd Backup

**Purpose:** For platform engineers, operators, documents etcd-backup configuration and descriptor conditional rendering.

> **Evidence:** `internal/config/services/etcd_backup.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/default_services.go`, and `internal/services/descriptors/data/service-etcd-backup.yaml`.

## Configuration

The generated default is disabled in `kube-system`. `EtcdBackupConfig` embeds `BaseConfig` and adds `s3_host`, `s3_endpoint`, `s3_bucket_name`, `s3_credential_id`, `s3_region`, and `storage_type`.

The type-level metadata documents `storage_type` values `s3` and `none`, with `s3` as default. `EtcdBackupSecrets` declares `access_key_id` and `secret_access_key`.

## Runtime and descriptor facts

The plugin validator accepts only empty, `s3`, or `none` storage values. The descriptor root `services/etcd-backup` is conditional: it is included when `opencenter.services.etcd-backup.storage_type` is not equal to `none`. The descriptor aggregates into `services-fluxcd-aggregate`.

This condition is a descriptor fact; the plugin's `Render` method is a no-op and does not itself render the root.

## Commands

```bash
opencenter cluster service enable etcd-backup
opencenter cluster service disable etcd-backup
opencenter cluster service options etcd-backup
```
