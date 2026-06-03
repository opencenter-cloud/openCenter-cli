---
id: reference-architecture
title: "Reference Architecture"
sidebar_label: Reference Architecture
description: Baseline infrastructure architecture, networking, security, and identity for openCenter deployments across bare metal, OpenStack, VMware, and Kind.
doc_type: explanation
audience: "architects, platform engineers, operators"
tags: [architecture, networking, security, identity, baseline, reference]
---
# Reference Architecture

**Purpose:** For architects and platform engineers, explains the baseline infrastructure architecture required to deploy openCenter, covering network topology, security, identity, storage, and operational concerns across bare metal, OpenStack, VMware, and Kind targets.

This document describes the recommended infrastructure foundation for openCenter deployments. It does not cover workload-specific architecture. Application teams should layer their own patterns on top of this baseline.

## Architecture

openCenter deploys a production-grade Kubernetes platform through a layered architecture. Each layer has a clear owner and a defined interface to the layer above it.

```
┌─────────────────────────────────────────────────────────┐
│  Layer 5: Platform Services (GitOps-managed)            │
│  cert-manager · Keycloak · Kyverno · Prometheus · Loki  │
│  Velero · Gateway API · RBAC Manager · Headlamp         │
├─────────────────────────────────────────────────────────┤
│  Layer 4: GitOps Engine (FluxCD)                        │
│  source-controller · kustomize-controller               │
│  helm-controller · notification-controller              │
├─────────────────────────────────────────────────────────┤
│  Layer 3: Kubernetes Cluster (Kubespray)                │
│  Control plane (3 nodes) · Workers (2+ nodes)           │
│  ContainerD · Calico/Cilium · CoreDNS                   │
├─────────────────────────────────────────────────────────┤
│  Layer 2: Infrastructure (OpenTofu / pre-provisioned)   │
│  Compute · Networking · Storage · Bastion               │
├─────────────────────────────────────────────────────────┤
│  Layer 1: Provider (OpenStack / VMware / Bare Metal)    │
│  Hypervisor · Physical network · Block storage          │
└─────────────────────────────────────────────────────────┘
```

### Design Principles

These principles shaped every architectural decision:

* **Configuration as code.** A single YAML file defines the entire cluster. No manual steps between configuration and deployment.
* **GitOps as the operational model.** Git is the source of truth. FluxCD reconciles desired state continuously. Changes flow through commits, not `kubectl apply`.
* **Defense in depth.** Security controls exist at every layer independently. Compromise of one layer does not compromise others.
* **Provider abstraction.** The same configuration structure works across OpenStack, VMware, bare metal, and Kind. Provider-specific details are isolated behind a unified interface.
* **Composition over duplication.** Base manifests live in `openCenter-gitops-base`. Clusters consume them via Kustomize overlays, adding only what differs.
* **Fail fast.** Multi-layered validation (schema → business rules → provider constraints → connectivity) catches errors before any infrastructure is provisioned.
* **Explicit dependencies.** Every service declares its dependencies. The platform deploys them in the correct order. No implicit coupling.

### Minimum Recommended Baseline

Most openCenter deployments should start with:

| Component | Recommendation |
| --- | --- |
| Control plane nodes | 3 (HA with VRRP or load balancer) |
| Worker nodes | 2 minimum, 3+ for production |
| Bastion host | 1 (SSH jump host, required) |
| CNI | Calico with VXLAN encapsulation |
| Load balancer | MetalLB (bare metal/VMware) or Octavia (OpenStack) |
| Ingress | Gateway API with Envoy |
| Identity | Keycloak with OIDC |
| Secrets | SOPS Age encryption + Kubernetes encryption at rest |
| Monitoring | kube-prometheus-stack + Loki + Tempo |
| Backup | Velero + etcd snapshots |
| Policy | Kyverno (17 default ClusterPolicies) |
| Storage | Provider CSI driver + Longhorn (optional) |
| Certificate management | cert-manager with Let’s Encrypt |

**Evidence:** `internal/config/defaults.go`, `docs/reference/platform-services.md`

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

