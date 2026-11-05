---
last_updated: 2026-09-25
id: service-longhorn
title: "Longhorn"
sidebar_label: Longhorn
description: Longhorn service configuration and built-in catalog rendering.
doc_type: reference
audience: "platform engineers, storage administrators"
tags: [storage, longhorn, services]
---
# Longhorn

**Purpose:** For platform engineers, storage administrators, documents longhorn service configuration and built-in catalog rendering.

> **Evidence:** `internal/config/services/longhorn.go`, `internal/config/v2/defaults.go`, and `internal/gitops/render_catalog.go`.

## Configuration

The generated default is disabled in `longhorn-system`; its generated hostname is `longhorn.<cluster FQDN>`. `LonghornConfig` adds hostname, replica, storage, and backup fields to `BaseConfig`.

| Field | Default in config metadata | Evidence |
|-------|----------------------------|----------|
| `enabled` | `false` | `NewDefaultServiceConfig` |
| `namespace` | `longhorn-system` | `NewDefaultServiceConfig` |
| `hostname` | generated from cluster FQDN | `NewDefaultServiceConfig` |
| `default_replica_count` | `3` | `LonghornConfig` schema tag |
| `default_data_path` | `/var/lib/longhorn` | `LonghornConfig` schema tag |
| `storage_over_provisioning_percentage` | `200` | `LonghornConfig` schema tag |
| `storage_minimal_available_percentage` | `25` | `LonghornConfig` schema tag |
| `backup_target`, `backup_target_credential_secret` | empty | `LonghornConfig` |

Common `BaseConfig` fields (`adoption_mode`, `source`, `image`, `address_pool`) are also available.

## Rendering

The catalog uses base path `applications/base/services/longhorn`, an overlay-files renderer, override values `persistence.defaultClass: false`, and override dependencies `sources`, `longhorn-base`, and `envoy-gateway-api-base`. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable longhorn
opencenter cluster service disable longhorn
opencenter cluster service options longhorn
```
