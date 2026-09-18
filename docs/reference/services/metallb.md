---
id: service-metallb
title: "MetalLB"
sidebar_label: MetalLB
description: Bare-metal load balancer providing external IP addresses for Kubernetes services.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, load-balancer, bare-metal]
---

> **Purpose:** For platform engineers, documents MetalLB service configuration, covering IP pool management, L2 advertisement, and BGP mode.

## Overview

MetalLB provides network load-balancer implementation for Kubernetes clusters that do not run on a cloud provider, giving bare-metal clusters access to `LoadBalancer`-type Services. It supports L2 advertisement mode for simple deployments and BGP mode for production routing, with configurable IP address pools that control which addresses are assigned to services.

## Configuration

```yaml
services:
  metallb:
    enabled: true
    namespace: metallb-system
    ip_address_pools:
      - name: public-pool
        addresses:
          - 72.4.119.48/28
      - name: private-pool
        default: true
        addresses:
          - 10.97.6.61/32
    l2_advertisements:
      - name: public-pool-l2
        type: l2
        ip_address_pools:
          - public-pool
        interfaces:
          - metal.105
```

The `services:` fragment above belongs in the cluster configuration under `opencenter:` when it is part of a complete file.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable MetalLB service |
| `namespace` | string | `metallb-system` | Namespace for generated MetalLB resources |
| `ip_address_pools` | list | — | List of IP address pool definitions |
| `ip_address_pools[].name` | string | — | Pool identifier |
| `ip_address_pools[].addresses` | list of strings | — | IP ranges in `start-end` or CIDR format |
| `ip_address_pools[].default` | bool | `false` | Mark this pool as the default other CLI features select when a service declares no explicit pool. At most one pool may set `default: true`; when none is marked, the first pool is treated as default |
| `ip_address_pools[].auto_assign` | bool | `true` when omitted | Automatically assign IPs from this pool |
| `ip_address_pools[].avoid_buggy_ips` | bool | `false` | Avoid `.0` and `.255` addresses |
| `l2_advertisements` | list | — | Layer-2 advertisements to generate |
| `l2_advertisements[].name` | string | — | Advertisement identifier |
| `l2_advertisements[].type` | string | `l2` | Advertisement type. Only `l2` is supported today; BGP advertisements are a follow-up |
| `l2_advertisements[].ip_address_pools` | list of strings | all pools when empty | Pools selected by the advertisement; an empty list selects all pools per MetalLB semantics |
| `l2_advertisements[].interfaces` | list of strings | — | Node interfaces on which to advertise |

If `l2_advertisements` is omitted, openCenter generates no L2 advertisements; it does not invent a default. An advertisement with an empty `ip_address_pools` selects all pools according to MetalLB's own semantics.

### Secrets

None.

## Dependencies

None.

## CLI Commands

```bash
opencenter cluster service enable metallb
opencenter cluster service disable metallb
opencenter cluster service status metallb
opencenter cluster service options metallb
```