* Automated VM provisioning via OpenTofu
* Cinder CSI for persistent volumes (default storage class: `csi-cinder-sc-delete`)
* Octavia load balancer or VRRP for API HA
* Optional Designate DNS integration
* Server group affinity policies (anti-affinity recommended for HA)
* Boot-from-volume with configurable size and type

**Evidence:** `internal/config/types_infrastructure.go`, `internal/config/types_cluster.go`

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

* VMs pre-provisioned by infrastructure team (static IPs in config)
* vSphere CSI driver for persistent volumes
* MetalLB for LoadBalancer services (no cloud LB)
* VRRP or kube-vip for API server HA
* Node IPs defined explicitly in `master_nodes` and `worker_nodes` arrays
* Drift detection available (detect only, no auto-reconcile)

**Evidence:** `docs/providers/vmware.md`, `internal/config/types_kubernetes.go` NodeConfig

#### Bare Metal

Bare metal follows the same model as VMware: pre-provisioned hosts with static IP assignments.

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

* Hardware lifecycle managed outside openCenter
* Longhorn recommended for persistent storage (no cloud CSI available)
* MetalLB required for LoadBalancer services
* VRRP for API server HA
* Calico BGP mode available for direct routing (no encapsulation overhead)
* Node IPs defined explicitly in configuration

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
│  │  │ Registry │  (optional, port 5000)               │  │
│  │  └──────────┘                                      │  │
│  └────────────────────────────────────────────────────┘  │
│                                                          │
│  API: localhost:<api_server_port>                         │
│  Ingress: extra_port_mappings (80, 443)                  │
└──────────────────────────────────────────────────────────┘
```

Key characteristics:

* Single-host, containerized nodes (not VMs)
* No bastion, no VRRP, no external load balancer
* Optional local container registry
* Extra port mappings for ingress testing
* Default CNI or Calico (configurable via `disable_default_cni`)
* Disposable: create and destroy in minutes

**Evidence:** `internal/config/types_infrastructure.go` KindConfig

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
│ Gateway API │  (Envoy, port 80/443)
│ + MetalLB   │  or Octavia LB
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Service     │  ClusterIP (10.43.x.x)
│ Network     │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│ Pod Network │  Pod IPs (10.42.x.x)
│ (Calico)    │  VXLAN/IPIP/BGP encapsulation
└─────────────┘
```

### Control Plane Access

The Kubernetes API server is exposed through one of these mechanisms (provider-dependent):

| Provider | API HA Mechanism | Configuration |
| --- | --- | --- |
| OpenStack (with Octavia) | Octavia load balancer | `use_octavia: true` |
| OpenStack (without Octavia) | VRRP virtual IP | `vrrp_enabled: true`, `vrrp_ip: <IP>` |
| VMware | VRRP or kube-vip | `vrrp_enabled: true` or `kube_vip_enabled: true` |
| Bare metal | VRRP | `vrrp_enabled: true`, `vrrp_ip: <IP>` |
| Kind | localhost port mapping | `api_server_port: <port>` |

**Evidence:** `internal/config/types_cluster.go` ClusterNetworkingConfig

## Plan the IP Addresses

Careful IP planning prevents conflicts and simplifies troubleshooting. Use this worksheet as a starting point.

### Node Network Allocation

For a `/22` node network (`10.2.128.0/22`, 1,024 addresses):

| Range | Purpose | Example |
| --- | --- | --- |
| `.1` | Network gateway | `10.2.128.1` |
| `.2–.9` | Infrastructure (DNS, NTP, bastion) | `10.2.128.2` (bastion) |
| `.10` | Kubernetes API VIP (VRRP) | `10.2.128.10` |
| `.20–.22` | Control plane nodes | `10.2.128.20–22` |
| `.23–.25` | Worker nodes (initial pool) | `10.2.128.23–25` |
| `.26–.99` | Additional worker pools / future growth | Reserved |
| `.100–.200` | MetalLB address pool (LoadBalancer services) | `10.2.128.100–200` |
| `.201–.254` | Reserved | Future use |

Configure the allocation pool to match:

