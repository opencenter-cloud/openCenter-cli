---
id: service-sealed-secrets
title: "Sealed Secrets"
sidebar_label: Sealed Secrets
description: Encrypted Kubernetes secrets safe for storage in Git repositories.
doc_type: reference
audience: "platform engineers, operators"
tags: [secrets, encryption, gitops]
---

> **Purpose:** For platform engineers, documents the Sealed Secrets service for encrypting Kubernetes secrets with asymmetric cryptography.

## Overview

Sealed Secrets enables encrypting Kubernetes Secret objects so they can be safely stored in version control. A cluster-side controller decrypts SealedSecret resources into regular Secrets. The asymmetric encryption ensures that only the controller with the private key can decrypt the values, making sealed secrets safe to commit alongside application manifests.

## Configuration

```yaml
opencenter:
  services:
    sealed-secrets:
      enabled: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable Sealed Secrets controller |

### Secrets

None.

## Dependencies

None.

## CLI Commands

```bash
opencenter cluster service enable sealed-secrets
opencenter cluster service disable sealed-secrets
opencenter cluster service status sealed-secrets
opencenter cluster service options sealed-secrets
```
