---
last_updated: 2026-09-25
id: service-kube-ovn
title: "Kube-OVN"
sidebar_label: Kube-OVN
description: Kube-OVN service configuration and registered plugin validation.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, cni, kube-ovn, services]
---
# Kube-OVN

**Purpose:** For platform engineers, operators, documents Kube-OVN service configuration and registered plugin validation.

> **Evidence:** `internal/config/services/kube_ovn.go`, `internal/services/plugins/kube_ovn.go`, and `internal/services/plugins/registry.go`.

## Configuration

`KubeOVNConfig` embeds `BaseConfig` and adds four fields. Kube-OVN is listed by `GetBuiltInServiceNames`, but is not materialized by `defaultServiceMap`.

```yaml
opencenter:
  services:
    kube-ovn:
      enabled: true
      namespace:
      cilium_integration: false
      default_subnet:
      version:
      enable_lb: false
      adoption_mode: managed
      address_pool:
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | not materialized by default map | `defaults.go` |
| `cilium_integration` | `false` in Go zero value | `KubeOVNConfig.CiliumIntegration` |
| `default_subnet` | empty | `KubeOVNConfig.DefaultSubnet` |
| `version` | empty | `KubeOVNConfig.Version` |
| `enable_lb` | `false` in Go zero value | `KubeOVNConfig.EnableLB` |
| common fields | — | `BaseConfig` |

## Runtime and rendering

The plugin registry records no Kube-OVN dependency. When enabled, `KubeOVNPlugin.Validate` requires a configured `version` to contain `.` and a configured `default_subnet` to contain `/`. `KubeOVNPlugin.Render` is a no-op. No Kube-OVN service descriptor or `RenderCatalog` entry exists.

## Commands

```bash
opencenter cluster service enable kube-ovn
opencenter cluster service disable kube-ovn
opencenter cluster service options kube-ovn
```
