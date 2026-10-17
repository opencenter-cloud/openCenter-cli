---
last_updated: 2026-09-24
id: reference-architecture
title: "Reference Architecture"
sidebar_label: Reference Architecture
description: Baseline infrastructure architecture, networking, security, and identity for openCenter deployments across OpenStack, VMware, bare metal, Kind, and Magnum.
doc_type: explanation
audience: "architects, platform engineers, operators"
tags: [architecture, networking, security, identity, baseline, reference, magnum]
---
# Reference Architecture

**Purpose:** For architects and platform engineers, explains the baseline infrastructure architecture required to deploy openCenter, covering network topology, security, identity, storage, and operational concerns across the supported infrastructure targets: OpenStack, VMware, bare metal, Kind, and Magnum.

This document describes the recommended infrastructure foundation for openCenter deployments. It does not cover workload-specific architecture. Application teams should layer their own patterns on top of this baseline. For the authoritative provider support matrix, see [Infrastructure Providers Reference](../reference/providers.md) and [Providers](../CODEMAPS/providers.md); this document stays consistent with those but does not repeat every detail.

## Architecture

openCenter deploys a Kubernetes platform through a layered architecture. Each layer has a clear owner and a defined interface to the layer above it. The exact mechanics differ by provider: OpenStack, VMware, and bare metal share a Kubespray-driven bootstrap; Kind uses its dedicated local bootstrap; Magnum delegates cluster creation to the OpenStack Magnum service and does not use OpenTofu.

```
┌─────────────────────────────────────────────────────────┐
│  Layer 5: Platform Services (GitOps-managed)             │
│  cert-manager · Keycloak · Kyverno · kube-prometheus-    │
│  stack · Loki · Tempo · Velero · Gateway API · RBAC      │
│  Manager · Headlamp · OLM · PostgreSQL Operator          │
├─────────────────────────────────────────────────────────┤
│  Layer 4: GitOps Engine (FluxCD)                         │
│  source-controller · kustomize-controller                │
│  helm-controller · notification-controller               │
├─────────────────────────────────────────────────────────┤
│  Layer 3: Kubernetes Cluster                              │
│  Kubespray (OpenStack/VMware/bare metal), Kind, or         │
│  Magnum (OpenStack-managed)                                │
├─────────────────────────────────────────────────────────┤
│  Layer 2: Infrastructure (OpenTofu / pre-provisioned;      │
│  not applicable to Magnum)                                 │
│  Compute · Networking · Storage · Bastion                  │
├─────────────────────────────────────────────────────────┤
│  Layer 1: Provider (OpenStack / VMware / bare metal /       │
│  Kind / Magnum)                                              │
│  Hypervisor or physical network · Block storage             │
└─────────────────────────────────────────────────────────┘
```

**Evidence:** `docs/reference/providers.md`, `docs/CODEMAPS/providers.md`, `internal/cluster/bootstrap_provider_infra.go`, `internal/cluster/kind_bootstrap_provider.go`, `internal/cluster/magnum_bootstrap_provider.go`

### Design Principles

These principles shaped every architectural decision:

* **Configuration as code.** A single YAML file (validated against `schema/opencenter-v2.schema.json`) defines the cluster. The live bootstrap paths consume that configuration for provider-specific provisioning and initialization.
* **GitOps as the operational model.** Git is the source of truth. FluxCD reconciles desired state continuously. Changes flow through commits, not `kubectl apply`.
* **Defense in depth.** Security controls exist at multiple layers (CLI input validation, secrets encryption, cluster admission, platform policy). See [Security Model](security-model.md).
* **Provider abstraction, with one exception.** The same configuration structure works across OpenStack, VMware, bare metal, and Kind. Magnum is deliberately different: it is backed by the OpenStack Magnum service rather than OpenTofu, and image/network/COE choices come from the Magnum cluster template rather than from openCenter's own infrastructure fields.
* **Composition over duplication.** Some platform-service manifests (etcd-backup, Calico, cert-manager, Keycloak, FluxCD, Harbor, Kafka, OLM, sources) are templated from `internal/gitops/templates/cluster-apps-base/` in this repository; others (for example Kyverno's policy set and Velero's manifests) are pulled from the separate `openCenter-gitops-base` repository via FluxCD. This document does not restate that external repository's contents.
* **Fail fast.** Configuration passes through a multi-stage pipeline (schema → business rules → provider/deployment-method validation → readiness checks) before any infrastructure is provisioned. See [Configuration Lifecycle](configuration-lifecycle.md).
* **Explicit dependencies.** Kind, for example, force-enables OLM and postgres-operator because Keycloak depends on them (`internal/config/v2/defaults.go`).

### Minimum Recommended Baseline

The `opencenter cluster init` defaults for OpenStack, VMware, and bare metal are:

