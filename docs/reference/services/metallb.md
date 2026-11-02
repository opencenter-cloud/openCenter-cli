---
last_updated: 2026-11-02
id: service-metallb
title: "MetalLB"
sidebar_label: MetalLB
description: MetalLB service configuration and generated pool and L2 advertisement files.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, load-balancer, metallb, services]
---

> **Evidence:** `internal/config/services/metallb.go`, `internal/config/v2/defaults.go`, and `internal/gitops/render_catalog.go`.

## Configuration

The generated default is disabled in `metallb-system`. `MetalLBConfig` adds IP address pools and L2 advertisements to `BaseConfig`.

```yaml
opencenter:
  services:
    metallb:
      enabled: false
      namespace: metallb-system
      ip_address_pools:
        - name:
          addresses: []
          default: false
          auto_assign: true
          avoid_buggy_ips: false
      l2_advertisements:
        - name:
          type: l2
          ip_address_pools: []
          interfaces: []
          node_selectors:
            - match_labels:
                kubernetes.io/hostname: worker-0
            - match_expressions:
                - key: node-role.kubernetes.io/worker
                  operator: Exists
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | `false` | `NewDefaultServiceConfig` |
| `namespace` | `metallb-system` | `NewDefaultServiceConfig` |
| `ip_address_pools[].name`, `addresses` | required fields | `IPAddressPool` |
| `ip_address_pools[].default` | false | `IPAddressPool.Default` |
| `ip_address_pools[].auto_assign` | true when omitted | `GetAutoAssign` |
| `ip_address_pools[].avoid_buggy_ips` | false | `IPAddressPool` |
| `l2_advertisements[].name` | required | `L2Advertisement` |
| `l2_advertisements[].type` | `l2` when omitted | `GetType` |
| `l2_advertisements[].ip_address_pools`, `interfaces` | empty | `L2Advertisement` |
| `l2_advertisements[].node_selectors` | empty (all nodes eligible) | `L2Advertisement.NodeSelectors` |

`DefaultPoolName` selects the explicitly marked pool, otherwise the first pool, otherwise an empty string.

### Node selectors

`node_selectors` restricts which nodes advertise a pool's load-balancer IPs. It renders to `spec.nodeSelectors` on the generated `L2Advertisement`, mirroring a Kubernetes label selector. Use it when only some nodes carry the configured `interfaces` (for example, to keep a remote GPU node from being elected to advertise a Gateway IP). When omitted, every node is eligible.

Each selector accepts `match_labels` (all key/value pairs must match) and/or `match_expressions` (`key`, `operator` one of `In`/`NotIn`/`Exists`/`DoesNotExist`, and `values`). `In`/`NotIn` require `values`; `Exists`/`DoesNotExist` must omit `values`.

```yaml
l2_advertisements:
  - name: private-pool-l2
    ip_address_pools: [private-pool]
    interfaces: [mgmt.102]
    node_selectors:
      - match_labels:
          kubernetes.io/hostname: rackai-dev-wrk-0
      - match_labels:
          kubernetes.io/hostname: rackai-dev-wrk-1
```

## Rendering

The built-in catalog uses an overlay-files renderer for MetalLB and sets no `GeneratedResourceFiles` for it. `metallbOverlayFilesRenderer` writes `ipaddresspool.yaml` and `l2advertisement.yaml`, and the auto-descriptor appends whatever filenames that renderer returns to the overlay Kustomization's resources list. No explicit service descriptor is present.

## Dependencies

No dependency is recorded for MetalLB in the plugin registry or catalog.

## Commands

```bash
opencenter cluster service enable metallb
opencenter cluster service disable metallb
opencenter cluster service options metallb
```
