---
last_updated: 2026-11-02
id: service-sources
title: "Sources"
sidebar_label: Sources
description: Sources service configuration and descriptor-owned Flux source aggregates.
doc_type: reference
audience: "platform engineers, contributors"
tags: [gitops, flux, sources, services]
---

> **Evidence:** `internal/config/services/default_services.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/registry.go`, `internal/gitops/render_catalog.go`, `internal/services/descriptors/data/services-sources-aggregate.yaml`, and `internal/services/descriptors/data/managed-services-sources-aggregate.yaml`.

## Configuration

Sources uses `DefaultServiceConfig`; the generated default is enabled in `flux-system`.

```yaml
opencenter:
  services:
    sources:
      enabled: true
      namespace: flux-system
      adoption_mode: managed
      address_pool:
```

## Rendering

`auto_descriptor.go` treats Sources as structural. The catalog entry uses source name `opencenter-sources`, source group `sources`, and base path `applications/base/services/sources`. The explicit services and managed-services aggregate descriptors each list only the sources Kustomization template (`services/sources/kustomization.yaml.tpl` and `managed-services/sources/kustomization.yaml.tpl`); the per-service `opencenter-*.yaml.tpl` GitRepository objects in those directories are owned elsewhere — by the service descriptors that list them (`service-cert-manager`, `service-harbor`, `service-keycloak`, `service-olm`, `service-kafka-cluster`, `service-alert-proxy`, and the always-on `observability-shared`), or, for catalog-owned services, by the auto-descriptor source action. No service-specific fields are registered.

## Commands

```bash
opencenter cluster service enable sources
opencenter cluster service disable sources
opencenter cluster service options sources
```
