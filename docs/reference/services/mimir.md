---
last_updated: 2026-09-25
id: service-mimir
title: "Grafana Mimir"
sidebar_label: Mimir
description: Long-term metrics storage with typed S3-compatible and legacy Swift backends.
doc_type: reference
audience: "platform engineers, operators"
tags: [monitoring, metrics, mimir, object-storage, rustfs, services]
---

> **Purpose:** For platform engineers and operators, documents the Mimir service's configuration surface.

## Overview

Mimir provides long-term metrics storage. Its service configuration has a typed
object-storage contract: new deployments should use the S3-compatible backend,
while the legacy Swift backend remains available for existing OpenStack
deployments. Mimir does not support filesystem or `storage_type: none` modes.

For non-production clusters, the openCenter-managed RustFS profile can provide
the S3-compatible object store. RustFS does **not** auto-provide Mimir
settings, including its endpoint or credentials. The current implementation
still requires `s3_endpoint` and Mimir S3 credentials; validation fails with a
clear error when either is missing. The profile also requires Longhorn:

```yaml
opencenter:
  infrastructure:
    storage:
      profile:
        lifecycle: non-production
        pvc_provider: longhorn
        object_storage_provider: rustfs
  services:
    longhorn:
      enabled: true
    mimir:
      enabled: true
      storage_type: s3
      s3_endpoint: https://rustfs.example.com

secrets:
  mimir:
    s3_access_key_id: "..."
    s3_secret_access_key: "..."
```

RustFS is restricted to non-production storage profiles. Production profiles
must use an externally managed S3-compatible endpoint and credentials.

## Configuration

```yaml
opencenter:
  services:
    mimir:
      enabled: false               # default: false
      namespace: observability      # default: observability
      storage_type: s3              # s3 or legacy swift; default: swift
      bucket_name:                   # Mimir blocks storage bucket
      ruler_bucket_name:             # optional; defaults to <bucket_name>-ruler
      alertmanager_bucket_name:      # optional; defaults to <bucket_name>-alertmanager
      s3_endpoint:
      s3_region:
      s3_credential_id:
      s3_force_path_style: false
      s3_insecure: false
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Whether Mimir is deployed |
| `namespace` | string | `observability` | Namespace for Mimir resources |
| `storage_type` | string | `swift` | Storage backend: `s3` for S3-compatible object storage or `swift` for legacy compatibility |
| `bucket_name` | string | `<cluster>-mimir` | Bucket/container used by Mimir blocks storage |
| `ruler_bucket_name` | string | `<bucket_name>-ruler` | Optional S3 bucket for Mimir ruler storage |
| `alertmanager_bucket_name` | string | `<bucket_name>-alertmanager` | Optional S3 bucket for Mimir's internal alertmanager storage |
| `s3_endpoint` | string | — | Absolute HTTP(S) S3-compatible endpoint URL; Mimir uses only its `host[:port]` authority, so URL paths are unsupported |
| `s3_region` | string | — | S3 region; the cluster region is used when omitted |
| `s3_credential_id` | string | — | OpenStack EC2 credential ID for the S3-compatible service |
| `s3_force_path_style` | bool | `false` | Force S3 path-style addressing |
| `s3_insecure` | bool | `false` | Allow insecure (HTTP) S3 connections |

When `storage_type: s3` is selected, the storage contract provisions three
distinct buckets: `bucket_name` configures `blocks_storage`, while the ruler
and alertmanager bucket fields configure Mimir's `ruler_storage` and internal
`alertmanager_storage`, respectively. If omitted, the latter two default to
`<bucket_name>-ruler` and `<bucket_name>-alertmanager`. The same endpoint,
region, credentials, path-style setting, and TLS/insecure setting are used for
all three stores.

`s3_endpoint` is the public configuration form and must be an absolute
HTTP(S) URL such as `https://s3.example.com:9000`. Mimir renders the endpoint
as `host[:port]`; endpoint paths, queries, and fragments are unsupported and
are rejected by validation. Set `s3_force_path_style: true` when the object
store requires path-style addressing. The renderer maps that setting to
Mimir's `bucket_lookup_type: path` configuration for each S3 store; it does
not encode a bucket path in the endpoint.

For an externally managed S3-compatible service, configure the endpoint and
credentials explicitly:

```yaml
opencenter:
  services:
    mimir:
      enabled: true
      storage_type: s3
      bucket_name: metrics-blocks
      ruler_bucket_name: metrics-ruler
      alertmanager_bucket_name: metrics-alertmanager
      s3_endpoint: https://s3.example.com
      s3_region: us-east-1
      s3_force_path_style: true
      s3_insecure: false

secrets:
  mimir:
    s3_access_key_id: "..."
    s3_secret_access_key: "..."
```

The service-specific S3 keys are preferred; the configured global AWS
application/infrastructure credentials are used as a compatibility fallback.
The controller writes the effective credentials to the generated Kubernetes
secret `opencenter-mimir-secret`. Mimir receives them through environment
variables referenced by the rendered configuration, with
`-config.expand-env=true`; the credential values are not stored literally in
the rendered Mimir config.

## Legacy Swift compatibility

`storage_type: swift` remains supported only for existing OpenStack
deployments. The `swift_*` fields are retained in the schema for that
compatibility backend:
`swift_auth_url`, `swift_region`, `swift_auth_version`,
`swift_application_credential_id`, `swift_container_name`, `swift_username`,
`swift_project_name`, `swift_project_domain_name`, `swift_user_domain_name`,
and `swift_domain_name`. Swift deployments use:

```yaml
secrets:
  mimir:
    swift_application_credential_secret: "..."
```

The Swift compatibility path configures Mimir's legacy blocks storage only;
the typed S3 fields and S3 bucket settings for ruler and internal alertmanager
storage do not apply. Its application-credential secret is likewise consumed
from `opencenter-mimir-secret` through environment expansion. Do not use the
legacy Swift path for new S3 deployments.

## Secrets

For `storage_type: s3`, configure `secrets.mimir.s3_access_key_id` and
`secrets.mimir.s3_secret_access_key` (or the documented compatibility
fallback). The managed RustFS profile does not supply these Mimir settings
automatically. For `storage_type: swift`, use the legacy application
credential secret described above.

## Dependencies

None enforced by `opencenter cluster service enable|disable`.

## Rendering

`mimir` has no dedicated YAML descriptor; it is rendered through the built-in render catalog with extra rendering-order dependencies on the observability namespace/sources and an override dependency on `sources`. Enabling `mimir` also causes the generated `services/sources/kustomization.yaml.tpl` to include the shared `opencenter-observability` source.

## CLI commands

```bash
opencenter cluster service enable mimir
opencenter cluster service disable mimir
opencenter cluster service status
opencenter cluster service options mimir
```
