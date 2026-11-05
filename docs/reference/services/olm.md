---
last_updated: 2026-09-25
id: service-olm
title: "Operator Lifecycle Manager"
sidebar_label: OLM
description: OLM service configuration and explicit descriptor templates.
doc_type: reference
audience: "platform engineers, operators"
tags: [operators, lifecycle, olm, services]
---
# Operator Lifecycle Manager

**Purpose:** For platform engineers, operators, documents OLM service configuration and explicit descriptor templates.

> **Evidence:** `internal/config/services/default_services.go`, `internal/config/v2/defaults.go`, `internal/services/descriptors/data/service-olm.yaml`, and `internal/services/plugins/registry.go`.

## Configuration

OLM uses `DefaultServiceConfig`; the generated default is enabled in `olm`.

```yaml
opencenter:
  services:
    olm:
      enabled: true
      namespace: olm
      adoption_mode: managed
      address_pool:
```

## Runtime and descriptor facts

The plugin registry records no OLM dependency. `service-olm.yaml` aggregates into `services-fluxcd-aggregate` and `services-sources-aggregate` and lists source objects, OLM Kustomization, bundle-unpack NetworkPolicy, and Flux templates. No conditional file is declared.

## Commands

```bash
opencenter cluster service enable olm
opencenter cluster service disable olm
opencenter cluster service options olm
```