```yaml
opencenter:
  cluster:
    networking:
      subnet_nodes: "10.2.128.0/22"
      allocation_pool_start: "10.2.128.10"
      allocation_pool_end: "10.2.131.250"
      vrrp_ip: "10.2.128.10"
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
  cluster:
    networking:
      dns_nameservers:
        - "10.0.0.53"        # Internal DNS preferred
        - "8.8.8.8"          # Fallback
      ntp_servers:
        - "time.example.com" # Internal NTP preferred
        - "0.pool.ntp.org"   # Fallback
```

Use internal DNS and NTP servers when available. Public servers are acceptable for development but introduce an external dependency in production.

**Evidence:** `docs/operations/configure-networking.md`, `internal/config/types_cluster.go`

## Add-ons and Preview Features

### GA Platform Services

These services are production-supported and enabled by default:

| Service | Category | Purpose |
| --- | --- | --- |
| cert-manager | Security | Automated TLS certificate lifecycle |
| Keycloak | Identity | OIDC provider, user federation, MFA |
| Kyverno | Policy | 17 ClusterPolicies for baseline security |
| RBAC Manager | Access | Declarative RBAC from Keycloak groups |
| kube-prometheus-stack | Observability | Prometheus, Grafana, Alertmanager |
| Loki | Observability | Log aggregation with LogQL |
| Tempo | Observability | Distributed tracing |
| Gateway API + Envoy | Networking | Modern ingress with HTTPRoute |
| Calico | Networking | CNI with network policy support |
| MetalLB | Networking | Bare metal load balancer |
| Velero | Backup | Application backup and restore |
| etcd-backup | Backup | Cluster state snapshots to S3 |
| FluxCD | GitOps | Continuous reconciliation engine |
| Headlamp | Management | Kubernetes dashboard with OIDC |
| OLM | Management | Operator lifecycle management |
| PostgreSQL Operator | Management | Database for Keycloak and other services |

### Preview / Optional Services

| Service | Status | Notes |
| --- | --- | --- |
| Cilium CNI | Preview | eBPF-based networking, kube-proxy replacement |
| Kube-OVN | Preview | Software-defined networking with Cilium integration |
| Istio | Optional | Service mesh with mTLS (for zero-trust requirements) |
| Weave GitOps | Optional | Web UI for FluxCD |
| Windows workers | Informational | Not a supported deployment target at GA |
| Talos Linux | Preview | Immutable OS with disk encryption, WireGuard, vTPM |

**Evidence:** `docs/reference/platform-services.md`, `docs/concepts/provider-comparison.md`

## Container Image Reference

All platform service images are pulled from public registries by default. For air-gapped deployments, `openCenter-AirGap` mirrors all images to a local bastion registry.

### Registry Sources

| Registry | Services |
| --- | --- |
| `registry.k8s.io` | Kubernetes components, vSphere CSI |
| `ghcr.io` | FluxCD, openCenter services, alert-proxy |
| `docker.io` | Calico, Longhorn, various Helm charts |
| `quay.io` | cert-manager, Prometheus, OLM |

### Image Pinning

Service versions are pinned in `openCenter-gitops-base` via Git tags. Clusters reference a specific tag:

```yaml
# GitRepository source in customer overlay
spec:
  ref:
    tag: v1.0.0  # Pinned version
```

Individual service images can be overridden per-cluster:

```yaml
opencenter:
  services:
    vsphere-csi:
      image_repository: "registry.k8s.io/csi-vsphere"
      image_tag: "v3.3.0"
```

### Air-Gap Image Mirroring

For disconnected environments, the bastion host serves as a local registry on port 5000. All images are rewritten to pull from `bastion:5000/` instead of public registries.

**Evidence:** `internal/config/types_services.go` ServiceCfg, Ecosystem.md air-gap section

## Configure Compute for the Base Cluster

### Node Sizing

openCenter uses "flavors" (instance types) to define compute resources. The minimum recommended sizes:

| Role | vCPUs | RAM | Disk | Config Field |
| --- | --- | --- | --- | --- |
| Control plane | 4 | 8 GB | 40 GB | `flavor_master` |
| Worker | 4 | 16 GB | 100 GB | `flavor_worker` |
| Bastion | 2 | 4 GB | 20 GB | `flavor_bastion` |

