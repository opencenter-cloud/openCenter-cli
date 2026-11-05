---
last_updated: 2026-09-25
id: service-kyverno
title: "Kyverno"
sidebar_label: Kyverno
description: Kyverno service configuration and built-in catalog rendering.
doc_type: reference
audience: "platform engineers, operators"
tags: [policy, security, services]
---
# Kyverno

**Purpose:** For platform engineers, operators, documents kyverno service configuration and built-in catalog rendering.

> **Evidence:** `internal/config/services/default_services.go`, `internal/config/v2/defaults.go`, and `internal/gitops/render_catalog.go`.

## Configuration

Kyverno uses `DefaultServiceConfig`, so it has the common `BaseConfig` fields only. The generated default is enabled in `kyverno`.

```yaml
opencenter:
  services:
    kyverno:
      enabled: true
      namespace: kyverno
      adoption_mode: managed
      address_pool:
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | `true` | `NewDefaultServiceConfig` |
| `namespace` | `kyverno` | `NewDefaultServiceConfig` |
| common fields | — | `BaseConfig` |

## Rendering

The catalog marks Kyverno as base-only and adds a post-base `kyverno-default-ruleset` stage depending on `sources` and `kyverno-base`. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable kyverno
opencenter cluster service disable kyverno
opencenter cluster service options kyverno
```
