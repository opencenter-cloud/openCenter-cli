---
last_updated: 2026-11-02
id: services-index
title: "Platform Services"
sidebar_label: Services
description: Repository-backed directory of built-in platform services, generated defaults, and configuration references.
doc_type: reference
audience: "operators, platform engineers"
tags: [services, platform, reference]
---

# Platform services

> **Scope:** This index covers the service names in `internal/config/v2/defaults.go`, the built-in plugin registry, and the managed `alert-proxy` entry. Defaults come from `internal/config/v2/defaults.go`; service-specific fields come from registered Go config types. It does not infer versions or upstream behavior.

Standard services are configured under `opencenter.services.<name>`. The descriptor for `alert-proxy` uses `opencenter.managed_services.alert-proxy`. All registered service configs embed the common fields in `internal/config/services/base.go`; see [Platform services architecture](../platform-services.md) for the surrounding configuration maps.

“Default” means the value returned by `NewDefaultServiceConfig` in `internal/config/v2/defaults.go`. **Not in default map** means the service is registered as a plugin but is not materialized by that default map.

## Networking

| Service | Generated default | Repository fact |
|---------|-------------------|-----------------|
| [calico](calico.md) | Enabled (`calico-system`) | Explicit descriptor owns `services/calico` |
| [cilium](cilium.md) | Not in default map | Registered networking plugin |
| [kube-ovn](kube-ovn.md) | Not in default map | Registered networking plugin |
| [gateway-api](gateway-api.md) | Enabled (`envoy-gateway-system`) | Built-in catalog entry named `envoy-gateway-api` |
| [gateway](gateway.md) | Enabled (`gateway`) | Built-in single-stage catalog entry |
| [metallb](metallb.md) | Disabled (`metallb-system`) | Built-in catalog entry with overlay rendering |

## Security

| Service | Generated default | Repository fact |
|---------|-------------------|-----------------|
| [cert-manager](cert-manager.md) | Enabled (`cert-manager`) | Explicit descriptor |
| [keycloak](keycloak.md) | Enabled (`keycloak`) | Explicit descriptor |
| [kyverno](kyverno.md) | Enabled (`kyverno`) | Built-in base-only catalog entry |
| [rbac-manager](rbac-manager.md) | Enabled (`rbac-system`) | Built-in base-only catalog entry |
| [sealed-secrets](sealed-secrets.md) | Disabled (`sealed-secrets`) | Built-in catalog entry with override stages |

## Storage

| Service | Generated default | Repository fact |
|---------|-------------------|-----------------|
| [openstack-ccm](openstack-ccm.md) | Enabled (`openstack-ccm`) | Built-in catalog entry |
| [openstack-csi](openstack-csi.md) | Enabled (`openstack-csi`) | Built-in catalog entry |
| [vsphere-csi](vsphere-csi.md) | Disabled (`vmware-system-csi`) | Built-in catalog entry plus storage-class config |
| [longhorn](longhorn.md) | Disabled (`longhorn-system`) | Built-in catalog entry plus storage config |
| [external-snapshotter](external-snapshotter.md) | Enabled (`external-snapshotter`) | Built-in base-only catalog entry |

## Observability

| Service | Generated default | Repository fact |
|---------|-------------------|-----------------|
| [kube-prometheus-stack](kube-prometheus-stack.md) | Enabled (`observability`) | Registered config plus catalog entry |
| [loki](loki.md) | Enabled (`observability`) | Registered config plus catalog entry |
| [tempo](tempo.md) | Enabled (`observability`) | Registered config plus catalog entry |
| [mimir](mimir.md) | Disabled (`observability`) | Registered config plus catalog entry |
| [opentelemetry-kube-stack](opentelemetry-kube-stack.md) | Disabled (`observability`) | Registered config plus catalog entry |
| [alert-proxy](alert-proxy.md) | Disabled (managed map) | Managed-service descriptor |

## GitOps

| Service | Generated default | Repository fact |
|---------|-------------------|-----------------|
| [fluxcd](fluxcd.md) | Enabled (`flux-system`) | Structural built-in catalog entry |
| [sources](sources.md) | Enabled (`flux-system`) | Structural source aggregate |
| [weave-gitops](weave-gitops.md) | Disabled (`flux-system`) | Built-in catalog entry |

## Backup

| Service | Generated default | Repository fact |
|---------|-------------------|-----------------|
| [velero](velero.md) | Enabled (`velero`) | Registered backup config and catalog entry |
| [etcd-backup](etcd-backup.md) | Disabled (`kube-system`) | Explicit descriptor with conditional root |

## Management

| Service | Generated default | Repository fact |
|---------|-------------------|-----------------|
| [headlamp](headlamp.md) | Enabled (`headlamp`) | Registered config and catalog entry |
| [olm](olm.md) | Enabled (`olm`) | Explicit descriptor |
| [postgres-operator](postgres-operator.md) | Enabled (`postgres-operator`) | Built-in catalog entry |
| [harbor](harbor.md) | Disabled (`harbor`) | Explicit descriptor with conditional Certificate file |
| [kafka-cluster](kafka-cluster.md) | Disabled (`kafka-system`) | Explicit descriptor with Strimzi templates |

## Common operations

```bash
opencenter cluster service status
opencenter cluster service enable <service> [--param key=value] [--secret key=value]
opencenter cluster service disable <service>
opencenter cluster service options <service>
```

## Storage provider registry

`internal/config/services/provider_registry.go` selects `s3` as the default provider for `loki`, `velero`, and `tempo` for every listed infrastructure provider. Its compatibility matrix additionally accepts `none` for Loki and Velero and `filesystem` for Tempo, and rejects Swift, GCS, and Azure for these service entries. Individual service config types expose their own fields; see each page for runtime validation.

| Infrastructure provider | Registry default |
|-------------------------|------------------|
| OpenStack | `s3` |
| AWS | `s3` |
| GCP | `s3` |
| Azure | `s3` |
| Bare-metal / vSphere | `s3` |

## Related documentation

- [Platform services architecture](../platform-services.md) — configuration maps and render ownership.