Production clusters should use larger flavors for workers depending on workload density.

### Node Counts

| Role | Minimum | Recommended | Maximum |
| --- | --- | --- | --- |
| Control plane | 1 (dev) | 3 (HA) | 100 |
| Workers | 1 | 2--3 | 1,000 |
| Windows workers | 0 | 0 | 100 |

### Additional Worker Pools

For heterogeneous workloads, define additional worker pools with different flavors, images, or affinity rules:

```yaml
opencenter:
  cluster:
    kubernetes:
      additional_server_pools_worker:
        - name: gpu-workers
          worker_count: 2
          flavor_worker: "gpu.large"
          node_worker: "gpu"
          server_group_affinity: "soft-anti-affinity"
          worker_node_bfv_volume_size: 200
```

Each pool gets its own naming convention, flavor, and optional subnet placement.

### Server Group Affinity

For HA, use anti-affinity to spread nodes across physical hosts:

```yaml
opencenter:
  infrastructure:
    server_group_affinity:
      - "anti-affinity"  # Hard: fail if same host
      # or "soft-anti-affinity"  # Best-effort spread
```

**Evidence:** `internal/config/types_kubernetes.go` KubernetesConfig, AdditionalServerPool

## Integrate OIDC for the Cluster

openCenter integrates Keycloak as the OIDC identity provider for the Kubernetes API server. This enables group-based RBAC without managing individual kubeconfig files.

### How It Works

```
User → Keycloak (authenticate) → ID Token (JWT)
  → kubectl (--token) → API Server (validate JWT)
    → RBAC (map groups to roles)
```

### Kubernetes API Server OIDC Configuration

```yaml
opencenter:
  cluster:
    kubernetes:
      oidc:
        enabled: true
        kube_oidc_url: "https://auth.<org>.<cluster>.<region>.k8s.opencenter.cloud/realms/opencenter"
        kube_oidc_client_id: "kubernetes"
        kube_oidc_username_claim: "preferred_username"
        kube_oidc_username_prefix: "oidc:"
        kube_oidc_groups_claim: "groups"
        kube_oidc_groups_prefix: "oidc:"
```

This configures the API server flags:

* `--oidc-issuer-url` → Keycloak realm URL
* `--oidc-client-id` → Client registered in Keycloak
* `--oidc-username-claim` → JWT claim for username
* `--oidc-groups-claim` → JWT claim for group membership

### Default RBAC Policies

RBAC Manager converts Keycloak groups to Kubernetes RoleBindings:

| Keycloak Group | Kubernetes Role | Scope |
| --- | --- | --- |
| `cluster-admins` | `cluster-admin` | Cluster-wide |
| `viewers` | `view` | Cluster-wide |

Custom groups can be added via RBACDefinition CRDs in the GitOps repository.

**Evidence:** `internal/config/types_kubernetes.go` OIDCConfig, `docs/reference/platform-services.md` keycloak/rbac-manager

## Integrate OIDC for the Workload

Platform services that expose web UIs also authenticate through Keycloak OIDC. This provides single sign-on across the platform.

### Services with OIDC Integration

| Service | OIDC Config Location | Purpose |
| --- | --- | --- |
| Headlamp | `services.headlamp.oidc_*` | Kubernetes dashboard SSO |
| Grafana | kube-prometheus-stack values | Monitoring dashboard SSO |
| Weave GitOps | `services.weave-gitops` | GitOps UI SSO |

### Global OIDC Configuration

A global OIDC block configures shared settings for all services:

```yaml
opencenter:
  oidc:
    enabled: true
    client_id: "opencenter"
    secret_name: "gateway-oidc-secret"
    scopes:
      - openid
      - profile
      - email
      - groups
    logout_path: "/logout"
```

Individual services reference this global configuration. The Gateway API can enforce OIDC authentication at the ingress layer, so services behind the gateway inherit authentication without implementing it themselves.

**Evidence:** `internal/config/types_opencenter.go` GlobalOIDCConfig, GatewayGlobalConfig

## Select a Networking Model

### CNI Selection

openCenter supports three CNI plugins. Only one can be active per cluster.

