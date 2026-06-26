---
id: service-kyverno
title: "Kyverno"
sidebar_label: Kyverno
description: Kubernetes-native policy engine for validation, mutation, and generation of resources.
doc_type: reference
audience: "platform engineers, security engineers"
tags: [policy, security, admission-control]
---

> **Purpose:** For platform engineers and security engineers, documents the Kyverno policy engine and its default ClusterPolicies.

## Overview

Kyverno is a Kubernetes-native policy engine that validates, mutates, and generates resources using policies written as Kubernetes resources. When enabled, openCenter deploys 17 default ClusterPolicies enforcing security best practices including container privilege restrictions, host namespace isolation, and volume type controls. Policies can be customized or extended via standard Kyverno ClusterPolicy resources.

## Configuration

```yaml
opencenter:
  services:
    kyverno:
      enabled: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable Kyverno policy engine |

### Default ClusterPolicies

- `disallow-privileged-containers`
- `disallow-host-namespaces`
- `disallow-host-path`
- `disallow-host-ports`
- `disallow-host-process`
- `disallow-privilege-escalation`
- `disallow-capabilities`
- `disallow-selinux`
- `disallow-proc-mount`
- `require-run-as-nonroot`
- `require-run-as-non-root-user`
- `require-default-seccomp`
- `restrict-seccomp`
- `restrict-sysctls`
- `restrict-volume-types`
- `restrict-apparmor-profiles`
- `restrict-image-registries`

### Secrets

None.

## Dependencies

None.

## CLI Commands

```bash
opencenter cluster service enable kyverno
opencenter cluster service disable kyverno
opencenter cluster service status kyverno
opencenter cluster service options kyverno
```
