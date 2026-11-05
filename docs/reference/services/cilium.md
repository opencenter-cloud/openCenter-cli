---
last_updated: 2026-09-25
id: service-cilium
title: "Cilium"
sidebar_label: Cilium
description: Cilium service configuration and registered plugin behavior.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, cni, cilium, services]
---
# Cilium

**Purpose:** For platform engineers, operators, documents cilium service configuration and registered plugin behavior.

> **Evidence:** `internal/config/services/cilium.go`, `internal/services/plugins/cilium.go`, and `internal/services/plugins/registry.go`.

## Configuration

`CiliumConfig` embeds `BaseConfig` and adds three fields. Cilium is listed by `GetBuiltInServiceNames`, but `internal/config/v2/defaults.go` does not materialize it in the generated default service map.

```yaml
opencenter:
  services:
    cilium:
      enabled: true
      namespace:
      operator_enabled: false
      kube_proxy_replacement: false
      module_source:
      adoption_mode: managed
      address_pool:
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | not materialized by default map | `defaults.go` |
| `operator_enabled` | `false` in Go zero value | `CiliumConfig.OperatorEnabled` |
| `kube_proxy_replacement` | `false` in Go zero value | `CiliumConfig.KubeProxyReplacement` |
| `module_source` | empty | `CiliumConfig.ModuleSource` |
| common fields | — | `BaseConfig` |

## Runtime and rendering

The plugin registry records no Cilium dependency. `CiliumPlugin.Validate` checks only the config type, and `CiliumPlugin.Render` is a no-op. The repository contains no Cilium service descriptor and no Cilium `RenderCatalog` entry.

## Commands

```bash
opencenter cluster service enable cilium
opencenter cluster service disable cilium
opencenter cluster service options cilium
```
