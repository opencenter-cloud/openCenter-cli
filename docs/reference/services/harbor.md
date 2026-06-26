---
id: service-harbor
title: "Harbor"
sidebar_label: Harbor
description: Enterprise container registry with vulnerability scanning and access control.
doc_type: reference
audience: "platform engineers, operators"
tags: [registry, containers, harbor]
---

> **Purpose:** For platform engineers, documents Harbor container registry configuration including storage backends, database options, and TLS.

## Overview

Harbor is an enterprise container registry that provides image storage, vulnerability scanning, content signing, and role-based access control. It supports multiple storage backends (filesystem, S3, Swift) and can use an internal or external PostgreSQL database. When deployed with openCenter, Harbor integrates with cert-manager for TLS certificate provisioning.

## Configuration

```yaml
opencenter:
  services:
    harbor:
      enabled: true
      hostname: registry.example.com
      external_url: https://registry.example.com
      storage_type: filesystem
      registry_volume_size: 100Gi
      s3_bucket: harbor-registry
      s3_region: us-east-1
      database_type: internal
      database_host: postgres.example.com
      database_port: 5432
      database_name: harbor
      database_user: harbor
      emit_certificate: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable Harbor registry |
| `hostname` | string | — | Ingress hostname |
| `external_url` | string | — | Externally reachable URL |
| `storage_type` | string | `filesystem` | Backend: `filesystem`, `s3`, or `swift` |
| `registry_volume_size` | string | `100Gi` | PVC size for filesystem storage |
| `s3_bucket` | string | — | S3 bucket name (when `storage_type: s3`) |
| `s3_region` | string | — | S3 region (when `storage_type: s3`) |
| `database_type` | string | `internal` | Database mode: `internal` or `external` |
| `database_host` | string | — | External database host |
| `database_port` | int | `5432` | External database port |
| `database_name` | string | — | External database name |
| `database_user` | string | — | External database user |
| `emit_certificate` | bool | — | Emit TLS certificate via cert-manager |

### Secrets

None documented.

## Dependencies

| Service | Required | Notes |
|---------|----------|-------|
| cert-manager | Yes | TLS certificate provisioning |
| postgres-operator | Conditional | Required when `database_type: external` |

## CLI Commands

```bash
opencenter cluster service enable harbor
opencenter cluster service disable harbor
opencenter cluster service status harbor
opencenter cluster service options harbor
```
