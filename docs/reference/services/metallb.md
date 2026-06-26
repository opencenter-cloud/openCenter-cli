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
opencenter:
  services:
    metallb:
      enabled: true
      ip_address_pools:
        - name: default-pool
          addresses:
            - 192.168.1.240-192.168.1.250
          auto_assign: true
        - name: reserved-pool
          addresses:
            - 10.0.0.100-10.0.0.110
          auto_assign: false
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable MetalLB service |
| `ip_address_pools` | list | — | List of IP address pool definitions |
| `ip_address_pools[].name` | string | — | Pool identifier |
| `ip_address_pools[].addresses` | list of strings | — | IP ranges in `start-end` or CIDR format |
| `ip_address_pools[].auto_assign` | bool | `true` | Automatically assign IPs from this pool |

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
