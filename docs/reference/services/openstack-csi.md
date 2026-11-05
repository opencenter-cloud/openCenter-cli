---
last_updated: 2026-09-25
id: service-openstack-csi
title: "OpenStack Cinder CSI"
sidebar_label: OpenStack CSI
description: OpenStack CSI service configuration and catalog ownership.
doc_type: reference
audience: "platform engineers, storage administrators"
tags: [openstack, storage, csi, services]
---
# OpenStack Cinder CSI

**Purpose:** For platform engineers, storage administrators, documents OpenStack CSI service configuration and catalog ownership.

> **Evidence:** `internal/config/services/default_services.go`, `internal/config/v2/defaults.go`, and `internal/gitops/render_catalog.go`.

## Configuration

`openstack-csi` uses `DefaultServiceConfig`. The generated default is enabled in `openstack-csi`.

```yaml
opencenter:
  services:
    openstack-csi:
      enabled: true
      namespace: openstack-csi
      adoption_mode: managed
      address_pool:
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | `true` | `NewDefaultServiceConfig` |
| `namespace` | `openstack-csi` | `NewDefaultServiceConfig` |
| common fields | — | `BaseConfig` |

## Rendering

The catalog entry marks OpenStack CSI as namespace-stage and privileged, with base path `applications/base/services/openstack-csi`, override values, and override dependencies `sources` and `openstack-csi-namespace`. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable openstack-csi
opencenter cluster service disable openstack-csi
opencenter cluster service options openstack-csi
```
