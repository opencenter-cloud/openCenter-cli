---
last_updated: 2026-11-02
id: service-loki
title: "Loki"
sidebar_label: Loki
description: Loki service configuration, storage validation, secrets, and catalog ownership.
doc_type: reference
audience: "operators, platform engineers"
tags: [loki, logging, observability, services]
---

> **Evidence:** `internal/config/services/loki.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/loki.go`, `internal/config/services/secrets_validator.go`, `internal/config/services/provider_registry.go`, and `internal/gitops/render_catalog.go`.

## Configuration

The generated default is enabled in `observability`. `LokiConfig` embeds `BaseConfig` and exposes storage fields:

`storage_type`, `bucket_name`, `volume_size`, `storage_class`, `swift_auth_url`, `swift_region`, `swift_auth_version`, `swift_username`, `swift_project_name`, `swift_project_domain_name`, `swift_container_name`, `swift_user_domain_name`, `swift_domain_name`, `swift_application_credential_id`, `s3_endpoint`, `s3_region`, `s3_credential_id`, `s3_force_path_style`, and `s3_insecure`.

The config schema metadata documents `storage_type` values `s3`, `swift`, and `none`, with `swift` as its type-level default. `defaults.go` materializes only `enabled: true` and `namespace: observability`; the provider registry separately selects `s3` for Loki.

An unset `storage_type` resolves to `swift`, the type-level default in `LokiConfig`; the registry's `s3` selection is a separate compatibility record and does not rewrite the field. The override-values renderer takes its backend from `objectStorageBackend "loki"`, which returns `swift` for an unset or `swift` value and `s3` for anything else, so an unset field renders a Swift-backed Loki. Loki's own plugin validates `storage_type` only when it is set, so it never catches the unset case. Swift is no longer supported for platform bulk data: `cluster validate` rejects an explicit `swift` with `Swift is no longer supported for platform bulk data; migrate to the S3-compatible storage profile.`, and readiness for a resolved Swift backend requires `secrets.loki.swift_application_credential_secret` instead of the S3 endpoint and key pair. Set `storage_type: s3` explicitly.

## Runtime validation and secrets

When enabled and `storage_type` is set, the plugin accepts `s3`, `swift`, or `none`. It requires `swift_auth_url` for `swift` and `s3_endpoint` for `s3`; `none` returns without those checks. `LokiSecrets` declares S3 access/secret keys and Swift password/application-credential fields. The conditional secret mapping records S3 keys for `s3` and `swift_password` for `swift`.

## Rendering

The catalog uses observability base path `applications/base/services/observability/loki`, stages `observability-namespace`, `observability-sources`, and `loki-override`, and an override dependency on `sources`. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable loki
opencenter cluster service disable loki
opencenter cluster service options loki
```
