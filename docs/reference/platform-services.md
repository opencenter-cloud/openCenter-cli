---
id: platform-services
title: "Platform Services Reference"
sidebar_label: Platform Services Reference
description: Complete reference of available platform services, versions, and configuration options.
doc_type: reference
audience: "operators, platform engineers"
tags: [services, platform, helm, monitoring, security]
---
# Platform Services Reference

> **⚠️ Deprecated:** This page is superseded by the per-service reference pages at [docs/reference/services/](services/index.md). Each service now has a dedicated page with complete configuration, secrets, dependencies, and verification steps. This file is retained for backward compatibility but will be removed in a future release.

**Purpose:** Complete reference of available platform services, versions, and configuration options for quick lookup.

This reference documents all platform services that can be deployed with openCenter clusters.

## Service Categories

* **Networking:** CNI plugins, ingress, load balancing
* **Security:** Certificate management, policy enforcement, identity
* **Storage:** Persistent storage, CSI drivers, snapshots
* **Observability:** Monitoring, logging, tracing
* **GitOps:** Continuous delivery, source management
* **Backup:** Disaster recovery, etcd backup
* **Management:** Dashboards, operators, RBAC

## Networking Services

### calico

**Category:** CNI Plugin\
**Default:** Enabled\
**Description:** Calico CNI for pod networking with BGP support

**Configuration:**

```yaml
opencenter:
  services:
    calico:
      enabled: true
      kube_api_server: "https://api.<cluster>.<region>.k8s.opencenter.cloud:6443"
```

**Features:**

* VXLAN or IPIP encapsulation
* BGP routing
* Network policies
* IPv4/IPv6 dual-stack

**Dependencies:** None

### gateway-api

**Category:** Networking\
**Default:** Enabled\
**Description:** Gateway API CRDs for modern ingress

**Configuration:**

```yaml
opencenter:
  services:
    gateway-api:
      enabled: true
```

**Features:**

* HTTPRoute, TLSRoute, TCPRoute
* Gateway class support
* Role-based access

**Dependencies:** None

### gateway

**Category:** Networking\
**Default:** Enabled\
**Description:** Gateway API implementation (Envoy-based)

**Configuration:**

```yaml
opencenter:
  services:
    gateway:
      enabled: true
```

**Features:**

* HTTP/HTTPS routing
* TLS termination
* Load balancing

**Dependencies:** gateway-api

## Security Services

### cert-manager

**Category:** Security\
**Default:** Enabled\
**Description:** Automated TLS certificate management

**Configuration:**

```yaml
opencenter:
  services:
    cert-manager:
      enabled: true
      email: "admin@example.com"
      region: "us-east-1"
      letsencrypt_server: "https://acme-v02.api.letsencrypt.org/directory"
```

**Features:**

* Let’s Encrypt integration
* ACME protocol support
* Certificate renewal
* DNS-01 and HTTP-01 challenges

**Dependencies:** None

**Secrets:**

```yaml
secrets:
  cert_manager:
    aws_access_key: ""        # For Route53 DNS-01
    aws_secret_access_key: ""
```

### keycloak

**Category:** Security\
**Default:** Enabled\
**Description:** Identity and access management

**Configuration:**

```yaml
opencenter:
  services:
    keycloak:
      enabled: true
      hostname: "auth.<org>.<cluster>.<region>.k8s.opencenter.cloud"
      realm: "opencenter"
      client_id: "kubernetes"
      frontend_url: "https://auth.<org>.<cluster>.<region>.k8s.opencenter.cloud"
```

**Features:**

* OIDC provider
* SAML support
* User federation
* Multi-factor authentication

**Dependencies:** cert-manager, gateway-api, postgres-operator

**Secrets:**

```yaml
secrets:
  keycloak:
    client_secret: ""   # internal OIDC bootstrap may create this later
    admin_password: ""  # user-provided for now
```

### kyverno

**Category:** Security\
**Default:** Enabled\
**Description:** Kubernetes policy engine

**Configuration:**

```yaml
opencenter:
  services:
    kyverno:
      enabled: true
```

**Features:**

* Policy validation
* Resource mutation
* Policy generation
* 17 default ClusterPolicies

**Default Policies:**

* disallow-privileged-containers
* disallow-host-namespaces
* disallow-host-path
* require-run-as-nonroot
* restrict-seccomp
* restrict-volume-types
* And 11 more

**Dependencies:** None

### rbac-manager

**Category:** Security\
**Default:** Enabled\
**Description:** Declarative RBAC management

**Configuration:**

```yaml
opencenter:
  services:
    rbac-manager:
      enabled: true
```

**Features:**

* RBACDefinition CRD
* Automatic RoleBinding creation
* Keycloak integration
* Group-based access

**Dependencies:** keycloak (optional)

## Storage Services

### openstack-csi

**Category:** Storage\
**Default:** Enabled (OpenStack only)\
**Description:** OpenStack Cinder CSI driver

**Configuration:**

