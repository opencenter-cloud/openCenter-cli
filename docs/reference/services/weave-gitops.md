---
last_updated: 2026-09-25
id: service-weave-gitops
title: "Weave GitOps"
sidebar_label: Weave GitOps
description: Weave GitOps service configuration, dependency records, and catalog ownership.
doc_type: reference
audience: "platform engineers, operators"
tags: [gitops, services]
---
# Weave GitOps

**Purpose:** For platform engineers, operators, documents weave GitOps service configuration, dependency records, and catalog ownership.

> **Evidence:** `internal/config/services/default_services.go`, `internal/config/v2/defaults.go`, `internal/config/v2/config.go`, `internal/config/services/dependency_validator.go`, `internal/services/plugins/registry.go`, and `internal/gitops/render_catalog.go`.

## Configuration

Weave GitOps uses `DefaultServiceConfig`; the generated default is disabled in `flux-system`. `WeaveGitOpsSecrets` declares `password` and `password_hash`.

```yaml
opencenter:
  services:
    weave-gitops:
      enabled: false
      namespace: flux-system
      adoption_mode: managed
      address_pool:
```

## Runtime and rendering

Both the config dependency graph and plugin registry record `fluxcd` for Weave GitOps. The catalog uses namespace `flux-system`, base path `applications/base/services/weave-gitops`, and override dependencies `sources` and `envoy-gateway-api-base`. No explicit service descriptor is present.

## Commands

```bash
opencenter cluster service enable weave-gitops
opencenter cluster service disable weave-gitops
opencenter cluster service options weave-gitops
```
