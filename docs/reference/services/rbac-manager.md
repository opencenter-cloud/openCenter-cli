---
id: service-rbac-manager
title: "RBAC Manager"
sidebar_label: RBAC Manager
description: Declarative RBAC management using RBACDefinition custom resources.
doc_type: reference
audience: "platform engineers, security engineers"
tags: [rbac, security, access-control]
---

> **Purpose:** For platform engineers, documents the RBAC Manager service for declarative role binding management via CRDs.

## Overview

RBAC Manager simplifies Kubernetes RBAC by providing an `RBACDefinition` CRD that declaratively manages RoleBindings and ClusterRoleBindings. Instead of manually creating individual bindings, operators define desired access in a single resource and RBAC Manager reconciles the bindings automatically. When integrated with Keycloak, group-based access control maps identity provider groups to Kubernetes RBAC roles.

## Configuration

```yaml
opencenter:
  services:
    rbac-manager:
      enabled: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable RBAC Manager |

### Secrets

None.

## Dependencies

| Service | Required | Notes |
|---------|----------|-------|
| keycloak | Optional | Enables group-based access from identity provider |

## CLI Commands

```bash
opencenter cluster service enable rbac-manager
opencenter cluster service disable rbac-manager
opencenter cluster service status rbac-manager
opencenter cluster service options rbac-manager
```
