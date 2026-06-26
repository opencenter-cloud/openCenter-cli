---
id: services-index
title: "Platform Services"
sidebar_label: Services
description: Index of all platform services available in openCenter clusters.
doc_type: reference
audience: "operators, platform engineers"
tags: [services, platform, reference]
---

# Platform Services

> **Purpose:** For operators and platform engineers, indexes all platform services deployable with openCenter, organized by category.

## Service Matrix

| Service | Category | Default | Description | Details |
|---------|----------|---------|-------------|---------|
| [calico](calico.md) | Networking | Enabled | Calico CNI for pod networking with BGP support | |
| [gateway-api](gateway-api.md) | Networking | Enabled | Gateway API CRDs for modern ingress routing | |
| [gateway](gateway.md) | Networking | Enabled | Gateway API implementation (Envoy-based) | Depends: gateway-api |
| [metallb](metallb.md) | Networking | Enabled | Bare-metal load balancer using L2/BGP | |
| [cert-manager](cert-manager.md) | Security | Enabled | Automated TLS certificate management | Multi-provider DNS |
| [keycloak](keycloak.md) | Security | Enabled | Identity and access management (OIDC/SAML) | Depends: cert-manager, postgres-operator |
| [kyverno](kyverno.md) | Security | Enabled | Kubernetes policy engine | 17 default policies |
| [rbac-manager](rbac-manager.md) | Security | Enabled | Declarative RBAC management | |
| [sealed-secrets](sealed-secrets.md) | Security | Enabled | Encrypted Kubernetes secrets in Git | |
| [openstack-ccm](openstack-ccm.md) | Cloud | Enabled | OpenStack Cloud Controller Manager | OpenStack only |
| [openstack-csi](openstack-csi.md) | Storage | Enabled | OpenStack Cinder CSI driver | OpenStack only |
| [vsphere-csi](vsphere-csi.md) | Storage | Disabled | VMware vSphere CSI driver | VMware only |
| [longhorn](longhorn.md) | Storage | Disabled | Distributed block storage | |
| [external-snapshotter](external-snapshotter.md) | Storage | Enabled | CSI volume snapshot controller | |
| [kube-prometheus-stack](kube-prometheus-stack.md) | Observability | Enabled | Prometheus, Grafana, Alertmanager | |
| [loki](loki.md) | Observability | Enabled | Log aggregation (S3/Swift backends) | |
| [tempo](tempo.md) | Observability | Enabled | Distributed tracing (S3/Swift backends) | |
| [mimir](mimir.md) | Observability | Disabled | Long-term metrics storage | Depends: kafka-cluster |
| [opentelemetry-kube-stack](opentelemetry-kube-stack.md) | Observability | Disabled | OpenTelemetry collectors | |
| [alert-proxy](alert-proxy.md) | Observability | Disabled | Alert forwarding proxy | Managed service |
| [fluxcd](fluxcd.md) | GitOps | Enabled | GitOps continuous delivery | Core dependency |
| [weave-gitops](weave-gitops.md) | GitOps | Disabled | GitOps dashboard UI | |
| [velero](velero.md) | Backup | Enabled | Cluster backup and disaster recovery | Multi-backend storage |
| [etcd-backup](etcd-backup.md) | Backup | Enabled | etcd snapshot backup | |
| [headlamp](headlamp.md) | Management | Enabled | Kubernetes dashboard with OIDC | |
| [olm](olm.md) | Management | Enabled | Operator Lifecycle Manager | |
| [postgres-operator](postgres-operator.md) | Management | Enabled | PostgreSQL operator (Zalando) | |
| [harbor](harbor.md) | Management | Disabled | Container registry | |
| [kafka-cluster](kafka-cluster.md) | Management | Disabled | Apache Kafka (Strimzi) | |

## Enable/Disable Services

```bash
# Enable a service
opencenter cluster set my-cluster opencenter.services.loki.enabled=true

# Disable a service
opencenter cluster set my-cluster opencenter.services.loki.enabled=false

# View service options
opencenter cluster service options loki

# View all service states
opencenter cluster service status
```

## Credential Fallback Chain

Services that need cloud credentials follow this resolution order:

1. Service-specific secret (e.g., `secrets.loki.s3_access_key_id`)
2. Global application credentials (`secrets.global.aws.application.*`)
3. Global infrastructure credentials (`secrets.global.aws.infrastructure.*`)

## Storage Provider Defaults

The infrastructure provider determines the default `storage_type` for services that require object storage:

| Infrastructure Provider | Default Storage | Available Options |
|------------------------|----------------|-------------------|
| OpenStack | `swift` | swift, s3 |
| AWS | `s3` | s3 |
| GCP | `gcs` | gcs |
| Azure | `azure` | azure |

## Related Documentation

- [Customize Services](../../operations/customize-services.md) — how-to guide for enabling, disabling, and configuring services
- [Services and Templates](../../concepts/services-templates.md) — how the template system generates service manifests
- [Adding Services](../../contributing/adding-services.md) — contributor guide for adding new services
