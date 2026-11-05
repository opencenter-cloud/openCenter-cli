---
last_updated: 2026-09-25
id: service-velero
title: "Velero"
sidebar_label: Velero
description: Velero service configuration, storage validation, secrets, and catalog ownership.
doc_type: reference
audience: "operators, platform engineers"
tags: [velero, backup, services]
---
# Velero

**Purpose:** For operators, platform engineers, documents Velero service configuration, storage validation, secrets, and catalog ownership.

> **Evidence:** `internal/config/services/velero.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/velero.go`, `internal/config/services/secrets_validator.go`, `internal/config/services/provider_registry.go`, and `internal/gitops/render_catalog.go`.

## Configuration

The generated default is enabled in `velero`. `VeleroConfig` embeds `BaseConfig` and adds `backup_bucket`, `region`, S3 endpoint/region/credential/path-style/insecure fields, and `storage_type`.

The type-level metadata documents `s3`, `swift`, `gcs`, `azure`, and `none`, with `s3` as default. The provider registry selects `s3` for Velero for every listed infrastructure provider and accepts `none` in its compatibility matrix.

## Runtime and secrets

The plugin requires `backup_bucket` when enabled unless `storage_type` is `none`. For `none`, the plugin returns without the bucket check. `VeleroSecrets` declares access and secret keys; the conditional mapping also records Swift, GCS, and Azure credential paths for their respective storage values.

## Rendering

The catalog uses namespace stage `velero`, base path `applications/base/services/velero`, an extra `velero-override` stage, and override dependencies `sources` and `velero-namespace`. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable velero
opencenter cluster service disable velero
opencenter cluster service options velero
```