| CNI | Encapsulation | kube-proxy | Best For |
| --- | --- | --- | --- |
| Calico (default) | VXLAN, IPIP, or None (BGP) | Standard | Most deployments. Mature, well-understood, strong network policy support. |
| Cilium (preview) | VXLAN or native | Replaceable (eBPF) | Teams wanting eBPF observability and kube-proxy replacement. |
| Kube-OVN (preview) | Geneve | Standard | Software-defined networking with subnet isolation. Optional Cilium integration. |

### Calico Encapsulation Trade-offs

| Mode | Overhead | Requirements | Use When |
| --- | --- | --- | --- |
| VXLAN | ~50 bytes/packet | None | Default. Works everywhere. |
| IPIP | ~20 bytes/packet | IP-in-IP protocol allowed | Lower overhead than VXLAN, but some clouds block it. |
| None (BGP) | Zero | BGP-capable network fabric | Bare metal with ToR switches that peer BGP. Best performance. |

### Load Balancer Selection

| Provider | Type | Use When |
| --- | --- | --- |
| MetalLB | L2 ARP or BGP | Bare metal, VMware, any environment without cloud LB |
| Octavia | Cloud LB | OpenStack with Octavia service available |
| OVN | Cloud LB | OpenStack with OVN networking |
| cloud-native | Provider LB | AWS (not GA for cluster provisioning, but usable for services) |

**Evidence:** `internal/config/types_kubernetes.go` NetworkPlugin, `docs/operations/configure-networking.md`

## Deploy Ingress Resources

### Gateway API (Recommended)

openCenter uses Gateway API as the standard ingress model. It replaces the older Ingress resource with a more expressive, role-oriented API.

```
Internet → LoadBalancer (MetalLB/Octavia)
              → Gateway (Envoy, namespace: rackspace-system)
                  → HTTPRoute (per-service routing)
                      → Service → Pods
```

### Hostname Convention

Services follow a predictable hostname pattern:

```
<service>.<org>.<cluster>.<region>.<base_domain>
```

Example: `auth.my-org.production.sjc3.k8s.opencenter.cloud`

Configure the base domain and cluster FQDN:

```yaml
opencenter:
  cluster:
    base_domain: "k8s.opencenter.cloud"
    cluster_fqdn: "production.sjc3.k8s.opencenter.cloud"
```

### TLS Certificates

cert-manager automatically provisions TLS certificates for HTTPRoute hostnames using Let’s Encrypt (or a custom ACME server):

```yaml
opencenter:
  services:
    cert-manager:
      enabled: true
      letsencrypt_server: "https://acme-v02.api.letsencrypt.org/directory"
```

For internal CAs, provide a custom CA certificate:

```yaml
opencenter:
  cluster:
    networking:
      security:
        ca_certificates: |
          -----BEGIN CERTIFICATE-----
          ...
          -----END CERTIFICATE-----
```

**Evidence:** `docs/reference/platform-services.md` gateway-api/gateway/cert-manager

## Secure the Network Flow

### Layer 1: Pod Security Admission (Cluster-Level)

Kubespray configures the API server with Pod Security Admission:

| Level | Mode | Effect |
| --- | --- | --- |
| Baseline | Enforce | Blocks known privilege escalations (no privileged containers, no host namespaces) |
| Restricted | Audit | Logs violations of restricted policy |
| Restricted | Warn | Warns users about restricted violations |

Specific namespaces can be exempted:

```yaml
opencenter:
  cluster:
    kubernetes:
      security:
        k8s_hardening: true
        pod_security_exemptions:
          - "kube-system"
          - "flux-system"
```

### Layer 2: Kyverno Policies (Resource-Level)

17 ClusterPolicies enforce baseline security across all namespaces:

* `disallow-privileged-containers`
* `disallow-host-namespaces`
* `disallow-host-path`
* `require-run-as-nonroot`
* `restrict-seccomp`
* `restrict-volume-types`
* `disallow-capabilities` (and 10 more)

Kyverno operates independently of Pod Security Admission, providing a second enforcement layer.

### Layer 3: NetworkPolicies

