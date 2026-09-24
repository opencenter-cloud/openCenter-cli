---
last_updated: 2026-09-24
id: service-harbor
title: "Harbor"
sidebar_label: Harbor
description: Container registry configuration, storage sizing, database options, and S3 or filesystem storage.
doc_type: reference
audience: "platform engineers, operators"
tags: [registry, containers, harbor, services]
---

> **Purpose:** For platform engineers, documents Harbor's configuration surface, storage/database options, and storage credentials.

## Overview

Harbor is a container registry. Image storage can use S3-compatible object storage or the Harbor registry PVC filesystem.

Filesystem storage is supported for production Harbor deployments, but it is not
recommended for durable registry storage. Use S3-compatible object storage for
durable registry data whenever possible; it remains the recommended production
backend.

With `storage_type: filesystem`, Harbor stores images on its registry PVC. This
mode does not use S3 and does not require Harbor S3 credentials. The other
Harbor PVCs (jobservice, database, Redis, and Trivy) remain configured as
usual.

```yaml
opencenter:
  services:
    harbor:
      enabled: true
      storage_type: filesystem
      registry_volume_size: 100
      storage_class: standard
```

## Configuration

```yaml
opencenter:
  services:
    harbor:
      enabled: false                    # default: false
      namespace: harbor                  # default: harbor
      hostname:
      external_url:
      storage_type: s3                     # default: s3; s3 | filesystem
      registry_volume_size: 100            # default: 100 (min 1)
      jobservice_volume_size: 10            # default: 10 (min 1; min 10 on Cinder-backed regions e.g. Rackspace SJC3)
      database_volume_size: 10              # default: 10 (min 1)
      redis_volume_size: 10                 # default: 10 (min 1; min 10 on Cinder-backed regions)
      trivy_volume_size: 10                 # default: 10 (min 1; min 10 on Cinder-backed regions)
      storage_class:
      s3_bucket:
      s3_region:
      s3_endpoint:
      database_type: internal               # default: internal; internal | external
      database_host:
      database_port:
      database_name:
      database_user:
      emit_certificate: false
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Whether Harbor is deployed |
| `namespace` | string | `harbor` | Namespace for Harbor resources |
| `hostname` | string | — | Harbor external hostname |
| `external_url` | string | — | External URL for Harbor; must be `http://` or `https://` when set |
| `storage_type` | string | `s3` | Image backend: `s3` or `filesystem`; filesystem uses the registry PVC and has no S3 credential requirement |
| `registry_volume_size` | int | `100` | Registry PVC size in GB (min 1); retained for compatibility and required Harbor cache/state |
| `jobservice_volume_size` | int | `10` | Jobservice log PVC size in GB (min 1; min 10 on Cinder-backed regions e.g. Rackspace SJC3) |
| `database_volume_size` | int | `10` | Internal database PVC size in GB (min 1) |
| `redis_volume_size` | int | `10` | Redis PVC size in GB (min 1; min 10 on Cinder-backed regions) |
| `trivy_volume_size` | int | `10` | Trivy PVC size in GB (min 1; min 10 on Cinder-backed regions) |
| `storage_class` | string | infrastructure `storage.default_storage_class` | Storage class for Harbor PVCs |
| `s3_bucket` | string | — | S3 bucket name for image storage |
| `s3_region` | string | — | S3 region |
| `s3_endpoint` | string | — | S3-compatible endpoint URL; must be a valid URL when set |
| `database_type` | string | `internal` | `internal` \| `external` |
| `database_host` / `database_port` / `database_name` / `database_user` | — | — | Required when `database_type: external` |
| `emit_certificate` | bool | — | Render the Harbor TLS certificate manifest via cert-manager |

The five volume-size defaults above (`100`/`10`/`10`/`10`/`10`) are applied by `HarborConfig.UnmarshalYAML` only when the corresponding field is omitted from YAML — an explicitly configured `0` is preserved so runtime validation can reject it.

## Secrets

```yaml
secrets:
  harbor:
    admin_password:
    database_password:
    registry_password:
    s3_access_key_id:
    s3_secret_access_key:
```

### Validation

For `storage_type: s3`, `opencenter cluster service enable harbor` requires the Harbor S3 access key and secret key to be provided together (both set, or both absent).
For `storage_type: filesystem`, Harbor uses the registry PVC and has no S3 credentials; `s3_bucket`, `s3_region`, `s3_endpoint`, and the Harbor S3 secret fields are not used.

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

Harbor has a dedicated descriptor (`internal/services/descriptors/data/service-harbor.yaml`, `service: harbor`) covering its `HTTPRoute`, Kustomization, Helm override values, and — conditionally, when `emit_certificate: true` — its `Certificate` manifest. It aggregates into `services-fluxcd-aggregate` and `services-sources-aggregate`.

## CLI commands

```bash
opencenter cluster service enable harbor --secret="s3_access_key_id=..." --secret="s3_secret_access_key=..."
opencenter cluster service disable harbor
opencenter cluster service status
opencenter cluster service options harbor
```
