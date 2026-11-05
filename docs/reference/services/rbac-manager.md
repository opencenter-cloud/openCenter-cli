---
last_updated: 2026-09-25
id: service-rbac-manager
title: "RBAC Manager"
sidebar_label: RBAC Manager
description: RBAC Manager service configuration and built-in catalog rendering.
doc_type: reference
audience: "platform engineers, security engineers"
tags: [rbac, security, services]
---
# RBAC Manager

**Purpose:** For platform engineers, security engineers, documents RBAC Manager service configuration and built-in catalog rendering.

> **Evidence:** `internal/config/services/default_services.go`, `internal/config/v2/defaults.go`, and `internal/gitops/render_catalog.go`.

## Configuration

RBAC Manager uses `DefaultServiceConfig`; the generated default is enabled in `rbac-system`.

```yaml
opencenter:
  services:
    rbac-manager:
      enabled: true
      namespace: rbac-system
      adoption_mode: managed
      address_pool:
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | `true` | `NewDefaultServiceConfig` |
| `namespace` | `rbac-system` | `NewDefaultServiceConfig` |
| common fields | — | `BaseConfig` |

## Rendering

The catalog marks RBAC Manager as base-only and records a conditional dependency on `kube-prometheus-stack-base` when `kube-prometheus-stack` is enabled. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable rbac-manager
opencenter cluster service disable rbac-manager
opencenter cluster service options rbac-manager
```