Platform services (FluxCD, OLM) ship with NetworkPolicies that restrict traffic to known peers. Application teams should add their own NetworkPolicies following the patterns in `openCenter-customer-app-example`.

### Layer 4: OS Hardening

When `os_hardening: true`, Kubespray applies kernel-level security:

* Firewall rules
* Sysctl hardening (IP forwarding, source routing)
* SSH hardening

### Layer 5: Optional Service Mesh

For zero-trust or multi-tenant environments, Istio provides mTLS between all pods. This is not enabled by default because it adds operational complexity.

**Evidence:** `internal/config/types_security.go`, `docs/concepts/security-model.md`

## Add Secret Management

### Encryption Model

openCenter uses a dual-encryption strategy:

```
Developer writes secret → SOPS encrypts (Age key) → Git commit (ciphertext)
  → FluxCD pulls → SOPS decrypts (Age key in cluster) → Kubernetes Secret
    → etcd stores (encrypted at rest)
```

### SOPS Age Key Lifecycle

| Key Type | Rotation Period | Storage |
| --- | --- | --- |
| Age encryption key | 90 days | `secrets/age/<cluster>_keys.txt` |
| SSH deploy key | 180 days | `secrets/ssh/` |

Rotation uses a dual-key strategy: the new key encrypts new secrets while the old key remains valid for decryption, ensuring zero-downtime rotation.

### Configuration

```yaml
secrets:
  sops:
    age_keys:
      - age1ql3z7hjy54pw3hyww5ayyfg7zqgvc7w3j2elw8zmrj2kg5sfn9aqmcac8p
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

The storage driver depends on the infrastructure provider:

| Provider | CSI Driver | Default Storage Class | Dynamic Provisioning |
| --- | --- | --- | --- |
| OpenStack | Cinder CSI | `csi-cinder-sc-delete` | Yes |
| VMware | vSphere CSI | `vsphere-sc` | Yes |
| Bare metal | Longhorn | `longhorn` | Yes |
| Kind | Local path | `standard` | Yes (host path) |

### Longhorn (Distributed Storage)

Longhorn provides replicated block storage for environments without a cloud storage backend. It runs on worker nodes and replicates data across them.

Enable Longhorn:

```yaml
opencenter:
  cluster:
    kubernetes:
      storage_plugin:
        longhorn:
          enabled: true
```

Longhorn is recommended for bare metal and can supplement cloud CSI drivers for workloads that need cross-node replication.

### Volume Snapshots

The `external-snapshotter` service provides VolumeSnapshot CRDs. It works with any CSI driver that supports snapshots (Cinder, vSphere, Longhorn).

### Boot Volume Configuration

Node boot volumes are configurable per provider:

```yaml
opencenter:
  storage:
    default_storage_class: "csi-cinder-sc-delete"
    worker_volume_size: 100          # GB
    worker_volume_destination_type: "volume"
    worker_volume_source_type: "image"
    worker_volume_type: "HA-Standard"
```

**Evidence:** `internal/config/types_storage.go`, `internal/config/types_kubernetes.go` StoragePlugin

## Policy Management

### Policy Layers

openCenter enforces policy at two independent layers:

| Layer | Engine | Scope | Action |
| --- | --- | --- | --- |
| Cluster | Pod Security Admission | Namespace-level | Enforce/Audit/Warn |
| Resource | Kyverno | Resource-level | Validate/Mutate/Generate |

### Kyverno Default Ruleset

The 17 default ClusterPolicies cover the Kubernetes Pod Security Standards baseline:

* Disallow privileged containers, host namespaces, host paths, host ports
* Require non-root execution, read-only root filesystem
* Restrict seccomp profiles, volume types, capabilities, sysctls
* Disallow privilege escalation, default service accounts

These policies are deployed via FluxCD from `openCenter-gitops-base` and apply to all namespaces except those explicitly exempted.

### Custom Policies

Add custom Kyverno policies in the cluster overlay:

```
applications/overlays/<cluster>/services/kyverno/custom-policies/
├── require-labels.yaml
└── restrict-registries.yaml
```

### Policy Exemptions

Namespaces that need elevated privileges (e.g., `kube-system`, `flux-system`) are exempted at the Pod Security Admission level via `pod_security_exemptions` in the cluster configuration.

**Evidence:** `docs/concepts/security-model.md`, `docs/reference/platform-services.md` kyverno

## Node and Pod Scalability

### Horizontal Scaling

Add workers by updating the configuration and re-running setup:

```yaml
opencenter:
  cluster:
    kubernetes:
      worker_count: 5  # Increase from 3
