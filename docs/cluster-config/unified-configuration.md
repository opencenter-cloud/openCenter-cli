# OpenCenter Unified Configuration Reference

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
  - [Control Plane Layers](#control-plane-layers)
  - [Configuration Hierarchy](#configuration-hierarchy)
- [Configuration Domains](#configuration-domains)
  - [Meta Domain](#meta-domain)
  - [Cluster Domain](#cluster-domain)
  - [Infrastructure Domain](#infrastructure-domain)
  - [Deployment Domain](#deployment-domain)
  - [Services Domain](#services-domain)
  - [Secrets Domain](#secrets-domain)
- [Reference Resolution](#reference-resolution)
  - [Reference Syntax](#reference-syntax)
  - [Resolution Phases](#resolution-phases)
  - [Dependency Graph](#dependency-graph)
- [Default Resolution Framework](#default-resolution-framework)
  - [Precedence Order](#precedence-order)
  - [Provider-Region Registry](#provider-region-registry)
- [Provider Architecture](#provider-architecture)
  - [Provider Interface](#provider-interface)
  - [Provider-Specific Extensions](#provider-specific-extensions)
- [Service Provider Polymorphism](#service-provider-polymorphism)
  - [Example: cert-manager DNS Providers](#example-cert-manager-dns-providers)
  - [Infrastructure-Aware Defaults](#infrastructure-aware-defaults)
- [Complete Configuration Examples](#complete-configuration-examples)
  - [Kubespray Deployment Example](#kubespray-deployment-example)
  - [Kamaji Hosted Control Plane Example](#kamaji-hosted-control-plane-example)
- [Migration Guide](#migration-guide)
  - [From v1.x to v2.0](#from-v1x-to-v20)
  - [Migrating to Kamaji Hosted Control Plane](#migrating-to-kamaji-hosted-control-plane)
- [Appendix: Configuration Validation Rules](#appendix-configuration-validation-rules)
  - [Required Fields by Provider](#required-fields-by-provider)
  - [Required Fields by Deployment Method](#required-fields-by-deployment-method)
  - [Kamaji-Specific Validation Rules](#kamaji-specific-validation-rules)
  - [Service Dependencies](#service-dependencies)
  - [Deployment Method Compatibility Matrix](#deployment-method-compatibility-matrix)
  - [Validation Error Codes](#validation-error-codes)

---

## Overview

OpenCenter uses a unified, hierarchical configuration model that transforms a single declarative YAML file into a production-ready Kubernetes cluster. The configuration system addresses:

- Single source of truth for all cluster settings
- Provider-agnostic core with provider-specific extensions
- Deployment method abstraction (Kubespray, Talos, managed K8s)
- Polymorphic service configuration
- Secure secrets management via SOPS/Age

**Schema Version**: `2.0`

---

## Architecture

### Control Plane Layers

```
┌─────────────────────────────────────────────────────────────┐
│                 Governance & Policy Layer                    │
│              (Validation, Compliance, Security)              │
├─────────────────────────────────────────────────────────────┤
│              Lifecycle & Reconciliation Engine               │
│         (Provisioning, Upgrades, Drift Detection)            │
├─────────────────────────────────────────────────────────────┤
│          Configuration Resolution & Validation               │
│        (Defaults, References, Schema Validation)             │
├─────────────────────────────────────────────────────────────┤
│              Provider & Deployment Adapters                  │
│     (OpenStack, AWS, GCP, Kubespray, Talos, ClusterAPI)      │
├─────────────────────────────────────────────────────────────┤
│                 Infrastructure Providers                     │
│           (Cloud APIs, Bare Metal, VMware)                   │
└─────────────────────────────────────────────────────────────┘
```

### Configuration Hierarchy

```
┌──────────────────────────────────────────────────────────────┐
│  Config (Root)                                               │
│  ├── schema_version: "2.0"                                   │
│  ├── metadata                                                │
│  │                                                           │
│  ├── opencenter                                              │
│  │   ├── meta ─────────────────► Identity & Ownership        │
│  │   ├── cluster ──────────────► Kubernetes Semantics        │
│  │   ├── infrastructure ───────► Networking, Compute         │
│  │   ├── deployment ───────────► Installation Method         │
│  │   ├── services ─────────────► Platform Workloads          │
│  │   └── managed_services ─────► External Integrations       │
│  │                                                           │
│  ├── opentofu ─────────────────► IaC Backend                 │
│  ├── secrets ──────────────────► Credentials & Keys          │
│  └── deployment ───────────────► Auto-deploy Settings        │
└──────────────────────────────────────────────────────────────┘
```

**Domain Ownership Diagram**:

```
                    ┌─────────────┐
                    │    Meta     │
                    │  (Identity) │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │   Cluster   │
                    │ (K8s Config)│
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              │            │            │
       ┌──────▼──────┐     │     ┌──────▼──────┐
       │Infrastructure│     │     │  Deployment │
       │  (Provider)  │     │     │  (Method)   │
       └──────┬──────┘     │     └──────┬──────┘
              │            │            │
              │     ┌──────▼──────┐     │
              │     │  Services   │     │
              │     │ (Workloads) │     │
              │     └─────────────┘     │
              │                         │
              └─────────┬───────────────┘
                        │
                 ┌──────▼──────┐
                 │   Secrets   │
                 │(Credentials)│
                 └─────────────┘
```

---

## Configuration Domains

### Meta Domain

Cluster identity and organizational metadata.

```yaml
opencenter:
  meta:
    name: "prod-cluster"           # Cluster identifier
    organization: "acme-corp"      # Organization namespace
    env: "production"              # Environment (dev/staging/production)
    region: "us-east-1"            # Deployment region
    status: ""                     # Lifecycle status (managed by system)
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique cluster identifier within organization |
| `organization` | string | No | Organization namespace (default: "opencenter") |
| `env` | string | No | Environment classification |
| `region` | string | Yes | Cloud provider region |
| `status` | string | No | System-managed lifecycle status |

---

### Cluster Domain

Kubernetes-specific configuration independent of infrastructure provider.

```yaml
opencenter:
  cluster:
    cluster_name: "prod-cluster"
    base_domain: "k8s.acme.com"
    cluster_fqdn: "prod-cluster.us-east-1.k8s.acme.com"
    admin_email: "admin@acme.com"
    
    kubernetes:
      version: "1.33.5"
      api_port: 443
      kube_vip_enabled: true
      
      # Kubernetes networking (CNI-managed only)
      subnet_pods: "10.42.0.0/16"
      subnet_services: "10.43.0.0/16"
      
      # CNI Plugin Selection
      network_plugin:
        calico:
          enabled: true
          encapsulation_type: "VXLAN"
          nat_outgoing: true
        cilium:
          enabled: false
        kube-ovn:
          enabled: false
      
      # Security
      security:
        k8s_hardening: true
        pod_security_exemptions:
          - "kube-system"
          - "tigera-operator"
      
      # OIDC (optional)
      oidc:
        enabled: false
        kube_oidc_url: ""
        kube_oidc_client_id: "kubernetes"
      
      # Additional worker pools (can override infrastructure defaults)
      additional_server_pools_worker:
        - name: "high-memory"
          worker_count: 2
          flavor_worker: "gp.0.8.64"
          worker_node_bfv_volume_size: 200
          additional_block_devices_worker:
            - device_name: "/dev/vdb"
              volume_size: 1000
```

**Networking Ownership**:

```
┌─────────────────────────────────────────────────────────────┐
│  cluster.networking (Infrastructure Layer)                   │
│  ├── subnet_nodes ────────► Node network CIDR                │
│  ├── vrrp_ip ─────────────► Kubernetes API VIP               │
│  ├── loadbalancer_provider► Cluster-wide LB decision         │
│  ├── dns_nameservers ─────► Infrastructure DNS               │
│  └── ntp_servers ─────────► Time synchronization             │
├─────────────────────────────────────────────────────────────┤
│  cluster.kubernetes.networking (Kubernetes Layer)            │
│  ├── subnet_pods ─────────► Pod network CIDR (CNI)           │
│  ├── subnet_services ─────► Service network CIDR             │
│  └── network_plugin ──────► CNI selection                    │
└─────────────────────────────────────────────────────────────┘
```

---

### Infrastructure Domain

Provider-agnostic core with provider-specific extensions.

```yaml
opencenter:
  infrastructure:
    provider: "openstack"    # openstack | aws | gcp | azure | baremetal | vsphere
    ssh_user: "ubuntu"
    ssh_key_path: ""
    os_version: "24"
    server_group_affinity: ["anti-affinity"]
    k8s_api_ip: ""           # References cluster.networking.vrrp_ip
    
    node_naming:
      worker: "wn"
      master: "cp"
      worker_windows: "win"
    
    bastion:
      address: "localhost"
    
    # Compute configuration (instance types and node counts)
    compute:
      # Instance flavors (provider-specific instance types)
      flavor_bastion: "gp.0.2.2"
      flavor_master: "gp.0.4.8"
      flavor_worker: "gp.0.4.16"
      flavor_worker_windows: "gp.5.4.16"
      
      # Node counts
      master_count: 3
      worker_count: 5
      worker_count_windows: 0
    
    # Storage configuration (boot volumes, additional devices)
    storage:
      default_storage_class: "csi-cinder-sc-delete"
      
      # Default worker boot volume configuration
      worker_volume_size: 100
      worker_volume_destination_type: "volume"
      worker_volume_source_type: "image"
      worker_volume_type: "HA-Standard"
      worker_volume_delete_on_termination: true
      
      # Master boot volume configuration
      master_volume_size: 100
      master_volume_destination_type: "volume"
      master_volume_source_type: "image"
      master_volume_type: "HA-Standard"
      
      # Additional block devices (data volumes)
      additional_block_devices: []
    
    cloud:
      # Provider-specific configuration
      openstack:
        auth_url: "https://identity.example.com/v3"
        region: "RegionOne"
        tenant_name: "production"
        availability_zone: "az1"
        image_id: "799dcf97-3656-4361-8187-13ab1b295e33"
        
        networking:
          floating_ip_pool: "PUBLICNET"
          router_external_network_id: "723f8fa2-..."
          k8s_api_port_acl: ["0.0.0.0/0"]
          
          designate:
            dns_zone_name: ""
          
          vlan:
            id: ""
            mtu: 1500
            provider: "physnet1"
      
      aws:
        profile: ""
        region: "us-east-1"
        vpc_id: ""
        private_subnets: []
        public_subnets: []
```

**Infrastructure Storage Ownership**:

```
┌─────────────────────────────────────────────────────────────┐
│  infrastructure.compute (Compute Layer)                      │
│  ├── flavor_* ────────────► Instance types/sizes             │
│  ├── master_count ────────► Control plane node count         │
│  ├── worker_count ────────► Worker node count                │
│  └── worker_count_windows ► Windows worker count             │
├─────────────────────────────────────────────────────────────┤
│  infrastructure.storage (Storage Layer)                      │
│  ├── default_storage_class ──► Kubernetes default SC         │
│  ├── worker_volume_* ────────► Worker boot volume config     │
│  ├── master_volume_* ────────► Master boot volume config     │
│  └── additional_block_devices ► Data volumes                 │
├─────────────────────────────────────────────────────────────┤
│  cluster.kubernetes (Kubernetes Layer)                       │
│  ├── version ─────────────────► Kubernetes version           │
│  ├── additional_server_pools_worker ──► Custom worker pools  │
│  │   ├── flavor_worker ─────────► Pool-specific flavor       │
│  │   ├── worker_node_bfv_* ─────► Pool-specific volumes      │
│  │   └── additional_block_devices_* ─► Pool-specific data    │
│  └── additional_server_pools_worker_windows ► Windows pools  │
└─────────────────────────────────────────────────────────────┘
```

**Provider Interface Contract**:

```
┌─────────────────────────────────────────────────────────────┐
│  Provider Interface (All providers must implement)           │
├─────────────────────────────────────────────────────────────┤
│  Authentication                                              │
│  ├── Validate credentials                                    │
│  └── Establish connection                                    │
├─────────────────────────────────────────────────────────────┤
│  Networking                                                  │
│  ├── Create/manage networks                                  │
│  ├── Configure load balancers                                │
│  └── DNS integration                                         │
├─────────────────────────────────────────────────────────────┤
│  Compute                                                     │
│  ├── Provision instances                                     │
│  ├── Manage server groups                                    │
│  └── Configure storage volumes                               │
├─────────────────────────────────────────────────────────────┤
│  Storage                                                     │
│  ├── Create boot volumes                                     │
│  ├── Attach additional volumes                               │
│  └── Manage storage classes                                  │
├─────────────────────────────────────────────────────────────┤
│  Validation                                                  │
│  ├── Preflight checks                                        │
│  └── Resource availability                                   │
└─────────────────────────────────────────────────────────────┘
```

---

### Deployment Domain

Deployment method configuration (how Kubernetes is installed).

```yaml
opencenter:
  deployment:
    method: "kubespray"    # kubespray | talos | kamaji | eks | gke | aks | cluster-api
    
    kubespray:
      version: "v2.29.1"
      modules:
        kubespray_cluster:
          source: "github.com/rackerlabs/opencenter-gitops-base.git//iac/provider/kubespray?ref=main"
    
    # Kamaji - Hosted Control Plane with Cluster API
    kamaji:
      enabled: false
      version: "v1.0.0"
      
      # Control plane configuration
      control_plane:
        replicas: 3
        datastore: "etcd"  # etcd | postgresql | mysql
        
        # Etcd configuration (when datastore: etcd)
        etcd:
          storage_class: "csi-cinder-sc-delete"
          storage_size: "10Gi"
        
        # PostgreSQL configuration (when datastore: postgresql)
        postgresql:
          host: ""
          port: 5432
          database: "kamaji"
          ssl_mode: "require"
        
        # Control plane endpoint
        service_type: "LoadBalancer"  # LoadBalancer | NodePort
        api_server_port: 6443
      
      # Cluster API configuration
      cluster_api:
        version: "v1.6.0"
        providers:
          infrastructure: "openstack"  # openstack | aws | azure | vsphere
          bootstrap: "kubeadm"
          control_plane: "kubeadm"
      
      # Worker node pools (mixed OS support)
      worker_pools:
        # Ubuntu workers via Kubespray/CAPI
        - name: "ubuntu-workers"
          os: "ubuntu"
          count: 3
          flavor: "gp.0.4.16"
          image: "ubuntu-24.04-k8s"
          bootstrap_provider: "kubeadm"
        
        # Windows workers via Kubespray/CAPI
        - name: "windows-workers"
          os: "windows"
          count: 2
          flavor: "gp.5.4.16"
          image: "windows-2022-k8s"
          bootstrap_provider: "kubeadm"
        
        # Talos workers via CAPI
        - name: "talos-workers"
          os: "talos"
          count: 3
          flavor: "gp.0.4.16"
          image: "talos-v1.8.0"
          bootstrap_provider: "talos"
          talos_version: "v1.8.0"
      
      modules:
        kamaji:
          source: "github.com/rackerlabs/opencenter-gitops-base.git//iac/deployment/kamaji?ref=main"
        cluster_api:
          source: "github.com/rackerlabs/opencenter-gitops-base.git//iac/deployment/cluster-api?ref=main"
    
  # Talos configuration (standalone or as worker pool in Kamaji)
  talos:
    enabled: false
    version: "v1.8.0"
    image_url: "https://github.com/siderolabs/talos/releases/..."
    
    machine_config:
      app_armor_enabled: true
      seccomp_enabled: true
      disk_encryption: true
    
    network_config:
      management_subnet: "10.0.1.0/24"
      control_subnet: "10.0.2.0/24"
      data_subnet: "10.0.3.0/24"
    
    security_config:
      vtpm_enabled: true
      image_verification: true
      audit_log_enabled: true
```

**Deployment Method Comparison**:

```
┌─────────────────────────────────────────────────────────────┐
│  Deployment Method    │  Control Plane  │  Worker Support   │
├───────────────────────┼─────────────────┼──────────────────┤
│  Kubespray            │  Self-hosted    │  Ubuntu, Windows  │
│  Talos                │  Self-hosted    │  Talos only       │
│  Kamaji + CAPI        │  Hosted (Kamaji)│  Mixed (all OS)   │
│  EKS/GKE/AKS          │  Managed        │  Provider-managed │
│  Cluster API          │  Self-hosted    │  Provider-specific│
└───────────────────────┴─────────────────┴──────────────────┘
```

**Kamaji Architecture**:

```
┌─────────────────────────────────────────────────────────────┐
│  Management Cluster (Kamaji + CAPI)                          │
│  ├── Kamaji Control Plane Manager                            │
│  ├── Cluster API Controllers                                 │
│  └── Hosted Control Planes (etcd/postgres)                   │
├─────────────────────────────────────────────────────────────┤
│  Tenant Cluster (Workload)                                   │
│  ├── Control Plane → Hosted in Management Cluster            │
│  └── Worker Nodes → Deployed via CAPI                        │
│      ├── Ubuntu workers (kubeadm bootstrap)                  │
│      ├── Windows workers (kubeadm bootstrap)                 │
│      └── Talos workers (talos bootstrap)                     │
└─────────────────────────────────────────────────────────────┘
```

**Deployment Method Abstraction**:

```
┌─────────────────────────────────────────────────────────────┐
│  Infrastructure Provider    │    Deployment Method           │
│  (WHERE to deploy)          │    (HOW to deploy)             │
├─────────────────────────────┼───────────────────────────────┤
│  OpenStack                  │    Kubespray                   │
│  AWS                        │    Talos                       │
│  GCP                        │    Cluster API                 │
│  Azure                      │    EKS/GKE/AKS (managed)       │
│  Bare Metal                 │                                │
│  VMware                     │                                │
└─────────────────────────────┴───────────────────────────────┘

Supported Combinations:
  OpenStack + Kubespray  ✓
  OpenStack + Talos      ✓
  AWS + Kubespray        ✓
  AWS + EKS              ✓
  GCP + GKE              ✓
  Bare Metal + Kubespray ✓
  Bare Metal + Talos     ✓
```

---

### Services Domain

Platform workloads deployed via GitOps.

```yaml
opencenter:
  # Self-hosted services (deployed in-cluster)
  services:
    calico:
      enabled: true
      calico_kube_api_server: "https://api.prod.k8s.acme.com:6443"
    
    cert-manager:
      enabled: true
      email: "admin@acme.com"
      letsencrypt_server: "https://acme-v02.api.letsencrypt.org/directory"
      region: "us-east-1"
    
    loki:
      enabled: true
      namespace: "monitoring"
      storage_type: "swift"           # swift | s3
      bucket_name: "prod-cluster-loki"
      volume_size: 20
      storage_class: "csi-cinder-sc-delete"
      
      # Swift configuration (when storage_type: swift)
      swift_auth_url: "https://keystone.api.sjc3.rackspacecloud.com/v3/"
      swift_region: "SJC3"
      swift_domain_name: "Default"
    
    tempo:
      enabled: false
      storage_type: "s3"
      bucket_name: "prod-cluster-tempo"
      s3_endpoint: ""
      s3_region: "us-east-1"
    
    kube-prometheus-stack:
      enabled: true
      prometheus_volume_size: 50
      prometheus_storage_class: "csi-cinder-sc-delete"
      grafana_volume_size: 10
      alertmanager_volume_size: 10
    
    keycloak:
      enabled: false
      hostname: "auth.prod.k8s.acme.com"
      realm: "opencenter"
      frontend_url: "https://auth.prod.k8s.acme.com"
    
    headlamp:
      enabled: false
      hostname: "dashboard.prod.k8s.acme.com"
      oidc_issuer_url: "https://auth.prod.k8s.acme.com/realms/opencenter"
      oidc_client_id: "kubernetes"
    
    velero:
      enabled: false
      backup_bucket: "prod-cluster-backups"
      region: "us-east-1"
    
    # Core services (typically always enabled)
    fluxcd:
      enabled: true
    gateway-api:
      enabled: true
    external-snapshotter:
      enabled: true
    openstack-ccm:
      enabled: true
    openstack-csi:
      enabled: true
  
  # Managed services (external/vendor-managed)
  managed_services:
    alert-proxy:
      enabled: false
      image_repository: "ghcr.io/rackerlabs/alert-proxy"
      image_tag: "latest"
      alert_manager_base_url: ""
      http_route_fqdn: "https://alerts.prod.k8s.acme.com"
```

**Services vs Managed Services**:

```
┌─────────────────────────────────────────────────────────────┐
│  services (Self-Hosted)                                      │
│  ├── Deployed in-cluster via GitOps                          │
│  ├── Managed by OpenCenter lifecycle                         │
│  ├── Uses cluster resources                                  │
│  └── Examples: loki, tempo, prometheus, keycloak             │
├─────────────────────────────────────────────────────────────┤
│  managed_services (External)                                 │
│  ├── Hosted outside cluster                                  │
│  ├── Requires external credentials/tokens                    │
│  ├── Different lifecycle (vendor-managed)                    │
│  └── Examples: alert-proxy, external monitoring              │
└─────────────────────────────────────────────────────────────┘
```

---

### Secrets Domain

Credentials and sensitive configuration.

```yaml
secrets:
  sops_age_key_file: ""
  
  ssh_key:
    private: "./secrets/ssh/prod-cluster"
    public: "./secrets/ssh/prod-cluster.pub"
    cypher: "ed25519"
  
  # Global secrets by scope
  global:
    aws:
      infrastructure:
        access_key: ""
        secret_access_key: ""
        region: "us-east-1"
      application:
        access_key: ""
        secret_access_key: ""
        region: ""
  
  # Service-specific secrets
  cert_manager:
    aws_access_key: ""
    aws_secret_access_key: ""
  
  loki:
    swift_password: ""
  
  tempo:
    access_key: ""
    secret_key: ""
  
  keycloak:
    client_secret: ""
    admin_password: ""
  
  headlamp:
    oidc_client_secret: ""
  
  grafana:
    admin_password: ""
  
  weave_gitops:
    password: ""
    password_hash: ""
  
  alert_proxy:
    core_device_id: ""
    account_service_token: ""
    core_account_number: ""
  
  vsphere_csi:
    vcenter_host: ""
    username: ""
    password: ""
    datacenters: ""
```

**Secrets Scoping**:

```
┌─────────────────────────────────────────────────────────────┐
│  secrets                                                     │
│  ├── global ──────────────► Infrastructure-wide credentials  │
│  │   ├── aws.infrastructure ► OpenTofu/provisioning          │
│  │   └── aws.application ───► Application-level access       │
│  │                                                           │
│  ├── <service> ───────────► Service-specific credentials     │
│  │   ├── cert_manager                                        │
│  │   ├── loki                                                │
│  │   ├── keycloak                                            │
│  │   └── ...                                                 │
│  │                                                           │
│  └── ssh_key ─────────────► Cluster access keys              │
└─────────────────────────────────────────────────────────────┘
```

---

## Reference Resolution

Configuration values can reference other fields using `${path.to.value}` syntax.

### Reference Syntax

```yaml
opencenter:
  cluster:
    networking:
      vrrp_ip: "10.2.128.10"
  
  services:
    calico:
      calico_kube_api_server: "https://${cluster.networking.vrrp_ip}:6443"
```

### Resolution Phases

```
┌─────────────────────────────────────────────────────────────┐
│  1. Parse ─────────────► Load YAML into structs              │
│  2. Normalize ─────────► Apply type coercion                 │
│  3. Resolve References ► Replace ${...} with values          │
│  4. Apply Defaults ────► Merge provider/region defaults      │
│  5. Validate ──────────► Schema + business rules             │
│  6. Freeze ────────────► Immutable configuration             │
└─────────────────────────────────────────────────────────────┘
```

### Dependency Graph

References form a directed acyclic graph (DAG). Cycles are rejected at validation.

```
cluster.networking.vrrp_ip
         │
         ├──► infrastructure.k8s_api_ip
         ├──► services.calico.calico_kube_api_server
         └──► deployment.kubespray.keepalived_vip
```

---

## Default Resolution Framework

### Precedence Order

```
┌─────────────────────────────────────────────────────────────┐
│  1. Cluster Config ────► User-specified values (highest)     │
│  2. CLI Overrides ─────► Command-line flags                  │
│  3. CLI Config ────────► ~/.config/opencenter/config.yaml    │
│  4. Provider-Region ───► Built-in provider+region defaults   │
│  5. Provider ──────────► Built-in provider defaults          │
│  6. Global ────────────► Built-in fallback (lowest)          │
└─────────────────────────────────────────────────────────────┘
```

### Provider-Region Registry

```yaml
# Built-in defaults by provider and region
defaults:
  providers:
    openstack:
      regions:
        sjc3:
          images:
            ubuntu-24: "799dcf97-3656-4361-8187-13ab1b295e33"
          availability_zones: ["az1", "az2", "az3"]
          ntp_servers: ["time.sjc3.rackspace.com"]
          dns_nameservers: ["8.8.8.8", "8.8.4.4"]
          flavors:
            bastion: "gp.0.2.2"
            master: "gp.0.4.8"
            worker: "gp.0.4.16"
        
        dfw3:
          images:
            ubuntu-24: "b9876543-4321-4321-4321-ba9876543210"
          availability_zones: ["az1", "az2"]
          ntp_servers: ["time.dfw3.rackspace.com"]
    
    aws:
      regions:
        us-east-1:
          images:
            ubuntu-24: "ami-0c55b159cbfafe1f0"
          availability_zones: ["us-east-1a", "us-east-1b", "us-east-1c"]
```

---

## Provider Architecture

### Provider Interface

All providers implement a common contract:

```
┌─────────────────────────────────────────────────────────────┐
│  ProviderInterface                                           │
├─────────────────────────────────────────────────────────────┤
│  Authenticate() error                                        │
│  ValidateConfig(cfg *Config) error                           │
│  ProvisionNetwork(cfg *NetworkConfig) error                  │
│  ProvisionCompute(cfg *ComputeConfig) error                  │
│  ProvisionStorage(cfg *StorageConfig) error                  │
│  GetProviderName() string                                    │
│  GetRegionDefaults(region string) *RegionDefaults            │
└─────────────────────────────────────────────────────────────┘
```

### Provider-Specific Extensions

Provider-specific fields are isolated under `infrastructure.cloud.<provider>`:

```yaml
infrastructure:
  provider: "openstack"
  
  cloud:
    openstack:
      # OpenStack-specific fields
      auth_url: "..."
      use_octavia: true
      use_designate: true
      networking:
        floating_ip_pool: "PUBLICNET"
        designate:
          dns_zone_name: "..."
    
    aws:
      # AWS-specific fields (ignored when provider != aws)
      vpc_id: "..."
      private_subnets: []
```

---

## Service Provider Polymorphism

Services with provider-specific needs use adapter pattern.

### Example: cert-manager DNS Providers

```yaml
services:
  cert-manager:
    enabled: true
    email: "admin@acme.com"
    
    dns_challenge:
      provider: "route53"    # route53 | cloudflare | designate
      
      route53:
        region: "us-east-1"
        hosted_zone_id: "Z1234567890ABC"
      
      cloudflare:
        email: "admin@acme.com"
      
      designate:
        auth_url: "${infrastructure.cloud.openstack.auth_url}"
        region: "${infrastructure.cloud.openstack.region}"
```

### Infrastructure-Aware Defaults

Services auto-select providers based on infrastructure:

```
┌─────────────────────────────────────────────────────────────┐
│  Infrastructure Provider  │  Default Service Provider        │
├───────────────────────────┼─────────────────────────────────┤
│  aws                      │  route53                         │
│  openstack (designate)    │  designate                       │
│  openstack (no designate) │  cloudflare                      │
│  gcp                      │  google-cloud-dns                │
└───────────────────────────┴─────────────────────────────────┘
```

---

## Complete Configuration Examples

### Kubespray Deployment Example

A complete production-ready configuration example is available in [`cluster-config-full.yaml`](./cluster-config-full.yaml).

This example demonstrates:

- **OpenStack provider** with full networking and storage configuration
- **Kubespray deployment** with Calico CNI
- **Infrastructure storage** with boot volumes and additional block devices
- **Additional worker pools** for specialized workloads (high-memory, GPU)
- **Comprehensive services** including monitoring (Prometheus, Loki, Tempo), identity (Keycloak), and GitOps (FluxCD, Weave GitOps)
- **OIDC integration** for cluster authentication
- **Secrets management** with SOPS/Age encryption
- **S3 backend** for OpenTofu state

#### Key Configuration Highlights

```yaml
# Infrastructure with compute and storage
infrastructure:
  provider: "openstack"
  
  compute:
    flavor_master: "gp.0.4.8"
    flavor_worker: "gp.0.4.16"
    master_count: 3
    worker_count: 5
  
  storage:
    default_storage_class: "csi-cinder-sc-delete"
    worker_volume_size: 100
    worker_volume_type: "HA-Standard"
    additional_block_devices: []

# Cluster with additional worker pools
cluster:
  kubernetes:
    version: "1.33.5"
    additional_server_pools_worker:
      - name: "high-memory"
        worker_count: 2
        flavor_worker: "gp.0.8.64"
        worker_node_bfv_volume_size: 200
        additional_block_devices_worker:
          - device_name: "/dev/vdb"
            volume_size: 1000

# Services with provider-specific configuration
services:
  loki:
    enabled: true
    storage_type: "swift"  # OpenStack Swift
  tempo:
    enabled: true
    storage_type: "swift"
  cert-manager:
    enabled: true
    region: "us-east-1"  # AWS Route53 for DNS challenges
```

See the [full example file](./cluster-config-full.yaml) for complete configuration details.

---

### Kamaji Hosted Control Plane Example

A complete Kamaji deployment example is available in [`cluster-config-kamaji.yaml`](./cluster-config-kamaji.yaml).

This example demonstrates:

- **Kamaji hosted control plane** running in a management cluster
- **Cluster API (CAPI)** for worker node provisioning
- **Mixed worker OS support**: Ubuntu (kubeadm), Windows (kubeadm), and Talos
- **OpenStack infrastructure provider** with CAPI OpenStack provider
- **Multiple worker pools** with different OS types and autoscaling
- **Etcd datastore** for control plane state (alternative: PostgreSQL)
- **LoadBalancer service** for control plane API endpoint

#### Key Kamaji Configuration Highlights

```yaml
# Deployment method: Kamaji
deployment:
  method: "kamaji"
  
  kamaji:
    enabled: true
    version: "v1.0.0"
    
    # Hosted control plane configuration
    control_plane:
      replicas: 3
      datastore: "etcd"
      service_type: "LoadBalancer"
      api_server_port: 6443
    
    # Cluster API configuration
    cluster_api:
      version: "v1.6.0"
      providers:
        infrastructure: "openstack"
        bootstrap: "kubeadm"
    
    # Mixed worker pools
    worker_pools:
      # Ubuntu workers
      - name: "ubuntu-workers"
        os: "ubuntu"
        count: 3
        flavor: "gp.0.4.16"
        bootstrap_provider: "kubeadm"
      
      # Windows workers
      - name: "windows-workers"
        os: "windows"
        count: 2
        flavor: "gp.5.4.16"
        bootstrap_provider: "kubeadm"
      
      # Talos workers
      - name: "talos-workers"
        os: "talos"
        count: 3
        flavor: "gp.0.4.16"
        bootstrap_provider: "talos"
        talos_version: "v1.8.0"
```

#### Kamaji Architecture Benefits

**Management Cluster vs Tenant Cluster**:

```
┌─────────────────────────────────────────────────────────────┐
│  Management Cluster                                          │
│  ├── Kamaji Controller                                       │
│  ├── Cluster API Controllers                                 │
│  ├── Hosted Control Planes (multiple tenant clusters)        │
│  │   ├── tenant-cluster-01-apiserver                         │
│  │   ├── tenant-cluster-01-controller-manager                │
│  │   ├── tenant-cluster-01-scheduler                         │
│  │   └── tenant-cluster-02-* (additional tenants)            │
│  └── Etcd/PostgreSQL (control plane datastore)               │
├─────────────────────────────────────────────────────────────┤
│  Tenant Cluster (tenant-cluster-01)                          │
│  ├── Control Plane → Hosted in Management Cluster            │
│  └── Worker Nodes → Deployed via CAPI                        │
│      ├── Ubuntu workers (3 nodes)                            │
│      ├── Windows workers (2 nodes)                           │
│      └── Talos workers (3 nodes)                             │
└─────────────────────────────────────────────────────────────┘
```

**Key Benefits**:
- **Multi-tenancy**: Single management cluster hosts multiple tenant control planes
- **Resource efficiency**: Control plane resources shared across tenants
- **Simplified operations**: Upgrade/patch control planes without touching worker nodes
- **Mixed OS support**: Ubuntu, Windows, and Talos workers in same cluster
- **Autoscaling**: Per-pool autoscaling with CAPI MachineDeployments
- **High availability**: Control plane HA managed by Kamaji

See the [full Kamaji example file](./cluster-config-kamaji.yaml) for complete configuration details.

---

## Migration Guide

### From v1.x to v2.0

#### Phase 1: Dual-Write (Backward Compatible)

```bash
# Migrate existing configuration
opencenter cluster migrate-config prod-cluster --output v2

# Validate migrated configuration
opencenter cluster validate prod-cluster --schema-version 2.0
```

#### Phase 2: Key Changes

| v1.x Location | v2.0 Location | Notes |
|---------------|---------------|-------|
| `cluster.networking.vrrp_ip` | `infrastructure.networking.vrrp_ip` | Moved to infrastructure |
| `cluster.networking.*` | `infrastructure.networking.*` | All infrastructure networking moved |
| `cluster.ssh_authorized_keys` | `infrastructure.ssh.authorized_keys` | SSH is infrastructure access |
| `cluster.aws_access_key` | `secrets.global.aws.infrastructure.*` | Moved to secrets |
| `cluster.kubernetes.kubespray_version` | `deployment.kubespray.version` | Deployment method config |
| `cluster.kubernetes.flavor_*` | `infrastructure.compute.flavor_*` | Compute is infrastructure |
| `cluster.kubernetes.*_count` | `infrastructure.compute.*_count` | Node counts are infrastructure |
| `opencenter.storage.*` | `infrastructure.storage.*` | Storage is infrastructure |

#### Phase 3: Validation

```bash
# Run preflight checks with new schema
opencenter cluster preflight prod-cluster

# Verify service configurations
opencenter cluster validate prod-cluster --services
```

---

### Migrating to Kamaji Hosted Control Plane

Organizations can migrate existing Kubespray or Talos clusters to Kamaji for improved multi-tenancy and operational efficiency.

#### Migration Strategy

**Option 1: Blue-Green Migration** (Recommended)
1. Deploy new Kamaji-based cluster alongside existing cluster
2. Migrate workloads using GitOps or backup/restore
3. Update DNS/load balancers to point to new cluster
4. Decommission old cluster

**Option 2: In-Place Migration** (Advanced)
1. Deploy management cluster with Kamaji
2. Migrate control plane to Kamaji (requires downtime)
3. Convert worker nodes to CAPI-managed MachineDeployments
4. Validate cluster functionality

#### Configuration Changes for Kamaji Migration

**From Kubespray to Kamaji**:

```yaml
# OLD: Kubespray configuration
deployment:
  method: "kubespray"
  kubespray:
    version: "v2.29.1"

infrastructure:
  compute:
    master_count: 3      # ❌ Remove (control plane hosted)
    worker_count: 5      # ❌ Move to worker_pools
  networking:
    vrrp_ip: "10.2.128.10"  # ❌ Remove (managed by Kamaji)
    vrrp_enabled: true      # ❌ Remove

cluster:
  kubernetes:
    kube_vip_enabled: true  # ❌ Remove

# NEW: Kamaji configuration
deployment:
  method: "kamaji"
  kamaji:
    enabled: true
    version: "v1.0.0"
    
    control_plane:
      replicas: 3          # ✅ Matches old master_count
      datastore: "etcd"
      service_type: "LoadBalancer"
    
    cluster_api:
      version: "v1.6.0"
      providers:
        infrastructure: "openstack"
        bootstrap: "kubeadm"
    
    worker_pools:
      - name: "default-workers"
        os: "ubuntu"
        count: 5           # ✅ Matches old worker_count
        flavor: "gp.0.4.16"
        bootstrap_provider: "kubeadm"

infrastructure:
  compute:
    master_count: 0        # ✅ No masters in Kamaji
    worker_count: 0        # ✅ Defined in worker_pools
  networking:
    vrrp_ip: ""            # ✅ Not used
    vrrp_enabled: false    # ✅ Disabled

cluster:
  kubernetes:
    kube_vip_enabled: false  # ✅ Disabled
```

**From Talos to Kamaji with Talos Workers**:

```yaml
# OLD: Talos standalone
deployment:
  method: "talos"

talos:
  enabled: true
  version: "v1.8.0"

infrastructure:
  compute:
    master_count: 3
    worker_count: 5

# NEW: Kamaji with Talos workers
deployment:
  method: "kamaji"
  kamaji:
    enabled: true
    
    control_plane:
      replicas: 3
      datastore: "etcd"
    
    cluster_api:
      version: "v1.6.0"
      providers:
        infrastructure: "openstack"
        bootstrap: "talos"      # ✅ Use Talos bootstrap
    
    worker_pools:
      - name: "talos-workers"
        os: "talos"
        count: 5
        flavor: "gp.0.4.16"
        bootstrap_provider: "talos"
        talos_version: "v1.8.0"
        
        talos_config:
          machine_config:
            app_armor_enabled: true
            seccomp_enabled: true
            disk_encryption: true

talos:
  enabled: false  # ✅ Talos config now in worker_pools
```

#### Migration Checklist

- [ ] Deploy management cluster with Kamaji and CAPI controllers
- [ ] Configure CAPI infrastructure provider (OpenStack, AWS, etc.)
- [ ] Create Kamaji TenantControlPlane resource
- [ ] Verify control plane API endpoint is accessible
- [ ] Deploy worker pools via CAPI MachineDeployments
- [ ] Migrate workloads to new cluster
- [ ] Update monitoring and logging integrations
- [ ] Update GitOps repository references
- [ ] Test disaster recovery procedures
- [ ] Decommission old cluster

#### Benefits of Kamaji Migration

**Operational Benefits**:
- Reduced infrastructure costs (shared control plane resources)
- Simplified control plane upgrades (no worker node impact)
- Improved multi-tenancy (multiple tenant clusters per management cluster)
- Centralized control plane management

**Technical Benefits**:
- Mixed OS worker support (Ubuntu + Windows + Talos in same cluster)
- Per-pool autoscaling with CAPI
- Declarative infrastructure management
- Better separation of concerns (control plane vs data plane)

**Use Cases**:
- Multi-tenant SaaS platforms
- Development/staging environments (many small clusters)
- Edge deployments (centralized control, distributed workers)
- Hybrid cloud (control plane in one cloud, workers in another)

---

## Appendix: Configuration Validation Rules

### Required Fields by Provider

| Provider | Required Fields |
|----------|-----------------|
| `openstack` | `auth_url`, `region`, `tenant_name`, `image_id` |
| `aws` | `region`, `vpc_id` |
| `gcp` | `project_id`, `region` |
| `baremetal` | `master_nodes`, `worker_nodes` |

### Required Fields by Deployment Method

| Deployment Method | Required Fields |
|-------------------|-----------------|
| `kubespray` | `version`, `infrastructure.compute.master_count`, `infrastructure.compute.worker_count` |
| `talos` | `version`, `image_url`, `infrastructure.compute.master_count`, `infrastructure.compute.worker_count` |
| `kamaji` | `version`, `control_plane.replicas`, `control_plane.datastore`, `cluster_api.version`, `worker_pools` |
| `eks` | `region`, `vpc_id`, `node_groups` |
| `gke` | `project_id`, `region`, `node_pools` |
| `aks` | `resource_group`, `location`, `node_pools` |

### Kamaji-Specific Validation Rules

**Control Plane Configuration**:
- `control_plane.replicas` must be odd number (1, 3, 5) for HA
- `control_plane.datastore` must be one of: `etcd`, `postgresql`, `mysql`
- When `datastore: etcd`, `etcd.storage_class` and `etcd.storage_size` are required
- When `datastore: postgresql`, `postgresql.host`, `postgresql.database` are required
- `control_plane.service_type` must be one of: `LoadBalancer`, `NodePort`

**Worker Pool Configuration**:
- At least one worker pool must be defined
- Each pool must have unique `name`
- `os` must be one of: `ubuntu`, `windows`, `talos`
- `bootstrap_provider` must match OS:
  - Ubuntu/Windows: `kubeadm`
  - Talos: `talos`
- When `os: talos`, `talos_version` is required
- `count` must be >= 1
- When `autoscaling.enabled: true`, `min_replicas` <= `count` <= `max_replicas`

**Cluster API Configuration**:
- `cluster_api.providers.infrastructure` must match `infrastructure.provider`
- `cluster_api.providers.bootstrap` must be one of: `kubeadm`, `talos`
- Infrastructure provider must support CAPI (openstack, aws, azure, vsphere, metal3)

**Incompatible Configurations**:
- Cannot use `infrastructure.compute.master_count` with Kamaji (control plane is hosted)
- Cannot use `infrastructure.networking.vrrp_ip` with Kamaji (API endpoint managed by Kamaji)
- Cannot use `cluster.kubernetes.kube_vip_enabled` with Kamaji

### Service Dependencies

```
┌─────────────────────────────────────────────────────────────┐
│  Service Dependencies                                        │
├─────────────────────────────────────────────────────────────┤
│  headlamp ──────────► keycloak (OIDC)                        │
│  weave-gitops ──────► fluxcd                                 │
│  loki ──────────────► kube-prometheus-stack (optional)       │
│  tempo ─────────────► kube-prometheus-stack (optional)       │
│  cert-manager ──────► gateway-api (for HTTPRoute)            │
│  openstack-csi ─────► openstack-ccm                          │
│  kamaji ────────────► cluster-api                            │
│  cluster-api ───────► infrastructure provider (CAPI)         │
└─────────────────────────────────────────────────────────────┘
```

### Deployment Method Compatibility Matrix

| Infrastructure Provider | Kubespray | Talos | Kamaji | EKS | GKE | AKS |
|------------------------|-----------|-------|--------|-----|-----|-----|
| OpenStack              | ✓         | ✓     | ✓      | ✗   | ✗   | ✗   |
| AWS                    | ✓         | ✓     | ✓      | ✓   | ✗   | ✗   |
| GCP                    | ✓         | ✓     | ✓      | ✗   | ✓   | ✗   |
| Azure                  | ✓         | ✓     | ✓      | ✗   | ✗   | ✓   |
| Bare Metal             | ✓         | ✓     | ✓*     | ✗   | ✗   | ✗   |
| VMware                 | ✓         | ✓     | ✓      | ✗   | ✗   | ✗   |

*Requires Metal3 CAPI provider for Kamaji on bare metal

### Validation Error Codes

| Code | Description |
|------|-------------|
| `E001` | Missing required field |
| `E002` | Invalid CIDR notation |
| `E003` | Reference resolution failed |
| `E004` | Provider-specific validation failed |
| `E005` | Service dependency not met |
| `E006` | Secret not configured for enabled service |
| `E007` | Incompatible provider-service combination |
| `E008` | Deployment method not supported for provider |
| `E009` | Kamaji control plane configuration invalid |
| `E010` | Worker pool configuration invalid |
| `E011` | Cluster API provider mismatch |
| `E012` | Autoscaling configuration invalid |
| `E013` | Mixed OS worker pool requires Kamaji or CAPI |
