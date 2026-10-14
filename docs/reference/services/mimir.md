---
last_updated: 2026-10-14
id: service-mimir
title: "Grafana Mimir"
sidebar_label: Mimir
description: Mimir service configuration, storage fields, secrets, and catalog ownership.
doc_type: reference
audience: "platform engineers, operators"
tags: [monitoring, metrics, mimir, observability, services]
---

> **Evidence:** `internal/config/services/mimir.go`, `internal/config/v2/defaults.go`, `internal/config/v2/config.go`, `internal/config/services/secrets_validator.go`, and `internal/gitops/render_catalog.go`.

## Configuration

The generated default is disabled in `observability`. `MimirConfig` embeds `BaseConfig` and exposes `storage_type`, `bucket_name`, `ruler_bucket_name`, `alertmanager_bucket_name`, S3 fields, and Swift fields.

The config metadata documents `s3` and `swift`, with `swift` as its type-level default. `defaults.go` materializes only disabled state and namespace. For S3, `GetMimirS3Credentials` prefers `secrets.mimir.s3_access_key_id` and `s3_secret_access_key`, then falls back to global AWS application credentials.

An unset `storage_type` resolves to `swift`, which is supported only on the `openstack` provider. Loading a configuration and `cluster set` do not block on this rule, so `storage_type` can be corrected on an existing configuration without failing unrelated service updates (OCTR-812). `cluster validate`, generation, and deployment still report an error for `swift` on other providers.

## Secrets and conditional facts

`MimirSecrets` declares S3 access/secret keys and a Swift application-credential secret. The conditional secret mapping associates the S3 pair with `storage_type=s3` and the Swift secret with `storage_type=swift`.

## Rendering

The catalog uses source group `observability`, base path `applications/base/services/observability/mimir`, stages `observability-namespace`, `observability-sources`, and `mimir-override`, and an override dependency on `sources`. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable mimir
opencenter cluster service disable mimir
opencenter cluster service options mimir
```