| Component | Default |
| --- | --- |
| Control plane nodes | 3 (`master_count: 3`) |
| Worker nodes | 3 (`worker_count: 3`) |
| Bastion host | Enabled for OpenStack/VMware; disabled by default for bare metal (`infrastructure.bastion.enabled`) |
| CNI | Calico (`cluster.kubernetes.network_plugin.calico`); Cilium and Kube-OVN are also schema-supported but are not part of the default service set |
| Control-plane HA | Both `vrrp_enabled` and `kube_vip_enabled` default to `true` for OpenStack/VMware/bare metal (`false` for Kind) |
| Load balancer provider | `infrastructure.networking.loadbalancer_provider` defaults to `"ovn"`; valid values are `ovn`, `octavia`, `metallb`, `cloud-native` |
| Ingress | Gateway API (`services.gateway`, `services.gateway-api`), enabled by default |
| Identity | Keycloak (`services.keycloak`), enabled by default |
| Secrets | SOPS/Age encryption for the GitOps tree; see [Security Model](security-model.md) |
| Monitoring | kube-prometheus-stack, Loki, Tempo — all enabled by default |
| Backup | etcd-backup and Velero exist as services, but **Velero defaults to enabled only for OpenStack** and etcd-backup defaults to disabled for every provider; see [Business Continuity Decisions](#business-continuity-decisions) |
| Policy | Kyverno (`services.kyverno`), enabled by default; its policy content is not in this repository — see [Policy Management](#policy-management) |
| Storage | Provider CSI plugin selected automatically by `cluster.kubernetes.storage_plugin` (Cinder for OpenStack, vSphere for VMware); Longhorn (`services.longhorn`) exists but defaults to disabled |
| Certificate management | cert-manager (`services.cert-manager`), enabled by default, `letsencrypt_server` defaults to the public Let's Encrypt ACME endpoint |

**Evidence:** `internal/config/v2/defaults.go` (`applyProviderBehaviorDefaults`, `NewDefaultServiceConfig`, `defaultServiceMap`), `schema/opencenter-v2.schema.json`

### Target-Specific Architecture

#### OpenStack

OpenStack is the most automated provider. openCenter provisions all infrastructure through OpenTofu.

```
┌──────────────────────────────────────────────────────────────┐
│  OpenStack Region                                            │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │  Tenant Network (VLAN or VXLAN)                      │    │
│  │  subnet_nodes: 10.2.128.0/22                         │    │
│  │                                                      │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐              │    │
│  │  │ CP-1    │  │ CP-2    │  │ CP-3    │  Control      │    │
│  │  │ .20     │  │ .21     │  │ .22     │  Plane        │    │
│  │  └────┬────┘  └────┬────┘  └────┬────┘              │    │
│  │       │             │            │                    │    │
│  │       └──────┬──────┘            │                    │    │
│  │              │  VRRP VIP (.10)   │                    │    │
│  │              │  or Octavia LB    │                    │    │
│  │              │                   │                    │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐              │    │
│  │  │ WK-1    │  │ WK-2    │  │ WK-3    │  Workers     │    │
│  │  │ .23     │  │ .24     │  │ .25     │              │    │
│  │  └─────────┘  └─────────┘  └─────────┘              │    │
│  │                                                      │    │
│  │  ┌─────────┐                                         │    │
│  │  │ Bastion │  .26  (SSH jump + ansible runner)       │    │
│  │  └─────────┘                                         │    │
│  └──────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐       │
│  │ Cinder       │  │ Octavia      │  │ Designate    │       │
│  │ (Block Vol)  │  │ (LB, opt.)   │  │ (DNS, opt.)  │       │
│  └──────────────┘  └──────────────┘  └──────────────┘       │
└──────────────────────────────────────────────────────────────┘
```

Key characteristics:

* Automated VM provisioning via OpenTofu; the generated infrastructure module invokes the Kubespray playbook through its `local-exec` provisioner (`internal/cluster/bootstrap_provider_infra.go`)
* Cinder CSI for persistent volumes; default storage class is `csi-cinder-sc-delete` and default boot-volume type is `HA-Standard` (`internal/config/v2/defaults.go`)
* `loadbalancer_provider` defaults to `"ovn"` (Neutron/OVN load balancer); `octavia` is also a supported value for API/service load balancing, and `vrrp_enabled`/`kube_vip_enabled` both default to `true` for VIP-based API HA
* Optional Designate DNS integration (`infrastructure.networking.use_designate`)
* `infrastructure.compute.worker_group_affinity` and `infrastructure.compute.cp_server_group_affinity` default to `["soft-anti-affinity"]`
* Boot-from-volume: `infrastructure.storage.worker_volume_size`/`master_volume_size` default to 40 GB each, with `worker_volume_destination_type: "volume"` and `worker_volume_source_type: "image"`
* Region-specific defaults (image IDs, flavors, DNS/NTP servers) are looked up from a provider-defaults registry (`internal/config/defaults/openstack.go`); for example the SJC3 and DFW3 regions both default to flavors `gp.5.2.4` (bastion), `gp.5.4.8` (master), and `gp.5.4.16` (worker)
* Windows worker pools are supported (`flavor_worker_windows`, `image_id_windows`, `additional_server_pools_worker_windows`), with OpenStack-specific readiness checks requiring `image_id_windows` or a per-pool image

**Evidence:** `internal/config/v2/infrastructure.go`, `internal/config/v2/defaults.go`, `internal/config/defaults/openstack.go`, `internal/config/v2/deployment_validator.go`, `internal/config/v2/readiness.go`

#### VMware (vSphere)

VMware uses pre-provisioned VMs. The infrastructure team owns VM lifecycle; openCenter owns cluster and service lifecycle.

```
┌──────────────────────────────────────────────────────────────┐
│  vSphere Cluster / Datacenter                                │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │  Port Group / VLAN                                   │    │
│  │  subnet_nodes: 192.168.12.0/24                       │    │
│  │                                                      │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐              │    │
│  │  │ CP-1    │  │ CP-2    │  │ CP-3    │  Control      │    │
│  │  │ .20     │  │ .21     │  │ .22     │  Plane        │    │
│  │  └────┬────┘  └────┬────┘  └────┬────┘              │    │
│  │       │             │            │                    │    │
│  │       └──────┬──────┘            │                    │    │
│  │              │  VRRP VIP (.10)   │                    │    │
│  │              │  or kube-vip      │                    │    │
│  │              │                   │                    │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐              │    │
│  │  │ WK-1    │  │ WK-2    │  │ WK-3    │  Workers     │    │
│  │  │ .23     │  │ .24     │  │ .25     │              │    │
│  │  └─────────┘  └─────────┘  └─────────┘              │    │
│  │                                                      │    │
│  │  ┌─────────┐                                         │    │
│  │  │ Bastion │  .26                                    │    │
│  │  └─────────┘                                         │    │
│  └──────────────────────────────────────────────────────┘    │
│                                                              │
│  ┌──────────────┐  ┌──────────────┐                          │
│  │ vSAN/VMFS    │  │ vSphere CSI  │                          │
│  │ (Datastore)  │  │ (PV driver)  │                          │
│  └──────────────┘  └──────────────┘                          │
└──────────────────────────────────────────────────────────────┘
```

Key characteristics:

* VMs pre-provisioned by the infrastructure team; canonical provider name is `vmware` (`vsphere` continues to load as a compatibility alias — `docs/reference/providers.md`)
* vSphere CSI driver for persistent volumes; the in-cluster plugin is enabled automatically (`cluster.kubernetes.storage_plugin.vsphere_csi`), and the default storage class is `vsphere-csi` (not a fixed `vsphere-sc` name — `internal/config/v2/defaults.go`)
* MetalLB (`services.metallb`) is available for LoadBalancer services but defaults to **disabled**; it must be explicitly enabled
* `vrrp_enabled` and `kube_vip_enabled` both default to `true` for API server HA
* Shares the same OpenTofu/Kubespray bootstrap provider as OpenStack and bare metal (`internal/cluster/provider/openstack` bootstrap path is reused across these three — `docs/CODEMAPS/providers.md`)
* Has a real drift implementation (`internal/cloud/vmware`) — detect only, no auto-reconcile, consistent with `docs/reference/providers.md`

**Evidence:** `internal/cloud/vmware/`, `internal/config/v2/defaults.go`, `docs/reference/providers.md`, `docs/CODEMAPS/providers.md`

#### Bare Metal

Bare metal is a real, GA-supported target for configuration, generation, and bootstrap/deploy (`docs/reference/providers.md`, `docs/CODEMAPS/providers.md`) — it follows the same pre-provisioned-host model as VMware. There is, however, no dedicated `internal/cloud/baremetal` package: `internal/cloud/` only contains `openstack`, `vmware`, `kind`, and `magnum`, because bare metal has no cloud-state drift/reconciliation interface to implement (`internal/cloud.CloudProvider`). Bare metal's config/generate and bootstrap/deploy support instead comes from the shared `openstackBootstrapProvider` bootstrap path in `internal/cluster/` (reused across OpenStack, VMware, and bare metal) plus static-node validation — it does not use OpenStack or vSphere credentials.

```
┌──────────────────────────────────────────────────────────────┐
│  Physical Rack / Network Segment                             │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐    │
│  │  L2/L3 Network Segment                               │    │
│  │  subnet_nodes: 10.0.0.0/24                           │    │
│  │                                                      │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐              │    │
│  │  │ CP-1    │  │ CP-2    │  │ CP-3    │  Physical     │    │
│  │  │ .10     │  │ .11     │  │ .12     │  Servers      │    │
│  │  └────┬────┘  └────┬────┘  └────┬────┘              │    │
│  │       └──────┬──────┘            │                    │    │
│  │              │  VRRP VIP (.5)    │                    │    │
│  │              │                   │                    │    │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐              │    │
│  │  │ WK-1    │  │ WK-2    │  │ WK-3    │  Physical     │    │
│  │  │ .20     │  │ .21     │  │ .22     │  Servers      │    │
│  │  └─────────┘  └─────────┘  └─────────┘              │    │
│  │                                                      │    │
│  │  ┌─────────┐                                         │    │
│  │  │ Bastion │  .2                                     │    │
│  │  └─────────┘                                         │    │
│  └──────────────────────────────────────────────────────┘    │
│                                                              │
│  Storage: Longhorn (distributed) or local disks              │
│  Load Balancer: MetalLB (L2 or BGP mode)                     │
└──────────────────────────────────────────────────────────────┘
```

Key characteristics:

* Hardware lifecycle managed outside openCenter; `infrastructure.bastion.enabled` defaults to `false` for bare metal (no automated bastion provisioning)
* No cloud CSI driver — `services.longhorn` exists and is a reasonable choice for persistent storage, but it defaults to **disabled** and must be explicitly enabled
* `services.metallb` also defaults to **disabled**; it is the practical choice for LoadBalancer services since there is no cloud LB, but nothing enables it automatically
* `vrrp_enabled`/`kube_vip_enabled` default to `true`, same as OpenStack/VMware
* Calico's `ipip_mode`/`vxlan_mode`/`encapsulation_type` fields (`cluster.kubernetes.network_plugin.calico`) allow selecting a BGP-style "Never" encapsulation mode for direct routing, but this is a general Calico option, not bare-metal-specific
* Windows worker pools have a bare-metal-specific readiness check: `worker_count_windows > 0` requires a non-empty `windows_nodes` list of static nodes (`internal/config/v2/readiness.go`)
* No OpenStack-CCM/CSI services are enabled for this provider (`applyProviderBehaviorDefaults` disables them)
* No cloud-state drift detection is available (consistent with `docs/reference/providers.md`)

**Evidence:** `internal/config/v2/defaults.go`, `internal/config/v2/readiness.go`, `docs/CODEMAPS/providers.md`

#### Kind (Local Development)

Kind runs Kubernetes inside Docker containers on a single host. It is not a production target.

```
┌──────────────────────────────────────────────────────────┐
│  Developer Workstation / CI Runner                       │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Docker Network (bridge)                           │  │
│  │  172.18.0.0/16 (Docker default)                    │  │
│  │                                                    │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐         │  │
│  │  │ CP-1     │  │ WK-1     │  │ WK-2     │         │  │
│  │  │ container│  │ container│  │ container│         │  │
│  │  └──────────┘  └──────────┘  └──────────┘         │  │
│  │                                                    │  │
│  │  ┌──────────┐                                      │  │
│  │  │ Registry │  (optional, disabled by default,     │  │
│  │  │          │   default port 5001)                 │  │
│  │  └──────────┘                                      │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  API: localhost:<api_server_port> (default 6443)          │
│  Ingress: extra_port_mappings                              │
└──────────────────────────────────────────────────────────┘
```

Key characteristics:

* Single-host, containerized nodes (not VMs); defaults to 1 control-plane and 2 worker containers (`kindDefaultControlPlaneCount`/`kindDefaultWorkerCount`)
* No bastion, no VRRP, no kube-vip: `bastion.enabled`, `kube_vip_enabled`, and `vrrp_enabled` are all forced to `false`/empty for Kind
* Kind's pod/service subnets default differently from the other providers: `10.244.0.0/16` (pods) and `10.96.0.0/16` (services), not the `10.42.0.0/16`/`10.43.0.0/16` used elsewhere
* Optional local container registry (`infrastructure.kind.registry`, disabled by default, default port 5001, default name `kind-registry`)
* `extra_port_mappings` for host-to-node port forwarding
* CNI selectable via `disable_default_cni` (defaults to `false`, i.e. kindnet)
* Disposable: create and destroy in minutes

**Evidence:** `internal/config/v2/infrastructure.go` (`KindConfig`, `KindRegistryConfig`), `internal/config/v2/defaults.go`

#### Magnum (Managed OpenStack Kubernetes)

Magnum is architecturally different from the other four targets: it is a thin client over the OpenStack Magnum API (`internal/cloud/magnum`), not an OpenTofu/Kubespray deployment. There is no "infrastructure layer" to plan in the OpenCenter sense — image, network, and COE (container orchestration engine) choices are all owned by a pre-existing Magnum cluster template.

```
┌────────────────────────────────────────────────────────┐
│  OpenStack Region                                        │
│                                                            │
│  Magnum Cluster Template (owns image, network, COE)         │
│         │                                                   │
│         ▼                                                   │
│  Magnum-managed Kubernetes cluster                            │
│  (Magnum provisions and manages control plane + workers)      │
└────────────────────────────────────────────────────────┘
```

Key characteristics:

* Configuration lives under `opencenter.infrastructure.cloud.magnum`: required fields are `auth_url`, `cluster_template`, `project_id`, and `region`; optional fields include `application_credential_id`/`application_credential_secret`, `domain`, `ca`, `insecure`, `keypair`, `labels`, `master_flavor_id`, `master_lb_enabled`, `node_flavor_id`, and `create_timeout`
* `cluster deploy` creates a Magnum cluster from the cluster template, polls Magnum until ready, and securely writes the resulting kubeconfig (`internal/cluster/magnum_bootstrap_provider.go`); `cluster destroy` deletes it (`internal/cluster/magnum_destroy_provider.go`)
* No OpenTofu step, and no drift detection — Magnum is not in the `opencenter cluster drift` provider set (`docs/reference/providers.md`)
* Gophercloud v1's Magnum client calls don't accept a `context.Context`, so `internal/cloud/magnum` checks cancellation before/after each call and re-authenticates its own `ProviderClient` per request group (`docs/CODEMAPS/providers.md`)

**Evidence:** `internal/cloud/magnum/`, `internal/cluster/magnum_bootstrap_provider.go`, `internal/cluster/magnum_configure_orchestrator.go`, `schema/opencenter-v2.schema.json` (`opencenter.infrastructure.cloud.magnum`)

## Network Topology

### Subnet Architecture

Every openCenter cluster operates on three non-overlapping IP networks:

| Network | Default CIDR | Purpose | Typical Size |
| --- | --- | --- | --- |
| Node network | `10.2.128.0/22` | Physical/virtual node IPs, bastion, VIP | /22 (1,024 IPs) |
| Pod network | `10.42.0.0/16` | Pod-to-pod communication (CNI-managed) | /16 (65,536 IPs) |
| Service network | `10.43.0.0/16` | ClusterIP services (kube-proxy/eBPF) | /16 (65,536 IPs) |

These three networks must not overlap with each other or with any existing infrastructure networks (VPNs, corporate LANs, other clusters).

### Traffic Flows

```
External Traffic
      │
      ▼
┌─────────────┐
│ Gateway API │  (services.gateway-api namespace defaults to
│             │   "envoy-gateway-system"); LB per
│             │   loadbalancer_provider (ovn/octavia/metallb/
│             │   cloud-native)
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Service     │  ClusterIP (10.43.x.x default)
│ Network     │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Pod Network │  Pod IPs (10.42.x.x default)
│ (Calico by default) │  VXLAN/IPIP/no-encapsulation, per network_plugin.calico
└─────────────┘
```

### Control Plane Access

The Kubernetes API server's HA mechanism is provider-dependent. Note that `vrrp_enabled` and `kube_vip_enabled` are independent booleans and both default to `true` for OpenStack, VMware, and bare metal simultaneously — the schema does not enforce that only one is set:

| Provider | API HA options | Configuration |
| --- | --- | --- |
| OpenStack | Neutron/OVN LB (default), Octavia LB, and/or VRRP/kube-vip VIP | `infrastructure.networking.loadbalancer_provider` (`ovn`\|`octavia`\|`metallb`\|`cloud-native`), `infrastructure.networking.use_octavia`, `infrastructure.networking.vrrp_enabled`/`vrrp_ip`, `cluster.kubernetes.kube_vip_enabled` |
| VMware | VRRP and/or kube-vip VIP | `infrastructure.networking.vrrp_enabled`/`vrrp_ip`, `cluster.kubernetes.kube_vip_enabled` |
| Bare metal | VRRP and/or kube-vip VIP | Same fields as VMware |
| Kind | Docker-to-host port mapping | `infrastructure.kind.api_server_port` (default `6443`) via `infrastructure.kind.api_server_address` (default `127.0.0.1`) |
| Magnum | Owned by the Magnum cluster template | Not configured through openCenter's networking fields |

**Evidence:** `internal/config/v2/infrastructure.go`, `internal/config/v2/defaults.go`

## Plan the IP Addresses

Careful IP planning prevents conflicts and simplifies troubleshooting. The defaults below apply to OpenStack, VMware, and bare metal (Kind uses its own Docker-network defaults; Magnum has no node-network fields).

### Node Network Allocation

The default node network is `10.2.128.0/22`. Within it, the individual defaults are **not** contiguous in the way a single allocation-pool worksheet would suggest — the gateway, VRRP VIP, and allocation-pool start are three distinct, independently configurable addresses:

| Field | Default | Purpose |
| --- | --- | --- |
| `infrastructure.networking.subnet_nodes` | `10.2.128.0/22` | Node network CIDR |
| `infrastructure.networking.gateway` | `10.2.128.1` | Network gateway |
| `infrastructure.networking.vrrp_ip` | `10.2.128.5` | Kubernetes API VIP (when `vrrp_enabled: true`) |
| `infrastructure.networking.allocation_pool_start` | `10.2.128.10` | Start of the DHCP/allocation range used for node IPs |
| `infrastructure.networking.allocation_pool_end` | `10.2.131.250` | End of the allocation range |

Control plane and worker node counts (`master_count: 3`, `worker_count: 3` by default) draw their addresses from within the allocation pool; for VMware and bare metal, node IPs are instead defined explicitly as static nodes.

Example, keeping the defaults:

```yaml
opencenter:
  infrastructure:
    networking:
      subnet_nodes: "10.2.128.0/22"
      gateway: "10.2.128.1"
      vrrp_ip: "10.2.128.5"
      allocation_pool_start: "10.2.128.10"
      allocation_pool_end: "10.2.131.250"
      vrrp_enabled: true
```

### Pod and Service Networks

Keep defaults unless they conflict with existing infrastructure:

```yaml
opencenter:
  cluster:
    kubernetes:
      subnet_pods: "10.42.0.0/16"
      subnet_services: "10.43.0.0/16"
```

If your corporate network uses `10.42.x.x` or `10.43.x.x`, shift to non-conflicting ranges:

```yaml
opencenter:
  cluster:
    kubernetes:
      subnet_pods: "10.244.0.0/16"
      subnet_services: "10.245.0.0/16"
```

### DNS and NTP

Every cluster requires at least one DNS nameserver and one NTP server:

```yaml
opencenter:
  infrastructure:
    networking:
      dns_nameservers:
        - "10.0.0.53"        # Internal DNS preferred
        - "8.8.8.8"          # Fallback
      ntp_servers:
        - "time.example.com" # Internal NTP preferred
        - "0.pool.ntp.org"   # Fallback
```

Use internal DNS and NTP servers when available. Public servers are acceptable for development but introduce an external dependency in production. `dns_nameservers` requires at least one entry validated as IPv4; both fields live under `opencenter.infrastructure.networking`, not under `cluster`.

**Evidence:** `internal/config/v2/infrastructure.go`, `internal/config/defaults/openstack.go` (region-specific NTP/DNS defaults)

## Add-ons and Default Service Set

The schema defines named platform services under `opencenter.services.*`. At
`cluster init` time, `internal/config/v2/defaults.go` materializes a subset with
real defaults; the rest exist in the schema but are not part of the generated
default set. The table below is split by that real distinction rather than by
an unverified "GA vs. preview" maturity label.

### Enabled by default (OpenStack / VMware / bare metal)

| Service | Purpose | Notes |
| --- | --- | --- |
| calico | CNI, network policy | Default CNI |
| cert-manager | TLS certificate lifecycle | `letsencrypt_server` defaults to the public Let's Encrypt ACME endpoint |
| external-snapshotter | VolumeSnapshot CRDs | Works with CSI drivers that support snapshots |
| fluxcd, sources | GitOps reconciliation engine | |
| gateway, gateway-api | Gateway API ingress (Envoy) | |
| headlamp | Kubernetes dashboard | Has real OIDC fields (`oidc_client_id`, `oidc_issuer_url`) |
| keycloak | OIDC identity provider | |
| postgres-operator | Database operator (used by Keycloak) | |
| rbac-manager | Converts Keycloak groups to cluster RBAC | Policy content not in this repository (see below) |
| kube-prometheus-stack | Prometheus, Grafana, Alertmanager | |
| kyverno | Policy engine | Policy content not in this repository (see below) |
| loki | Log aggregation | |
| tempo | Distributed tracing | |
| olm | Operator Lifecycle Manager | |
| openstack-ccm, openstack-csi | OpenStack cloud provider integration | Enabled only for `openstack`; explicitly disabled by defaults for `kind`, `baremetal`, `vmware` |
| velero | Application backup | **Enabled by default only for OpenStack** — disabled by default for `kind`, `baremetal`, and `vmware` |

### Present in the schema but disabled by default everywhere

These require an explicit `enabled: true` after `cluster init`:

| Service | Purpose |
| --- | --- |
| etcd-backup | etcd snapshot CronJob (see [Business Continuity Decisions](#business-continuity-decisions)) |
| metallb | Load balancer for environments without a cloud LB |
| longhorn | Distributed block storage |
| harbor | Self-hosted container registry |
| vsphere-csi | vSphere CSI platform-service entry (distinct from the automatically-enabled `cluster.kubernetes.storage_plugin.vsphere_csi` in-cluster plugin) |
| weave-gitops | Web UI for FluxCD |
| mimir | Metrics long-term storage |
| opentelemetry-kube-stack | OpenTelemetry Collector |
| sealed-secrets | Alternative secret sealing |
| kafka-cluster | Kafka cluster operator |

### CNI alternatives (Calico is default)

Cilium and Kube-OVN are real, schema-backed CNI options under
`cluster.kubernetes.network_plugin.{cilium,kube-ovn}`, but neither is part of
`defaultServiceMap`/`NewDefaultServiceConfig`'s default set. In practice this
means: Calico is what a new cluster gets unless you explicitly configure a
different CNI plugin.

### Not implemented in this codebase

A grep of this repository found no trace of the following, so they are omitted rather than described as roadmap items: Istio, Talos Linux (in fact, `opencenter.talos` is explicitly rejected — `internal/config/v2/validator.go`: `"opencenter.talos is not supported in v2; remove the opencenter.talos section"` — and `internal/gitops/copy.go` skips any `talos/` template directory with the comment "Talos is no longer supported"), `openCenter-AirGap`, and `openCenter-customer-app-example`. Windows worker pools, by contrast, **are** real and implemented (see [Configure Compute for the Base Cluster](#configure-compute-for-the-base-cluster)) — only the separate `opencenter-windows` Ansible-collection repository referenced by some historical docs has no trace in this codebase.

**Evidence:** `internal/config/v2/defaults.go` (`NewDefaultServiceConfig`, `defaultServiceMap`, `applyProviderBehaviorDefaults`), `internal/config/v2/validator.go`, `internal/gitops/copy.go`, `schema/opencenter-v2.schema.json`

## Container Image Reference

Each service in the schema has a generic `image: {repository, tag}` override (for example `opencenter.services.calico.image.repository`), but this repository does not contain the default repository/tag values themselves, nor any registry-mirroring or air-gap tooling. Concretely:

* A repo-wide search found no `registry.k8s.io`/`ghcr.io`/`quay.io`/`docker.io` image defaults hardcoded in `internal/config/services/` or `internal/gitops/templates/` — actual default images and version pins live in the external `openCenter-gitops-base` repository that FluxCD pulls from, not in this CLI.
* There is no `openCenter-AirGap` reference, no bastion-registry-mirroring code, and no `image_repository`/`image_tag` flat fields (the real shape is the nested `image.repository`/`image.tag` object shown above).
* The one registry that does exist in this repository is Kind's own optional local registry (`infrastructure.kind.registry`, disabled by default, default port `5001`) — this is for local development, not air-gapped production mirroring.

Example of the real per-service override shape (values below are illustrative, not verified defaults):

```yaml
opencenter:
  services:
    calico:
      image:
        repository: "<your-mirror>/calico/node"
        tag: "<pinned-version>"
```

**Evidence:** `schema/opencenter-v2.schema.json` (per-service `image` object), `internal/config/v2/infrastructure.go` (`KindRegistryConfig`)

## Configure Compute for the Base Cluster

### Node Sizing

openCenter uses provider "flavors" (instance types), configured under `opencenter.infrastructure.compute.{flavor_master,flavor_worker,flavor_bastion}` — not under `cluster.kubernetes`. For OpenStack, the region-defaults registry names its flavors with an implied vCPU/RAM shape (`internal/config/defaults/openstack.go`), for example both the `sjc3` and `dfw3` regions default to:

| Role | Default flavor | Implied sizing |
| --- | --- | --- |
| Bastion | `gp.5.2.4` | 2 vCPU / 4 GB (naming convention) |
| Control plane | `gp.5.4.8` | 4 vCPU / 8 GB |
| Worker | `gp.5.4.16` | 4 vCPU / 16 GB |
| Windows worker | `gp.5.4.16` | 4 vCPU / 16 GB |

Boot volume size defaults to 40 GB for both control plane and worker nodes (`infrastructure.storage.master_volume_size`/`worker_volume_size`); there is no separate bastion volume-size field. VMware and bare metal have no numeric vCPU/RAM defaults in this repository — their flavor/image fields are free strings supplied by the operator (VMware defaults to placeholder names like `vmware-master`/`vmware-worker`; bare metal has no default bastion flavor since `bastion.enabled` defaults to `false`).

### Node Counts

`master_count`, `worker_count`, and `worker_count_windows` (`opencenter.infrastructure.compute`) are all validated with `min=0` only — the schema and Go validation tags define no maximum:

| Role | Schema minimum | `cluster init` default |
| --- | --- | --- |
| Control plane | 0 | 3 |
| Workers | 0 | 3 |
| Windows workers | 0 | 0 |

The live Kubespray bootstrap requires at least one control-plane node; Kind has
its own provider-specific node defaults. Deployment-method validation also
contains configuration-only method values that are not wired to a bootstrap
provider, so those values should not be read as supported deploy paths.

### Additional Worker Pools

`opencenter.infrastructure.compute.additional_server_pools_worker` is an array whose items require `name` and `flavor`; the other real fields are `count`, `image`, `labels` (map of strings), `taints` (each with required `key`/`effect`), `boot_volume` (with required `size`, plus `destination_type`/`source_type`/`type`/`delete_on_termination`), and `additional_volumes`. There is **no** `worker_count`, `flavor_worker`, `node_worker`, `server_group_affinity`, or `worker_node_bfv_volume_size` field on this object — those names do not exist in `schema/opencenter-v2.schema.json`:

```yaml
opencenter:
  infrastructure:
    compute:
      additional_server_pools_worker:
        - name: gpu-workers
          flavor: "gpu.large"
          count: 2
          boot_volume:
            size: 200
          labels:
            workload: gpu
          taints:
            - key: "nvidia.com/gpu"
              effect: "NoSchedule"
```

The Windows-specific equivalent, `additional_server_pools_worker_windows`, has the same shape plus a per-pool `server_group_affinity` string (`affinity`\|`anti-affinity`\|`soft-affinity`\|`soft-anti-affinity`) — that field exists only on the Windows pool item, not on the regular worker pool item.

### Server Group Affinity

Worker and control-plane nodes each have their own affinity field under `compute`, not per pool:

```yaml
opencenter:
  infrastructure:
    compute:
      worker_group_affinity:
        - "soft-anti-affinity"   # cluster init default
      cp_server_group_affinity:
        - "soft-anti-affinity"   # cluster init default
```

Both are `[]string`; `cluster init` sets each to `["soft-anti-affinity"]` by default for OpenStack/VMware/bare metal.

* `compute.worker_group_affinity` renders to `locals.wn_server_group_affinity` in the OpenStack `main.tf` and is forwarded to the `openstack-nova` module as `wn_server_group_affinity`.
* `compute.cp_server_group_affinity` renders to `locals.cp_server_group_affinity` and is forwarded to the `openstack-nova` module as `cp_server_group_affinity`.

Both accept the same values (`affinity`, `anti-affinity`, `soft-affinity`, `soft-anti-affinity`).

**Evidence:** `internal/config/v2/infrastructure.go`, `internal/config/v2/defaults.go`, `internal/gitops/templates/infrastructure-cluster-template/main-default.tf.tpl`, `schema/opencenter-v2.schema.json`

## Integrate OIDC for the Cluster

openCenter integrates Keycloak as the OIDC identity provider for the Kubernetes API server. This enables group-based RBAC without managing individual kubeconfig files.

### How It Works

```
User → Keycloak (authenticate) → ID Token (JWT)
  → kubectl (--token) → API Server (validate JWT)
    → RBAC (map groups to roles)
```

### Kubernetes API Server OIDC Configuration

The API-server-facing OIDC fields live at `opencenter.cluster.kubernetes.oidc` (not under `opencenter.oidc` — there is no such top-level block):

```yaml
opencenter:
  cluster:
    kubernetes:
      oidc:
        enabled: true
        kube_oidc_url: "https://auth.<cluster>.<region>.k8s.opencenter.cloud/realms/opencenter"
        kube_oidc_client_id: "kubernetes"
        kube_oidc_username_claim: "preferred_username"
        kube_oidc_username_prefix: "oidc:"
        kube_oidc_groups_claim: "groups"
        kube_oidc_groups_prefix: "oidc:"
        kube_oidc_ca_file: "/etc/kubernetes/pki/oidc-ca.pem"
```

The same `oidc` object also carries a second, generic set of fields (`client_id`, `client_secret`, `issuer_url`, `username_claim`, `groups_claim`) alongside the `kube_oidc_*` fields — both live in the single `OIDCConfig` struct (`internal/config/v2/cluster.go`).

### Keycloak Groups and RBAC

Keycloak's realm template provisions a `cluster-admins` group (`internal/gitops/templates/cluster-apps-base/services/keycloak/20-keycloak/opencenter-realm.yaml.tpl`). RBAC Manager (`services.rbac-manager`, enabled by default) is described as converting Keycloak groups into cluster RBAC, but this repository contains no RBAC Manager templates or CRD definitions to verify the specific mapping mechanism or additional default groups (for example a `viewers`/`view` mapping) — that content, if it exists, lives outside this codebase. Treat `cluster-admins` as the one verifiable default group.

**Evidence:** `internal/config/v2/cluster.go` (`OIDCConfig`), `internal/gitops/templates/cluster-apps-base/services/keycloak/20-keycloak/opencenter-realm.yaml.tpl`

## Integrate OIDC for the Workload

There is no top-level `opencenter.oidc` block with `client_id`/`secret_name`/`scopes`/`logout_path` fields — that path does not exist in the schema. The real, verifiable workload-OIDC surface is smaller:

* `opencenter.identity.oidc` — `enabled` (bool), `provider` (`keycloak`\|`entra`\|`generic`), `source` (`internal`\|`external`). This is the platform-wide identity provider selection, not a per-service SSO block.
* `opencenter.services.headlamp.oidc_client_id` / `oidc_issuer_url` — Headlamp is the one service in the schema with its own OIDC fields.

No other service in `schema/opencenter-v2.schema.json` (including `kube-prometheus-stack`/Grafana, `weave-gitops`, and `gateway`) has an `oidc_*` field, so claims of Grafana SSO via kube-prometheus-stack values, Weave GitOps OIDC, or Gateway-API-enforced OIDC authentication are not verifiable in this repository and have been removed rather than asserted.

```yaml
opencenter:
  identity:
    oidc:
      enabled: true
      provider: keycloak
      source: internal
  services:
    headlamp:
      oidc_client_id: "headlamp"
      oidc_issuer_url: "https://auth.<cluster>.<region>.k8s.opencenter.cloud/realms/opencenter"
```

**Evidence:** `schema/opencenter-v2.schema.json` (`opencenter.identity.oidc`, `opencenter.services.headlamp`), `internal/config/v2/readiness.go` (`cfg.OpenCenter.Identity.OIDC`)

## Select a Networking Model

### CNI Selection

`opencenter.cluster.kubernetes.network_plugin` has three sub-objects — `calico`, `cilium`, `kube-ovn` — each independently toggled by its own `enabled` flag. Calico is the only one materialized into the default config; Cilium and Kube-OVN have real, tunable configuration (see below) but are not part of `cluster init`'s generated defaults, so selecting either is an explicit operator choice, not a documented "preview" tier:

| CNI | Config path | Tunable fields | kube-proxy |
| --- | --- | --- | --- |
| Calico (default) | `network_plugin.calico` | `ipip_mode`/`vxlan_mode` (`Always`\|`CrossSubnet`\|`Never`), `encapsulation_type`, `cni_iface`, `nat_outgoing` | Standard |
| Cilium | `network_plugin.cilium` | `tunnel_mode` (`vxlan`\|`geneve`\|`disabled`), `hubble`, `operator_enabled`, `kube_proxy_replacement` | Replaceable via `kube_proxy_replacement` |
| Kube-OVN | `network_plugin.kube-ovn` | `cilium_integration`, `network_policy` | Standard |

Separately, `opencenter.services.calico`/`cilium`/`kube-ovn` are platform-service entries (enable/version/namespace) distinct from this per-CNI tuning object; only `services.calico` is in the default service set.

### Calico Encapsulation Modes

`ipip_mode`/`vxlan_mode` each accept `Always`, `CrossSubnet`, or `Never` — setting both to `Never` (with an appropriate `encapsulation_type`) is how a BGP/no-encapsulation setup is expressed. There is no separate boolean "BGP mode" flag; it falls out of the IPIP/VXLAN mode combination.

### Load Balancer Provider

`infrastructure.networking.loadbalancer_provider` is a single enum field, not an independent per-environment choice — valid values are `ovn` (default), `octavia`, `metallb`, and `cloud-native`. Whichever value is set there is orthogonal to whether the `services.metallb` platform service is enabled (it defaults to `disabled` and must be turned on explicitly if `metallb` is the chosen load-balancer provider).

**Evidence:** `internal/config/v2/cluster.go` (`NetworkPluginConfig`, `CalicoConfig`, `CiliumConfig`, `KubeOVNConfig`), `internal/config/v2/infrastructure.go` (`LoadbalancerProvider`), `schema/opencenter-v2.schema.json`, `internal/gitops/render_catalog.go`

## Deploy Ingress Resources

### Gateway API (Recommended)

openCenter uses Gateway API as the standard ingress model. Two related services exist: `services.gateway` (namespace `gateway`, has `default_issuer`/`gateway_class`/`gateway_name`/`gateway_namespace`/`listeners` fields) and `services.gateway-api` (namespace `envoy-gateway-system`). Both default to enabled.

```
Internet → LoadBalancer (per loadbalancer_provider)
              → Gateway (namespace: gateway)
                  → HTTPRoute (per-service routing)
                      → Service → Pods
```

### Hostname Convention

Several services derive their hostname from `cluster_fqdn` directly, for example Headlamp defaults to `dashboard.<cluster_fqdn>` and Keycloak to `auth.<cluster_fqdn>` (`internal/config/v2/defaults.go`). `cluster_fqdn` itself defaults to:

```
<cluster-name>.<region>.k8s.opencenter.cloud
```

(`defaultBaseDomain = "k8s.opencenter.cloud"`; there is no separate "org" segment baked into the default — `base_domain`/`cluster_fqdn` are both free-form FQDN-validated strings that can be fully customized):

```yaml
opencenter:
  cluster:
    base_domain: "k8s.opencenter.cloud"
    cluster_fqdn: "my-cluster.sjc3.k8s.opencenter.cloud"
```

### TLS Certificates

cert-manager provisions TLS certificates and defaults to the public Let's Encrypt ACME endpoint:

```yaml
opencenter:
  services:
    cert-manager:
      enabled: true
      letsencrypt_server: "https://acme-v02.api.letsencrypt.org/directory"
```

For DNS-01 ACME challenges, `cert-manager` has real `dns_provider` (`route53`\|`designate`\|`cloudflare`\|`clouddns`\|`azuredns`) and `dns_zones` fields — OpenStack Designate is one of the supported DNS-01 providers, consistent with the optional Designate DNS integration mentioned for OpenStack above. There is no `ca_certificates` field anywhere in the schema for supplying a custom internal CA bundle to cert-manager; that specific capability is not verifiable in this repository.

**Evidence:** `internal/config/services/cert_manager.go`, `internal/config/v2/defaults.go`, `schema/opencenter-v2.schema.json` (`opencenter.services.gateway`, `gateway-api`)

## Secure the Network Flow

This section summarizes the cluster- and platform-layer controls also covered in [Security Model](security-model.md); see that document for the full layered model and for what is and is not verifiable in this repository.

### Layer 1: Pod Security Admission (Cluster-Level)

`opencenter.cluster.kubernetes.security.pod_security_standards` is a single string field with values `privileged`, `baseline`, or `restricted` (`cluster init` defaults to `"baseline"`). There is no separate enforce/audit/warn matrix in this schema — that would be a Kubernetes-native Pod Security Admission behavior layered on top of a single configured level, not something this field itself encodes.

Namespaces can be exempted via `pod_security_exemptions`; when left unset, the generated Terraform/Ansible inputs default the exemption list to `["trivy-temp"]` (`internal/gitops/templates/infrastructure-cluster-template/main-default.tf.tpl` and the VMware/bare-metal variants):

```yaml
opencenter:
  cluster:
    kubernetes:
      security:
        pod_security_standards: "baseline"
        k8s_hardening: true
        pod_security_exemptions:
          - "kube-system"
          - "flux-system"
```

### Layer 2: Kyverno Policies (Resource-Level)

Kyverno (`services.kyverno`) is enabled by default, but this repository contains no Kyverno `ClusterPolicy` templates — there is no `internal/gitops/templates/.../kyverno/` directory. Policy content is deployed from the separate `openCenter-gitops-base` repository. This document does not assert a specific policy count or list; see [Security Model](security-model.md) for the same caveat applied consistently.

### Layer 3: NetworkPolicies

Calico, Cilium, and Kube-OVN each have a real `network_policy` boolean field (`internal/config/v2/cluster.go`), so NetworkPolicy enforcement is CNI-level and configurable. Specific NetworkPolicy manifests for platform services are not verifiable in this repository; this document does not claim which services ship with them.

### Layer 4: OS Hardening

`infrastructure.networking.security.os_hardening` (bool, default `true` in the generated Terraform) is passed into the bare-metal and VMware bootstrap templates as `os_hardening_enabled` (`internal/gitops/templates/infrastructure-cluster-template/main-baremetal.tf.tpl`, `main-vmware.tf.tpl`, `main-default.tf.tpl`). The specific hardening actions it triggers (firewall rules, sysctl settings, SSH policy) are implemented in the Kubespray/Ansible layer invoked by these templates, not in this CLI's Go code, so this document does not enumerate them as verified facts.

**Evidence:** `internal/config/v2/cluster.go`, `internal/gitops/templates/infrastructure-cluster-template/`, `docs/concepts/security-model.md`

## Add Secret Management

### Encryption Model

openCenter uses a dual-encryption strategy:

```
Developer writes secret → SOPS encrypts (Age key) → Git commit (ciphertext)
  → FluxCD pulls → SOPS decrypts (Age key in cluster) → Kubernetes Secret
    → etcd stores (encrypted at rest)
```

### SOPS Age Key Lifecycle

| Key Type | Default expiration | Local storage |
| --- | --- | --- |
| Age encryption key | 90 days | `<secrets-dir>/<organization>/<cluster>/age/keys/<cluster>-key.txt` |
| SSH deploy key | 180 days | `<secrets-dir>/<organization>/<cluster>/ssh/<cluster>` (with `.pub` alongside it) |

Rotation uses a dual-key strategy: the new key encrypts new secrets while the old key remains valid for decryption, ensuring zero-downtime rotation. See [Security Model](security-model.md) for the full key-registry and rotation-state model.

### Configuration

The cluster-config SOPS block lives at `secrets.sops` and does not take an `age_keys` list — it references a single key file:

```yaml
secrets:
  sops:
    enabled: true
    age_key_file: "/path/to/age-key.txt"
```

### Key Management Commands

```bash
opencenter secrets keys check --cluster my-cluster      # Monitor key expiration
opencenter secrets keys rotate --cluster my-cluster --type age # Rotate encryption keys
opencenter secrets validate my-cluster                  # Detect configuration drift
opencenter secrets sync my-cluster                      # Synchronize secrets
```

**Evidence:** `internal/sops/manager.go`, `docs/concepts/security-model.md`

## Workload Storage

### CSI Driver Selection

There are two distinct storage-plugin surfaces: `cluster.kubernetes.storage_plugin` (the in-cluster CSI plugin, enabled automatically based on provider) and `opencenter.infrastructure.storage.default_storage_class` (the default `StorageClass` name). Neither is named `storage_plugin.longhorn` — Longhorn is a separate top-level platform service (`opencenter.services.longhorn`), not a `storage_plugin` entry; `storage_plugin`'s real children are `aws_ebs_csi`, `azure_disk_csi`, `ceph`, `cinder_csi`, `gcp_compute_csi`, `trident`, and `vsphere_csi`.

| Provider | In-cluster CSI plugin | Default storage class |
| --- | --- | --- |
| OpenStack | `storage_plugin.cinder_csi` (auto-enabled) | `csi-cinder-sc-delete` |
| VMware | `storage_plugin.vsphere_csi` (auto-enabled) | `vsphere-csi` |
| Bare metal | None auto-enabled; `services.longhorn` is a reasonable choice but defaults to disabled | `standard` |
| Kind | Kind's built-in local-path provisioner | `standard` |

### Longhorn (Distributed Storage)

Longhorn provides replicated block storage for environments without a cloud storage backend. It is a platform service (`opencenter.services.longhorn`) with real fields `enabled`, `hostname`, `default_data_path`, `default_replica_count`, `storage_over_provisioning_percentage`, `storage_minimal_available_percentage`, `backup_target`, and `backup_target_credential_secret`. It defaults to **disabled** for every provider and must be explicitly enabled:

```yaml
opencenter:
  services:
    longhorn:
      enabled: true
```

### Volume Snapshots

The `external-snapshotter` service (enabled by default) provides VolumeSnapshot CRDs. It works with any CSI driver that supports snapshots.

### Boot Volume Configuration

Node boot volumes are configured under `opencenter.infrastructure.storage`, and the fields are separate per node role (`worker_*` and `master_*`), not shared:

```yaml
opencenter:
  infrastructure:
    storage:
      default_storage_class: "csi-cinder-sc-delete"   # provider-specific default; see table above
      worker_volume_size: 40                            # GB, cluster init default
      worker_volume_destination_type: "volume"          # volume|local
      worker_volume_source_type: "image"                # image|volume|snapshot
      worker_volume_type: "HA-Standard"                  # provider-specific default; free-text, not an enum
      worker_volume_delete_on_termination: false
      master_volume_size: 40
      master_volume_destination_type: "volume"
      master_volume_source_type: "image"
      master_volume_type: "HA-Standard"
```

`worker_volume_type`'s `cluster init` default is `"HA-Standard"` for OpenStack, `"vsphere-default"` for VMware, `"local-path"` for Kind, and `"Performance"` for bare metal — it is a free-text field validated only as `required`, not a fixed enum, so other values (like a hypothetical `"HA-Performance"`) are not schema-enforced defaults, just whatever your provider's catalog offers.

**Evidence:** `internal/config/v2/infrastructure.go`, `internal/config/v2/cluster.go` (`StoragePluginConfig`), `internal/config/v2/defaults.go` (`defaultStorageClass`, `defaultStorageType`), `schema/opencenter-v2.schema.json`

## Policy Management

### Policy Layers

openCenter enforces policy at two independent layers:

| Layer | Engine | Scope | Notes |
| --- | --- | --- | --- |
| Cluster | Pod Security Admission | Namespace-level | Configured via the single `pod_security_standards` field (`privileged`\|`baseline`\|`restricted`) |
| Resource | Kyverno | Resource-level | `services.kyverno` is enabled by default, but its `ClusterPolicy` content is not in this repository |

### Kyverno Policy Content

This repository has no Kyverno `ClusterPolicy` templates and no count of default policies to verify — there is no `.../kyverno/` template directory anywhere under `internal/gitops/templates/`. Whatever baseline ruleset is deployed comes from the external `openCenter-gitops-base` repository that FluxCD reconciles from. This document intentionally does not restate a specific policy list or count; see [Security Model](security-model.md) for the same treatment.

### Custom Policies

Per-cluster overlays follow the pattern `applications/overlays/<cluster>/services/<service>/...` (confirmed for Calico's Helm-values override, `internal/cluster/openstack_network_plugin.go`; the general glob `applications/overlays/[^/]+/(managed-services|services)/.*/.*\.ya?ml$` is used for SOPS encryption-path matching in `internal/core/validation/validators/gitops.go` and `internal/config/flags/sops_integration.go`). Following that same convention, a `kyverno` service overlay would live under `applications/overlays/<cluster>/services/kyverno/`, but this repository does not itself define or ship any Kyverno policy files there — that content is external.

### Policy Exemptions

Namespaces that need elevated privileges are exempted at the Pod Security Admission level via `pod_security_exemptions`; when left unset, the generated infrastructure templates default this list to `["trivy-temp"]`.

**Evidence:** `internal/config/v2/cluster.go`, `internal/gitops/templates/infrastructure-cluster-template/`, `internal/cluster/openstack_network_plugin.go`, `internal/core/validation/validators/gitops.go`, `docs/concepts/security-model.md`

## Node and Pod Scalability

### Horizontal Scaling

`worker_count` lives under `opencenter.infrastructure.compute`, not `opencenter.cluster.kubernetes`:

```yaml
opencenter:
  infrastructure:
    compute:
      worker_count: 5  # increase from the cluster init default of 3
```

For OpenStack, new VMs are provisioned automatically through OpenTofu. For VMware and bare metal, pre-provision the hosts and add them as static nodes in configuration.

### Additional Worker Pools

Separate pools allow different flavors, images, labels, and taints for different workloads — using the real field names (`name`, `flavor`, `count`; see [Configure Compute for the Base Cluster](#configure-compute-for-the-base-cluster) for the full shape):

```yaml
opencenter:
  infrastructure:
    compute:
      additional_server_pools_worker:
        - name: memory-optimized
          flavor: "m1.xlarge"
          count: 3
```

### Scaling Limits

`master_count`, `worker_count`, and `worker_count_windows` are each validated only with `min=0` in both the Go struct tags and the JSON schema — there is **no** schema-enforced maximum (no "100 control plane / 1,000 workers / 100 Windows workers" cap exists in this codebase). Practical ceilings come from your provider's quotas and from Kubespray/etcd scaling characteristics, not from openCenter's own validation.

### Pod Density

Pod-per-node density is a general Kubernetes/kubelet concern (the upstream default is 110 pods per node) rather than something this repository configures directly; adjusting it would go through the Kubespray/Ansible layer this CLI's bootstrap templates invoke, which is out of scope for this repository's Go code.

**Evidence:** `internal/config/v2/infrastructure.go`, `schema/opencenter-v2.schema.json`

## Business Continuity Decisions

### Backup Strategy

openCenter provides two backup mechanisms, and **neither is enabled by default for every provider**:

| Mechanism | What It Backs Up | Default enablement | Schedule | Retention |
| --- | --- | --- | --- | --- |
| etcd-backup (`services.etcd-backup`) | etcd snapshot via `etcdctl snapshot save`, uploaded to S3-compatible storage | Disabled by default for every provider | `"0 1 * * *"` (daily at 01:00), hardcoded in the CronJob template — not user-configurable | Not managed by this repository's template; the CronJob only uploads a dated snapshot file, with no pruning step visible in `internal/gitops/templates/cluster-apps-base/services/etcd-backup/` |
| Velero (`services.velero`) | Application resources + persistent volumes | **Enabled by default only for OpenStack**; explicitly disabled by default for `kind`, `baremetal`, and `vmware` (`internal/config/v2/defaults.go`) | Not defined in this repository — Velero's manifests live in the external `openCenter-gitops-base` repo | Not defined in this repository |

Both services require S3 (or, for Loki/Tempo elsewhere, Swift) credentials configured through their respective `s3_*`/`backup_bucket` schema fields before they can actually run.

### Recovery Mechanisms

This document does not assert specific recovery-time figures (RTOs), since none are defined anywhere in this repository. The verifiable mechanisms are:

* **Pod/node failure** — handled by ordinary Kubernetes scheduling and self-healing; no openCenter-specific mechanism.
* **etcd loss** — restorable from an etcd-backup snapshot, if the service was enabled and credentials were configured before the loss occurred.
* **Application-data loss** — restorable via Velero, if enabled (OpenStack only, by default) and configured.
* **Full cluster loss** — rebuildable from the GitOps repository plus whatever backups were actually running; see below.

### GitOps as Disaster Recovery

Because cluster configuration lives in Git, a rebuild follows the same steps as an initial deploy:

1. Provision infrastructure (OpenTofu, for OpenStack/VMware/bare metal) or recreate the Magnum cluster
2. Deploy Kubernetes through the selected live provider path (Kubespray, Kind, or Magnum)
3. Bootstrap FluxCD, pointing at the same Git repository
4. FluxCD reconciles all services and applications
5. Restore persistent data from etcd-backup/Velero backups, if those were enabled and running

The Git repository is the recovery artifact for configuration; it is not, by itself, a backup of application data unless a backup mechanism was actually enabled.

### Multi-Region Considerations

For multi-region deployments, each region gets its own cluster. Shared platform-service content is pulled from the external `openCenter-gitops-base` repository at a pinned release (`defaultGitBaseRepoURL`/`defaultGitBaseRepoRelease` in `internal/config/v2/defaults.go`); region-specific configuration lives in each cluster's own generated overlay.

**Evidence:** `internal/config/v2/defaults.go`, `internal/gitops/templates/cluster-apps-base/services/etcd-backup/cronjob.yaml`, `internal/config/services/velero.go`

## Monitor and Collect Logs and Metrics

### Observability Stack

kube-prometheus-stack, Loki, and Tempo are all part of the default service set (enabled by default for OpenStack/VMware/bare metal):

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ Prometheus   │    │ Loki         │    │ Tempo        │
│ (Metrics)    │    │ (Logs)       │    │ (Traces)     │
│              │    │              │    │              │
│ kube-prometheus- │ S3 or Swift  │    │ OTLP spans,  │
│ stack service │  │ storage      │    │ S3/Swift     │
└──────┬───────┘    └──────┬───────┘    └──────┬───────┘
       │                   │                   │
       └───────────┬───────┘───────────────────┘
                   │
            ┌──────▼───────┐
            │   Grafana    │
            │ (Dashboards) │
            └──────────────┘
```

No scrape-interval, retention-period, or Grafana-OIDC-SSO field was found in the `kube-prometheus-stack` schema entry, so this document does not assert defaults for those.

### Metrics (kube-prometheus-stack)

Real fields (`opencenter.services.kube-prometheus-stack`): `enabled`, `prometheus_hostname`/`grafana_hostname`/`alertmanager_hostname`, `prometheus_volume_size`/`grafana_volume_size`/`alertmanager_volume_size`, `prometheus_storage_class`/`grafana_storage_class`/`alertmanager_storage_class`, and `webhook_url` (for Alertmanager's generic webhook receiver):

```yaml
opencenter:
  services:
    kube-prometheus-stack:
      enabled: true
      prometheus_volume_size: 50       # GB — illustrative, not a verified default
      prometheus_storage_class: "csi-cinder-sc-delete"
      grafana_volume_size: 10
      alertmanager_volume_size: 10
      webhook_url: "https://alerts.example.com/webhook"
```

### Logs (Loki)

The real field is `storage_type` (not `loki_storage_type`), with values `s3` or `swift` plus matching `s3_*`/`swift_*` credential fields:

```yaml
opencenter:
  services:
    loki:
      storage_type: "swift"   # or "s3"
      volume_size: 50
```

### Traces (Tempo)

Tempo (`opencenter.services.tempo`) has the same `storage_type`/`s3_*`/`swift_*` shape as Loki, for receiving and storing OTLP spans.

### OpenTelemetry

`opentelemetry-kube-stack` is a real service (`enabled`, `collector_mode`, `collector_replicas`, `exporters`, `processors`) but defaults to **disabled**.

**Evidence:** `schema/opencenter-v2.schema.json` (`opencenter.services.kube-prometheus-stack`, `loki`, `tempo`, `opentelemetry-kube-stack`)

## Cluster and Workload Operations

### Lifecycle Commands

| Stage | Command | What It Does |
| --- | --- | --- |
| Initialize | `opencenter cluster init` | Creates configuration file with defaults |
| Edit | `opencenter cluster edit` | Opens configuration in editor |
| Validate | `opencenter cluster validate` | Schema + business rules + provider checks |
| Setup | `opencenter cluster generate` | Generates GitOps repository |
| Bootstrap | `opencenter cluster deploy` | Deploys FluxCD, starts reconciliation |
| Status | `opencenter cluster status` | Shows cluster and service health |
| Destroy | `opencenter cluster destroy` | Tears down infrastructure |

### Drift Detection

`opencenter cluster drift` is a command group with `detect`, `reconcile`, and `schedule` subcommands:

```bash
opencenter cluster drift detect my-cluster
```

| Provider | Drift Detection | Auto-Reconcile |
| --- | --- | --- |
| OpenStack | Yes | Limited (per `docs/reference/providers.md`) |
| VMware | Yes | No |
| Magnum | No | No |
| Bare metal | No (no cloud-state drift backend) | No |
| Kind | Not applicable | Not applicable |

FluxCD handles application-level drift automatically by continuously reconciling Git state to the cluster; this is separate from the infrastructure-level drift command above.

### Day-2 Operations

| Operation | Method |
| --- | --- |
| Upgrade Kubernetes | Update `cluster.kubernetes.version`, re-run `cluster generate`/`cluster deploy` |
| Add workers | Update `infrastructure.compute.worker_count`, re-run `cluster generate`/`cluster deploy` |
| Enable/disable services | Toggle a service's `enabled` flag, commit, FluxCD reconciles |
| Rotate secrets | `opencenter secrets keys rotate --cluster <cluster> --type age` |
| Backup | Only runs if `services.etcd-backup` and/or `services.velero` were explicitly enabled and configured with storage credentials — see [Business Continuity Decisions](#business-continuity-decisions) |
| Restore | Upstream `velero restore create --from-backup <name>` (Velero's own CLI, not an `opencenter` subcommand) |

**Evidence:** `cmd/cluster_init.go`, `cmd/cluster_edit.go`, `cmd/cluster_validate.go`, `cmd/cluster_generate.go`, `cmd/cluster_deploy.go`, `cmd/cluster_status.go`, `cmd/cluster_destroy.go`, `cmd/cluster_drift.go`

## Cost Management

openCenter does not include built-in cost management tooling. Cost optimization is an infrastructure-level concern that depends on the provider.

### Recommendations by Provider

| Provider | Cost Lever | Approach |
| --- | --- | --- |
| OpenStack | Instance flavors | Right-size `flavor_master`/`flavor_worker`/`flavor_bastion` for the workload; smaller custom flavors for dev/staging |
| OpenStack | Storage | Choose `worker_volume_type`/`master_volume_type` from your OpenStack catalog to match performance needs — this is a free-text field, not a fixed `HA-Standard`/`HA-Performance` enum in the schema |
| VMware | VM sizing | Align VM resources with actual utilization; monitor via kube-prometheus-stack, if enabled |
| Bare metal | Hardware utilization | Maximize pod density per node; use additional worker pools for burst capacity |
| All | Observability storage | Size `prometheus_volume_size`/`grafana_volume_size`/`alertmanager_volume_size`/Loki's `volume_size`/Tempo's storage deliberately — this repository does not set a default retention period for any of them |
| All | Backup | Only pay for etcd-backup/Velero storage if you actually enable them; neither is on by default outside OpenStack's Velero default |

### Resource Monitoring

Use the deployed kube-prometheus-stack to monitor resource utilization:

* Node CPU/memory utilization (node-exporter)
* Pod resource requests vs actual usage (kube-state-metrics)
* Persistent volume usage (kubelet metrics)

Right-size nodes and storage based on observed utilization, not initial estimates.

## Next Steps

* [Getting Started Tutorial](../getting-started/getting-started.md) -- Deploy your first cluster end-to-end
* [Configuration Schema Reference](../reference/configuration-schema.md) -- Complete field reference for the configuration file
* [Provider Comparison](provider-comparison.md) -- Detailed trade-offs between OpenStack, VMware, bare metal, and Kind
* [Security Model](security-model.md) -- Deep dive into the defense-in-depth security architecture
* [GitOps Workflow](gitops-workflow.md) -- How FluxCD reconciliation works
* [Configure Networking](../operations/configure-networking.md) -- Step-by-step networking configuration
* [Manage Secrets](../operations/manage-secrets.md) -- SOPS encryption and key rotation
* [Backup and Restore](../operations/backup-and-restore.md) -- Disaster recovery procedures
* [Customize Services](../operations/customize-services.md) -- Enable, disable, and configure platform services

## Related Resources

* `openCenter-gitops-base` -- external repository providing base platform-service manifests (Kyverno policy content, Velero manifests, and other content not present in this repository) that FluxCD reconciles from
* [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) -- Upstream PSS documentation
* [FluxCD Documentation](https://fluxcd.io/docs/) -- GitOps toolkit reference

This repository has no `.kiro/steering/` directory and no trace of `openCenter-AirGap`, `openCenter-customer-app-example`, or `opencenter-windows` — those repository names are not cited as evidence anywhere in this document.

---

## Evidence

This document is based on:

* Configuration types and validation: `internal/config/v2/*.go` (`cluster.go`, `infrastructure.go`, `defaults.go`, `validator.go`, `deployment_validator.go`, `readiness.go`)
* Configuration schema: `schema/opencenter-v2.schema.json`
* Provider-region defaults: `internal/config/defaults/openstack.go`, `interfaces.go`
* Provider implementations and boundaries: `internal/cloud/{openstack,vmware,kind,magnum}/`, `internal/cluster/provider/openstack/`, `internal/cluster/magnum_bootstrap_provider.go`
* CNI configuration and service rendering: `internal/config/v2/`, `internal/config/services/`, `internal/gitops/render_catalog.go`
* GitOps templates: `internal/gitops/templates/cluster-apps-base/`, `internal/gitops/templates/infrastructure-cluster-template/`, `internal/gitops/copy.go`
* SOPS/secrets management: `internal/sops/manager.go`, `internal/secrets/registry.go`, `rotation.go`
* CLI commands: `cmd/cluster_*.go`, `cmd/secrets_*.go`
* Sibling documentation (already verified this session, treated as consistent context): `docs/reference/providers.md`, `docs/CODEMAPS/providers.md`, `docs/concepts/security-model.md`, `docs/concepts/gitops-workflow.md`
