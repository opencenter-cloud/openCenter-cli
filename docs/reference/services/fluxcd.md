---
last_updated: 2026-11-02
id: service-fluxcd
title: "FluxCD"
sidebar_label: FluxCD
description: FluxCD service configuration and structural catalog ownership.
doc_type: reference
audience: "platform engineers, operators"
tags: [gitops, flux, services]
---

> **Evidence:** `internal/config/services/default_services.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/registry.go`, `internal/gitops/render_catalog.go`, and `internal/services/descriptors/data/*-fluxcd-aggregate.yaml`.

## Configuration

FluxCD uses `DefaultServiceConfig`; the generated default is enabled in `flux-system`.

```yaml
opencenter:
  services:
    fluxcd:
      enabled: true
      namespace: flux-system
      adoption_mode: managed
      address_pool:
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | `true` | `NewDefaultServiceConfig` |
| `namespace` | `flux-system` | `NewDefaultServiceConfig` |
| common fields | — | `BaseConfig` |

## Rendering

The catalog gives FluxCD an override-values stage and base path `applications/base/services/fluxcd`. `auto_descriptor.go` treats it as structural. Descriptor facts are split across the services and managed-services Flux aggregate descriptors, which list the Kustomization, sources, monitoring, and Flux config templates; `root-overlay.yaml` is the root-layer aggregate and lists only `kustomization.yaml` and the conditional `.sops.yaml.tpl`, not FluxCD service templates. The plugin registry records no dependency.

## Commands

```bash
opencenter cluster service enable fluxcd
opencenter cluster service disable fluxcd
opencenter cluster service options fluxcd
```