```yaml
opencenter:
  services:
    openstack-csi:
      enabled: true
```

**Features:**

* Dynamic volume provisioning
* Volume snapshots
* Volume expansion
* Multi-attach volumes

**Dependencies:** openstack-ccm

### vsphere-csi

**Category:** Storage\
**Default:** Disabled\
**Description:** VMware vSphere CSI driver

**Configuration:**

```yaml
opencenter:
  services:
    vsphere-csi:
      enabled: true
      image_repository: "registry.k8s.io/csi-vsphere"
      image_tag: "v3.3.0"
```

**Features:**

* vSphere datastore integration
* Volume snapshots
* Storage policies
* Topology awareness

**Dependencies:** None

**Secrets:**

```yaml
secrets:
  vsphere_csi:
    vcenter_host: ""
    username: ""
    password: ""
    datacenters: ""
    insecure_flag: "false"
    port: "443"
```

### external-snapshotter

**Category:** Storage\
**Default:** Enabled\
**Description:** Volume snapshot controller

**Configuration:**

```yaml
opencenter:
  services:
    external-snapshotter:
      enabled: true
```

**Features:**

* VolumeSnapshot CRD
* Snapshot scheduling
* Snapshot restore

**Dependencies:** CSI driver (openstack-csi or vsphere-csi)

## Observability Services

### kube-prometheus-stack

**Category:** Observability\
**Default:** Enabled\
**Description:** Complete monitoring solution

**Configuration:**

```yaml
opencenter:
  services:
    kube-prometheus-stack:
      enabled: true
      prometheus_volume_size: 50
      prometheus_storage_class: "csi-cinder-sc-delete"
      grafana_volume_size: 10
      grafana_storage_class: "csi-cinder-sc-delete"
      alertmanager_volume_size: 10
      alertmanager_storage_class: "csi-cinder-sc-delete"
```

**Components:**

* Prometheus (metrics)
* Grafana (visualization)
* Alertmanager (alerting)
* Node exporter
* Kube-state-metrics

**Features:**

* Pre-configured dashboards
* Alert rules
* Service discovery
* Long-term storage

**Dependencies:** None

**Secrets:**

```yaml
secrets:
  grafana:
    admin_password: ""
```

### loki

**Category:** Observability\
**Default:** Enabled\
**Description:** Log aggregation system

**Configuration:**

```yaml
opencenter:
  services:
    loki:
      enabled: true
      volume_size: 20
      storage_class: "csi-cinder-sc-delete"
      bucket_name: "my-cluster-loki"
      swift_auth_url: "https://keystone.api.sjc3.rackspacecloud.com/v3/"
      swift_region: "SJC3"
      swift_domain_name: "Default"
```

**Features:**

* S3-compatible storage
* LogQL query language
* Grafana integration
* Multi-tenancy

**Dependencies:** kube-prometheus-stack (for Grafana)

**Secrets:**

```yaml
secrets:
  loki:
    swift_password: ""
```

### tempo

**Category:** Observability\
**Default:** Enabled\
**Description:** Distributed tracing backend

**Configuration:**

```yaml
opencenter:
  services:
    tempo:
      enabled: true
      storage_type: "s3"
      bucket_name: "my-cluster-tempo"
      volume_size: 10
      storage_class: "csi-cinder-sc-delete"
      s3_endpoint: "https://swift.api.sjc3.rackspacecloud.com"
      s3_region: "SJC3"
      s3_force_path_style: false
      s3_insecure: false
```

**Features:**

* OpenTelemetry support
* Jaeger compatibility
* S3 storage backend
* Grafana integration

**Dependencies:** kube-prometheus-stack (for Grafana)

**Secrets:**

```yaml
secrets:
  tempo:
    access_key: ""
    secret_key: ""
```

## GitOps Services

### fluxcd

**Category:** GitOps\
**Default:** Enabled\
**Description:** GitOps continuous delivery

**Configuration:**

```yaml
opencenter:
  services:
    fluxcd:
      enabled: true
```

**Components:**

* source-controller
* kustomize-controller
* helm-controller
* notification-controller

**Features:**

* Git repository sync
* Helm release management
* Kustomize support
* SOPS decryption

**Dependencies:** None

### sources

**Category:** GitOps\
**Default:** Enabled\
**Description:** FluxCD GitRepository sources

**Configuration:**

```yaml
opencenter:
  services:
    sources:
      enabled: true
```

**Features:**

* GitRepository CRDs
* SSH authentication
* Branch/tag tracking

**Dependencies:** fluxcd

### weave-gitops

**Category:** GitOps\
**Default:** Disabled\
**Description:** Weave GitOps UI

**Configuration:**

```yaml
opencenter:
  services:
    weave-gitops:
      enabled: true
      hostname: "gitops.<org>.<cluster>.<region>.k8s.opencenter.cloud"
```

**Features:**

* Web UI for FluxCD
* Resource visualization
* Reconciliation status
* Application management

**Dependencies:** fluxcd, cert-manager, gateway-api

**Secrets:**

