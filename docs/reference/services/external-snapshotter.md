---
last_updated: 2026-09-25
id: service-external-snapshotter
title: "External Snapshotter"
sidebar_label: External Snapshotter
description: External snapshotter service configuration and built-in catalog ownership.
doc_type: reference
audience: "platform engineers, storage administrators"
tags: [storage, csi, snapshots, services]
---
# External Snapshotter

**Purpose:** For platform engineers, storage administrators, documents external snapshotter service configuration and built-in catalog ownership.

> **Evidence:** `internal/config/services/default_services.go`, `internal/config/v2/defaults.go`, and `internal/gitops/render_catalog.go`.

## Configuration

External Snapshotter uses `DefaultServiceConfig`; the generated default is enabled in `external-snapshotter`.

```yaml
opencenter:
  services:
    external-snapshotter:
      enabled: true
      namespace: external-snapshotter
      adoption_mode: managed
      address_pool:
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | `true` | `NewDefaultServiceConfig` |
| `namespace` | `external-snapshotter` | `NewDefaultServiceConfig` |
| common fields | — | `BaseConfig` |

## Rendering

The catalog marks External Snapshotter as base-only with base path `applications/base/services/external-snapshotter`. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable external-snapshotter
opencenter cluster service disable external-snapshotter
opencenter cluster service options external-snapshotter
```
