---
id: service-weave-gitops
title: "Weave GitOps"
sidebar_label: Weave GitOps
description: Web dashboard for visualizing and managing FluxCD GitOps resources.
doc_type: reference
audience: "platform engineers, operators"
tags: [gitops, dashboard, flux]
---

> **Purpose:** For platform engineers, documents the Weave GitOps dashboard for Flux resource visualization and reconciliation management.

## Overview

Weave GitOps provides a web-based dashboard for visualizing and managing FluxCD GitOps resources. It displays GitRepository sources, Kustomizations, HelmReleases, and their reconciliation status in a unified interface. The dashboard requires authentication via a password configured in the cluster secrets.

## Configuration

```yaml
opencenter:
  services:
    weave-gitops:
      enabled: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable Weave GitOps dashboard |

### Secrets

```yaml
secrets:
  weave_gitops:
    password: <plaintext-password>
    password_hash: <bcrypt-hash>
```

| Secret | Description |
|--------|-------------|
| `secrets.weave_gitops.password` | Dashboard login password |
| `secrets.weave_gitops.password_hash` | Bcrypt hash of the password |

## Dependencies

None (FluxCD is implicitly available as a core service).

## CLI Commands

```bash
opencenter cluster service enable weave-gitops
opencenter cluster service disable weave-gitops
opencenter cluster service status weave-gitops
opencenter cluster service options weave-gitops
```