```

For OpenStack, new VMs are provisioned automatically. For VMware and bare metal, pre-provision the hosts and add their IPs to the `worker_nodes` array.

### Additional Worker Pools

Separate pools allow different instance types for different workloads:

```yaml
opencenter:
  cluster:
    kubernetes:
      additional_server_pools_worker:
        - name: memory-optimized
          worker_count: 3
          flavor_worker: "m1.xlarge"
          node_worker: "mem"
```

### Validated Limits

| Dimension | Validated Maximum |
| --- | --- |
| Control plane nodes | 100 |
| Worker nodes (per pool) | 1,000 |
| Windows workers (per pool) | 100 |
| Additional worker pools | No hard limit (validated per pool) |

### Pod Density

Pod density depends on the CNI and node size. Calico and Cilium both support the Kubernetes default of 110 pods per node. Adjust via Kubespray if needed.

**Evidence:** `internal/config/types_kubernetes.go` KubernetesConfig validation tags

## Business Continuity Decisions

### Backup Strategy

openCenter provides two complementary backup mechanisms:

| Mechanism | What It Backs Up | Schedule | Retention |
| --- | --- | --- | --- |
| etcd snapshots | Cluster state (all API objects) | Every 6h (prod) | 30 days |
| Velero | Application resources + persistent volumes | Daily (prod) | 90 days |

### Recovery Scenarios

| Scenario | Recovery Method | RTO |
| --- | --- | --- |
| Single pod failure | Kubernetes self-healing | Seconds |
| Node failure | Kubernetes reschedules pods | Minutes |
| etcd corruption | Restore from etcd snapshot | 30--60 minutes |
| Application deletion | Velero restore | 15--30 minutes |
| Full cluster loss | Rebuild from GitOps repo + restore data | 1--2 hours |
| Cross-cluster migration | Velero backup/restore to new cluster | 1--2 hours |

### GitOps as Disaster Recovery

Because all cluster configuration lives in Git, a full cluster rebuild is deterministic:

1. Provision new infrastructure (OpenTofu)
2. Deploy Kubernetes (Kubespray)
3. Bootstrap FluxCD (points to same Git repo)
4. FluxCD reconciles all services and applications
5. Restore persistent data from Velero backups

The Git repository is the recovery artifact. Protect it accordingly.

### Multi-Region Considerations

For multi-region deployments, each region gets its own cluster with its own GitOps overlay. Shared configuration lives in `openCenter-gitops-base`. Region-specific overrides live in the cluster overlay.

**Evidence:** `docs/operations/backup-and-restore.md`, `docs/reference/platform-services.md` velero/etcd-backup

## Monitor and Collect Logs and Metrics

### Observability Stack

openCenter deploys a complete observability pipeline:

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ Prometheus   │    │ Loki         │    │ Tempo        │
│ (Metrics)    │    │ (Logs)       │    │ (Traces)     │
│              │    │              │    │              │
│ Scrapes pods │    │ Receives     │    │ Receives     │
│ every 30s    │    │ from agents  │    │ OTLP spans   │
└──────┬───────┘    └──────┬───────┘    └──────┬───────┘
       │                   │                   │
       └───────────┬───────┘───────────────────┘
                   │
            ┌──────▼───────┐
            │   Grafana    │
            │ (Dashboards) │
            │ OIDC SSO     │
            └──────────────┘
```

### Metrics (kube-prometheus-stack)

Components: Prometheus, Grafana, Alertmanager, node-exporter, kube-state-metrics.

Configure storage:

```yaml
opencenter:
  services:
    kube-prometheus-stack:
      enabled: true
      prometheus_volume_size: 50       # GB
      prometheus_storage_class: "csi-cinder-sc-delete"
      grafana_volume_size: 10
      alertmanager_volume_size: 10
```

