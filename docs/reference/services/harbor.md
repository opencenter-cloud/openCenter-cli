---
last_updated: 2026-11-02
id: service-harbor
title: "Harbor"
sidebar_label: Harbor
description: Harbor service configuration, runtime validation, secrets, and descriptor conditions.
doc_type: reference
audience: "platform engineers, operators"
tags: [registry, harbor, services]
---
# Harbor

**Purpose:** For platform engineers, operators, documents Harbor service configuration, runtime validation, secrets, and descriptor conditions.

> **Evidence:** `internal/config/services/harbor.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/harbor.go`, `internal/config/v2/config.go`, `internal/services/plugins/registry.go`, and `internal/services/descriptors/data/service-harbor.yaml`.

## Configuration

The generated default is disabled in `harbor`. `HarborConfig` embeds `BaseConfig` and adds access, storage, database, and TLS fields.

| Field group | Fields |
|-------------|--------|
| Access | `hostname`, `external_url`, `issuer_ref` |
| Storage | `storage_type`, `registry_volume_size`, `jobservice_volume_size`, `database_volume_size`, `redis_volume_size`, `trivy_volume_size`, `storage_class`, `s3_bucket`, `s3_region`, `s3_endpoint` |
| Database | `database_type`, `database_host`, `database_port`, `database_name`, `database_user` |
| TLS | `emit_certificate` |

When omitted from YAML, the custom unmarshaller sets `storage_type: s3`, volume sizes `100/10/10/10/10`; explicit zero values remain zero. `HarborSecrets` declares admin, database, registry, and S3 access/secret fields.

`issuer_ref` names the ClusterIssuer that mints Harbor's certificate. Empty means the issuer the Gateway listener in front of Harbor uses, which keeps the listener and the leaf certificate in agreement; pin it only to deliberately use a different CA for Harbor than the rest of the cluster.

## Runtime validation and dependencies

When enabled, the plugin validates HTTP(S) `external_url`, storage type `s3` or `filesystem`, S3 bucket/region for `s3`, external database fields for `database_type: external`, and non-negative registry size. The plugin registry records `cert-manager` as a Harbor dependency.

## Descriptor rendering

`service-harbor.yaml` lists HTTPRoute, Kustomization, Helm values, source, and Flux templates and aggregates into the services Flux and sources aggregates. The Certificate template is included only when `opencenter.services.harbor.emit_certificate` is true.

## Commands

```bash
opencenter cluster service enable harbor
opencenter cluster service disable harbor
opencenter cluster service options harbor
```
