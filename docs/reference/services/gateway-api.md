---
last_updated: 2026-09-25
id: service-gateway-api
title: "Gateway API"
sidebar_label: Gateway API
description: Gateway API service configuration and built-in catalog rendering.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, gateway-api, services]
---
# Gateway API

**Purpose:** For platform engineers, operators, documents gateway API service configuration and built-in catalog rendering.

> **Evidence:** `internal/config/services/default_services.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/registry.go`, and `internal/gitops/render_catalog.go`.

## Configuration

`gateway-api` uses `DefaultServiceConfig`; its service-specific configuration is the common `BaseConfig`. The generated default is enabled in `envoy-gateway-system`.

```yaml
opencenter:
  services:
    gateway-api:
      enabled: true
      namespace: envoy-gateway-system
      adoption_mode: managed
      address_pool:
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | `true` | `NewDefaultServiceConfig` |
| `namespace` | `envoy-gateway-system` | `NewDefaultServiceConfig` |
| common fields | — | `BaseConfig` |

## Runtime and rendering

The plugin registry records no dependency for `gateway-api`. The catalog entry uses Kustomization name `envoy-gateway-api` and override values containing logging level `info`. No service descriptor file names `gateway-api`.

## Commands

```bash
opencenter cluster service enable gateway-api
opencenter cluster service disable gateway-api
opencenter cluster service options gateway-api
```
