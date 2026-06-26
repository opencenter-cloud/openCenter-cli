---
id: service-fluxcd
title: "FluxCD"
sidebar_label: FluxCD
description: GitOps continuous delivery engine for Kubernetes.
doc_type: reference
audience: "platform engineers, operators"
tags: [gitops, flux, continuous-delivery, core]
---

> **Purpose:** For platform engineers, documents the FluxCD core service that provides GitOps reconciliation for all cluster resources.

## Overview

FluxCD is the core GitOps engine that reconciles cluster state with the generated GitOps repository. It manages GitRepository sources, Kustomization resources, and HelmRelease objects to ensure the cluster converges to the declared state. FluxCD is a foundational service—other services depend on it for deployment and lifecycle management. Disabling FluxCD breaks GitOps functionality for the entire cluster.

## Configuration

```yaml
opencenter:
  services:
    fluxcd:
      enabled: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable FluxCD (core service, always enabled) |

> **Note:** FluxCD is a core service. Disabling it prevents GitOps reconciliation for all other services.

### Secrets

None.

## Dependencies

None.

## CLI Commands

```bash
opencenter cluster service enable fluxcd
opencenter cluster service disable fluxcd
opencenter cluster service status fluxcd
opencenter cluster service options fluxcd
```
