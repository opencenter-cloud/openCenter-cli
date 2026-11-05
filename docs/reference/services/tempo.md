---
last_updated: 2026-09-25
id: service-tempo
title: "Tempo"
sidebar_label: Tempo
description: Tempo service configuration, runtime storage validation, and catalog ownership.
doc_type: reference
audience: "operators, platform engineers"
tags: [tempo, tracing, observability, services]
---
# Tempo

**Purpose:** For operators, platform engineers, documents Tempo service configuration, runtime storage validation, and catalog ownership.

> **Evidence:** `internal/config/services/tempo.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/tempo.go`, `internal/config/services/secrets_validator.go`, `internal/config/services/provider_registry.go`, and `internal/gitops/render_catalog.go`.

## Configuration

The generated default is enabled in `observability`. `TempoConfig` embeds `BaseConfig` and exposes `storage_type`, `bucket_name`, `volume_size`, `storage_class`, S3 fields (`s3_endpoint`, `s3_region`, `s3_credential_id`, `s3_force_path_style`, `s3_insecure`), and retained `swift_*` fields.

The config metadata documents `s3` and `swift`, with `s3` as the type-level default. `defaults.go` materializes only enabled state and namespace. The provider registry separately selects `s3` for Tempo.

## Runtime validation and secrets

When enabled and `storage_type` is set, the plugin accepts the values at its first check as `s3` or `swift`, then returns an error for `swift`. For `s3`, it requires both `s3_endpoint` and `bucket_name`. `TempoSecrets` declares `access_key`, `secret_key`, and a retained Swift credential field; the conditional secret mapping records S3 keys for `s3`.

This page records the repository's validation behavior only; it makes no claim about an external component's versions or implementation.

## Rendering

The catalog uses observability base path `applications/base/services/observability/tempo`, stages `observability-namespace`, `observability-sources`, and `tempo-override`, and an override dependency on `sources`. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable tempo
opencenter cluster service disable tempo
opencenter cluster service options tempo
```