```yaml
secrets:
  weave_gitops:
    password: ""
    password_hash: ""
```

## Backup Services

### velero

**Category:** Backup\
**Default:** Enabled\
**Description:** Backup and disaster recovery

**Configuration:**

```yaml
opencenter:
  services:
    velero:
      enabled: true
      backup_bucket: "my-cluster-backups"
      region: "us-east-1"
```

**Features:**

* Cluster backup
* Namespace backup
* Scheduled backups
* Restore operations

**Dependencies:** CSI driver (for volume snapshots)

### etcd-backup

**Category:** Backup\
**Default:** Enabled\
**Description:** Etcd backup to S3

**Configuration:**

```yaml
opencenter:
  services:
    etcd-backup:
      enabled: true
      s3_host: "https://swift.api.dfw3.rackspacecloud.com"
      s3_region: "DFW3"
```

**Features:**

* Scheduled etcd snapshots
* S3 storage
* Encryption at rest
* Retention policies

**Dependencies:** None

## Management Services

### headlamp

**Category:** Management\
**Default:** Enabled\
**Description:** Kubernetes dashboard

**Configuration:**

```yaml
opencenter:
  services:
    headlamp:
      enabled: true
      hostname: "dashboard.<org>.<cluster>.<region>.k8s.opencenter.cloud"
      oidc_issuer_url: "https://auth.<org>.<cluster>.<region>.k8s.opencenter.cloud/realms/opencenter"
      oidc_client_id: "kubernetes"
```

**Features:**

* Web-based UI
* OIDC authentication
* Resource management
* Log viewing

**Dependencies:** keycloak, cert-manager, gateway-api

**Secrets:**

```yaml
secrets:
  headlamp:
    oidc_client_secret: "" # internal OIDC bootstrap may create this later
```

### olm

**Category:** Management\
**Default:** Enabled\
**Description:** Operator Lifecycle Manager

**Configuration:**

```yaml
opencenter:
  services:
    olm:
      enabled: true
```

**Features:**

* Operator installation
* Dependency resolution
* Upgrade management
* Catalog management

**Dependencies:** None

### postgres-operator

**Category:** Management\
**Default:** Enabled\
**Description:** PostgreSQL operator

**Configuration:**

```yaml
opencenter:
  services:
    postgres-operator:
      enabled: true
```

**Features:**

* PostgreSQL cluster management
* High availability
* Backup and restore
* Connection pooling

**Dependencies:** None

## Cloud Provider Services

### openstack-ccm

**Category:** Cloud Provider\
**Default:** Enabled (OpenStack only)\
**Description:** OpenStack cloud controller manager

**Configuration:**

```yaml
opencenter:
  services:
    openstack-ccm:
      enabled: true
```

**Features:**

* Load balancer integration
* Node lifecycle management
* Route management
* Service integration

**Dependencies:** None

## Managed Services

### alert-proxy

**Category:** Managed Service\
**Default:** Disabled\
**Description:** Alert forwarding to external systems

**Configuration:**

```yaml
opencenter:
  managed_service:
    alert-proxy:
      enabled: true
      image_repository: "ghcr.io/opencenter-cloud/alert-proxy"
      image_tag: "latest"
      alertmanager_base_url: "http://alertmanager:9093"
      httproute_fqdn: "https://alerts.<org>.<cluster>.<region>.k8s.opencenter.cloud"
```

**Features:**

* Alertmanager integration
* External API forwarding
* Alert transformation

**Dependencies:** kube-prometheus-stack

**Secrets:**

```yaml
secrets:
  alert_proxy:
    core_device_id: ""
    account_service_token: ""
    core_account_number: ""
```

## Service Dependencies

### Dependency Graph

```
cert-manager (no deps)
  ├── keycloak
  │   ├── headlamp
  │   └── rbac-manager
  ├── gateway-api
  │   ├── gateway
  │   ├── headlamp
  │   ├── keycloak
  │   └── weave-gitops
  └── weave-gitops

fluxcd (no deps)
  ├── sources
  └── weave-gitops

kube-prometheus-stack (no deps)
  ├── loki
  ├── tempo
  └── alert-proxy

postgres-operator (no deps)
  └── keycloak

openstack-ccm (no deps)
  └── openstack-csi

CSI driver (openstack-csi or vsphere-csi)
  ├── external-snapshotter
  └── velero
```

## Service Versions

Service versions are managed in openCenter-gitops-base repository. Versions are pinned for reproducibility.

**Update Strategy:**

1. Test new version in dev environment
2. Update gitops-base repository
3. Tag new release
4. Update cluster configuration to use new tag

## Enabling/Disabling Services

See [Customize Services](../operations/customize-services.md) for detailed instructions.

---

## Evidence

This reference is based on:

* Service defaults: `internal/config/defaults.go:293-388`
* Service base config: `internal/config/services/base.go:1-35`
* Session 2 facts inventory: B0 section 6
* Ecosystem services: Ecosystem.md infrastructure services
* Session 1 architecture: A2