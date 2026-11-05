---
last_updated: 2026-09-25
id: service-sealed-secrets
title: "Sealed Secrets"
sidebar_label: Sealed Secrets
description: Sealed Secrets service configuration and built-in catalog stages.
doc_type: reference
audience: "platform engineers, operators"
tags: [secrets, security, services]
---
# Sealed Secrets

**Purpose:** For platform engineers, operators, documents sealed Secrets service configuration and built-in catalog stages.

> **Evidence:** `internal/config/services/default_services.go`, `internal/config/v2/defaults.go`, and `internal/gitops/render_catalog.go`.

## Configuration

Sealed Secrets uses `DefaultServiceConfig`; the generated default is disabled in `sealed-secrets`.

```yaml
opencenter:
  services:
    sealed-secrets:
      enabled: false
      namespace: sealed-secrets
      adoption_mode: managed
      address_pool:
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | `false` | `NewDefaultServiceConfig` |
| `namespace` | `sealed-secrets` | `NewDefaultServiceConfig` |
| common fields | — | `BaseConfig` |

## Rendering

The catalog gives Sealed Secrets namespace `sealed-secrets`, override values `keyrenewperiod: "0"`, an extra `sealed-secrets-override` stage, and override dependencies on `sources` and `sealed-secrets-namespace`. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable sealed-secrets
opencenter cluster service disable sealed-secrets
opencenter cluster service options sealed-secrets
```