Alertmanager supports webhook integration for external alerting systems:

```yaml
opencenter:
  services:
    kube-prometheus-stack:
      webhook_url: "https://alerts.example.com/webhook"
```

### Logs (Loki)

Loki supports two storage backends:

| Backend | Use When | Configuration |
| --- | --- | --- |
| Swift (OpenStack) | OpenStack deployments | `loki_storage_type: "swift"` |
| S3 | AWS or S3-compatible storage | `loki_storage_type: "s3"` |

### Traces (Tempo)

Tempo receives OpenTelemetry (OTLP) spans and stores them in S3-compatible storage. Grafana queries Tempo for trace visualization.

### OpenTelemetry

The OpenTelemetry Collector can be enabled to receive, process, and export telemetry data (metrics, logs, traces) in a vendor-neutral format.

**Evidence:** `docs/reference/platform-services.md` kube-prometheus-stack/loki/tempo

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

openCenter can detect configuration drift between the desired state (Git) and actual state (cluster/infrastructure):

```bash
opencenter cluster drift my-cluster
```

| Provider | Drift Detection | Auto-Reconcile |
| --- | --- | --- |
| OpenStack | Yes | Limited |
| VMware | Yes | No |
| Bare metal | No | No |
| Kind | No | No |

FluxCD handles application-level drift automatically by continuously reconciling Git state to the cluster.

### Day-2 Operations

| Operation | Method |
| --- | --- |
| Upgrade Kubernetes | Update `version` in config, re-run setup, Kubespray handles rolling upgrade |
| Add workers | Update `worker_count`, re-run setup |
| Enable/disable services | Toggle `enabled` flag, commit, FluxCD reconciles |
| Rotate secrets | `opencenter secrets keys rotate --cluster <cluster> --type age` |
| Backup | Automated via etcd-backup and Velero schedules |
| Restore | `velero restore create --from-backup <name>` |

**Evidence:** `docs/reference/cli-commands.md`, `docs/concepts/drift-detection.md`

## Cost Management

openCenter does not include built-in cost management tooling. Cost optimization is an infrastructure-level concern that depends on the provider.

### Recommendations by Provider

| Provider | Cost Lever | Approach |
| --- | --- | --- |
| OpenStack | Instance flavors | Right-size flavors for workload. Use smaller flavors for dev/staging. |
| OpenStack | Storage | Use `HA-Standard` volumes for non-critical workloads, `HA-Performance` for databases. |
| VMware | VM sizing | Align VM resources with actual utilization. Monitor via Prometheus. |
| Bare metal | Hardware utilization | Maximize pod density per node. Use additional worker pools for burst capacity. |
| All | Observability storage | Set appropriate retention periods for Prometheus (15d default), Loki, and Tempo. |
| All | Backup retention | Match retention to compliance requirements, not "just in case." |

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

* `openCenter-gitops-base` -- Base manifests for all platform services
* `openCenter-customer-app-example` -- Reference patterns for application deployment
* `openCenter-AirGap` -- Packaging for disconnected environments
* `opencenter-windows` -- Windows worker node support (Ansible collection)
* [Kubernetes Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) -- Upstream PSS documentation
* [FluxCD Documentation](https://fluxcd.io/docs/) -- GitOps toolkit reference
* [Kyverno Policies](https://kyverno.io/policies/) -- Policy library and examples

---

## Evidence

This document is based on:

* Configuration types: `internal/config/types*.go`
* Provider implementations: `internal/cloud/factory.go`
* GitOps generation: `internal/gitops/generator.go`
* SOPS management: `internal/sops/manager.go`
* Existing documentation: `docs/concepts/architecture.md`, `docs/concepts/security-model.md`, `docs/concepts/gitops-workflow.md`, `docs/concepts/provider-comparison.md`
* Platform services: `docs/reference/platform-services.md`
* Networking guide: `docs/operations/configure-networking.md`
* Backup guide: `docs/operations/backup-and-restore.md`
* Ecosystem architecture: `.kiro/steering/ecosystem.md`