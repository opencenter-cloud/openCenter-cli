# Cluster Configuration Structure Recommendations

## Table of Contents

- [Executive Summary](#executive-summary)
- [Current State Analysis](#current-state-analysis)
  - [Configuration Hierarchy](#configuration-hierarchy)
  - [Identified Issues](#identified-issues)
- [Recommendations](#recommendations)
  - [1. Establish Clear Configuration Hierarchy](#1-establish-clear-configuration-hierarchy)
  - [2. Eliminate Configuration Duplication](#2-eliminate-configuration-duplication)
  - [3. Provider-Agnostic Core Configuration](#3-provider-agnostic-core-configuration)
  - [4. Deployment Method Abstraction](#4-deployment-method-abstraction)
  - [5. Shared Resource References](#5-shared-resource-references)
  - [6. Provider-Specific Defaults and Regional Configuration](#6-provider-specific-defaults-and-regional-configuration)
  - [7. Service Configuration Polymorphism](#7-service-configuration-polymorphism)
- [Future State Architecture](#future-state-architecture)
  - [Proposed Structure](#proposed-structure)
  - [Configuration Resolution Order](#configuration-resolution-order)
  - [Migration Path](#migration-path)
- [Implementation Priorities](#implementation-priorities)
- [Conclusion](#conclusion)

## Table of Contents

- [Executive Summary](#executive-summary)
- [Current State Analysis](#current-state-analysis)
  - [Configuration Hierarchy](#configuration-hierarchy)
  - [Identified Issues](#identified-issues)
- [Recommendations](#recommendations)
  - [1. Establish Clear Configuration Hierarchy](#1-establish-clear-configuration-hierarchy)
  - [2. Eliminate Configuration Duplication](#2-eliminate-configuration-duplication)
  - [3. Provider-Agnostic Core Configuration](#3-provider-agnostic-core-configuration)
  - [4. Deployment Method Abstraction](#4-deployment-method-abstraction)
  - [5. Shared Resource References](#5-shared-resource-references)
- [Future State Architecture](#future-state-architecture)
  - [Proposed Structure](#proposed-structure)
  - [Configuration Resolution Order](#configuration-resolution-order)
  - [Migration Path](#migration-path)
- [Implementation Priorities](#implementation-priorities)

## Executive Summary

The opencenter cluster configuration currently suffers from significant duplication and unclear ownership of shared settings across providers (OpenStack, AWS, GCP, Azure, bare metal) and deployment methods (Kubespray, Talos, managed services). This document proposes a hierarchical configuration model where shared settings are defined once at the appropriate level and referenced by provider/deployment-specific configurations.

**Key Recommendation**: Establish a three-tier hierarchy (Cluster → Infrastructure → Provider/Deployment) with explicit reference patterns for shared resources like VRRP IPs, networking, and security settings.

## Current State Analysis

### Configuration Hierarchy

The current configuration structure is defined across multiple type files:

```
Config (root)
├── OpenCenter
│   ├── Meta (cluster identity)
│   ├── Secrets (OpenCenterSecrets - Barbican config)
│   ├── Cluster
│   │   ├── ClusterName, BaseDomain, AdminEmail
│   │   ├── Networking (ClusterNetworkingConfig)
│   │   │   ├── VRRPIP ⚠️ DUPLICATED
│   │   │   ├── VRRPEnabled ⚠️ DUPLICATED
│   │   │   ├── UseOctavia ⚠️ DUPLICATED
│   │   │   ├── LoadbalancerProvider ⚠️ DUPLICATED
│   │   │   ├── DNSNameservers, NTPServers
│   │   │   ├── SubnetNodes, AllocationPool*
│   │   │   └── VLAN ⚠️ DUPLICATED
│   │   └── Kubernetes
│   │       ├── Version, APIPort
│   │       ├── SubnetPods, SubnetServices
│   │       ├── LoadbalancerProvider ⚠️ DUPLICATED
│   │       ├── DNSZoneName ⚠️ DUPLICATED
│   │       ├── Networking (Networking struct) ⚠️ DUPLICATED
│   │       ├── NetworkPlugin (Calico/Cilium/KubeOVN)
│   │       └── Modules
│   ├── Infrastructure
│   │   ├── Provider (openstack/aws/gcp/azure/baremetal)
│   │   ├── K8sAPIIP ⚠️ Related to VRRPIP
│   │   └── Cloud
│   │       ├── OpenStack
│   │       │   ├── Networking
│   │       │   │   ├── VLAN ⚠️ DUPLICATED
│   │       │   │   └── K8sAPIPortACL
│   │       │   └── VRRPIP ⚠️ DUPLICATED (in OpenStackCloud)
│   │       └── AWS
│   ├── GitOps (repository configuration)
│   ├── Storage (default storage class, volume settings)
│   ├── Talos (optional deployment method)
│   │   ├── NetworkConfig
│   │   │   ├── ManagementSubnet
│   │   │   ├── ControlSubnet
│   │   │   └── DataSubnet
│   │   └── SecurityConfig
│   ├── ManagedService (ServiceMap)
│   │   └── alert-proxy (AlertProxyConfig)
│   └── Services (ServiceMap)
│       ├── calico, cert-manager, etcd-backup
│       ├── external-snapshotter, fluxcd, gateway
│       ├── gateway-api, headlamp, keycloak
│       ├── kube-prometheus-stack, kyverno, loki
│       ├── olm, openstack-ccm, openstack-csi
│       ├── postgres-operator, rbac-manager, sources
│       ├── tempo, velero, vsphere-csi, weave-gitops
│       └── (each service has BaseConfig + service-specific fields)
├── OpenTofu (IaC backend configuration)
├── Secrets (global secrets, service-specific secrets)
└── Deployment (auto_deploy flag)
```

### Identified Issues

#### 1. **VRRP Configuration Duplication**

**Current State**: VRRP IP appears in multiple locations:
- `OpenCenter.Cluster.Networking.VRRPIP`
- `OpenCenter.Infrastructure.Cloud.OpenStack.VRRPIP` (legacy)
- `OpenCenter.Infrastructure.K8sAPIIP` (related concept)
- Referenced by: Kubernetes, Kubespray, Talos, OpenStack

**Problem**: No single source of truth. Different providers may read from different locations, causing inconsistency.

#### 2. **Networking Configuration Fragmentation**

**Current State**: Network settings scattered across:
- `ClusterNetworkingConfig` (cluster level)
- `Networking` struct (kubernetes level) - **DUPLICATE**
- `OpenStackNetworkingConfig` (provider level)
- `TalosNetworkConfig` (deployment method level)

**Problem**: Overlapping responsibilities. `SubnetNodes`, `DNSNameservers`, `NTPServers` are cluster-wide but duplicated in multiple scopes.

#### 3. **Load Balancer Provider Ambiguity**

**Current State**: `LoadbalancerProvider` appears in:
- `ClusterNetworkingConfig.LoadbalancerProvider`
- `KubernetesConfig.LoadbalancerProvider`

**Problem**: Unclear which takes precedence. Should be cluster-wide decision, not per-deployment-method.

#### 4. **VLAN Configuration Duplication**

**Current State**: VLAN struct appears in:
- `ClusterNetworkingConfig.VLAN`
- `OpenStackNetworkingConfig.VLAN`

**Problem**: VLAN is infrastructure-specific but duplicated at cluster level.

#### 5. **DNS Zone Name Duplication**

**Current State**: DNS zone appears in:
- `ClusterNetworkingConfig.DNSZoneName`
- `KubernetesConfig.DNSZoneName`
- `OpenStackNetworkingConfig.Designate.DNSZoneName`

**Problem**: DNS is cluster-wide infrastructure, not deployment-method-specific.

#### 6. **Provider-Specific Settings in Generic Structs**

**Current State**: 
- `UseOctavia` (OpenStack-specific) in `ClusterNetworkingConfig`
- `UseDesignate` (OpenStack-specific) in `ClusterNetworkingConfig`

**Problem**: Generic cluster config polluted with provider-specific flags.

#### 7. **Deployment Method Coupling**

**Current State**: Kubespray and Talos configurations are siblings under `OpenCenter`, but both need to reference the same cluster-level settings.

**Problem**: No clear abstraction for "deployment method" vs "infrastructure provider". Talos on AWS and Kubespray on AWS should share AWS infrastructure config but have different deployment configs.

#### 8. **Services vs ManagedService Distinction**

**Current State**: Two separate ServiceMap collections:
- `OpenCenter.Services` - Contains 20+ services (calico, cert-manager, loki, tempo, etc.)
- `OpenCenter.ManagedService` - Contains managed services (alert-proxy)

**Problem**: Unclear distinction between "services" and "managed-service". Both use the same ServiceMap type and service registry pattern. The separation appears arbitrary and may cause confusion about where new services should be added.

**Questions**:
- What makes a service "managed" vs regular?
- Should this be a property of the service config rather than separate collections?
- Are there different lifecycle/deployment patterns for managed services?

#### 9. **Provider-Specific Defaults and Regional Variations**

**Current State**: Default values are hardcoded in `defaultConfig()` function with minimal regional awareness:
- NTP servers use region-based templating: `time.{region}.rackspace.com`
- Most other defaults are static regardless of provider or region
- No mechanism for provider-specific default overrides

**Problem**: Different providers and regions have different optimal defaults:
- OpenStack regions have different image IDs, availability zones, network configurations
- AWS regions have different AMI IDs, availability zones, endpoint URLs
- GCP regions have different machine types, zones, network topologies
- Provider-specific services (Octavia, Designate, Route53, CloudFlare) need different defaults

**Examples of Missing Regional Defaults**:
- OpenStack image IDs vary by region (current: hardcoded single ID)
- AWS AMI IDs are region-specific
- Availability zones differ per region (current: hardcoded "az1")
- DNS endpoints vary by provider and region
- Storage classes differ by provider

#### 10. **Service Configuration Polymorphism**

**Current State**: Services like cert-manager have provider-specific configuration needs:
- DNS verification can use Route53 (AWS), CloudFlare, Designate (OpenStack), or other providers
- Current structure doesn't support multiple DNS provider configurations
- No clear pattern for service-level provider selection

**Problem**: Services need to adapt to infrastructure provider but lack polymorphic configuration:
- cert-manager needs different DNS challenge providers based on infrastructure
- Backup services (velero) need different storage backends per provider
- Monitoring/logging services may need provider-specific integrations
- No standardized way to express "use Route53 if AWS, use Designate if OpenStack"

**Example**: cert-manager DNS challenge configuration
```yaml
# Current: Single provider assumed
services:
  cert-manager:
    enabled: true
    letsencrypt_server: "https://acme-v02.api.letsencrypt.org/directory"
    email: "admin@example.com"
    # Missing: Which DNS provider? Route53? CloudFlare? Designate?
```

## Recommendations

### 1. Establish Clear Configuration Hierarchy

**Principle**: Settings should be defined at the highest applicable level and referenced by lower levels.

**Hierarchy Levels**:
1. **Cluster** - Identity, domain, admin contact, cluster-wide policies
2. **Infrastructure** - Provider-agnostic infrastructure (networking topology, security)
3. **Provider** - Provider-specific settings (OpenStack, AWS, GCP, Azure, bare metal)
4. **Deployment** - Deployment method settings (Kubespray, Talos, managed K8s)
5. **Services** - Application-level services (monitoring, logging, etc.)

**Rationale**: This hierarchy ensures that:
- Shared settings are defined once
- Provider-specific settings don't pollute generic configs
- Deployment methods can reference infrastructure settings without duplication
- Multi-provider and multi-deployment scenarios are supported

### 2. Eliminate Configuration Duplication

#### 2.1 VRRP IP Ownership

**Recommendation**: VRRP IP should live in `Infrastructure.Networking.VRRPIP` (not cluster, not provider).

**Rationale**:
- VRRP is an infrastructure-level HA mechanism
- Used by Kubernetes API server (all deployment methods)
- May be used by provider-specific load balancers
- Not cluster identity (like domain name), but infrastructure implementation detail

**References**:
- `Kubernetes` → references `Infrastructure.Networking.VRRPIP` for API server VIP
- `Kubespray` → references `Infrastructure.Networking.VRRPIP` for keepalived config
- `Talos` → references `Infrastructure.Networking.VRRPIP` for control plane VIP
- `OpenStack` → may use for floating IP allocation

**Migration**:
```yaml
# OLD (multiple locations)
opencenter:
  cluster:
    networking:
      vrrp_ip: "10.2.128.10"  # ❌ Remove
  infrastructure:
    k8s_api_ip: "10.2.128.10"  # ❌ Remove
    cloud:
      openstack:
        vrrp_ip: "10.2.128.10"  # ❌ Remove

# NEW (single source of truth)
opencenter:
  infrastructure:
    networking:
      vrrp_ip: "10.2.128.10"  # ✅ Single source
      vrrp_enabled: true
```

#### 2.2 Networking Configuration Consolidation

**Recommendation**: Create `Infrastructure.Networking` as the authoritative source for all infrastructure networking.

**Structure**:
```yaml
opencenter:
  infrastructure:
    networking:
      # Physical/Virtual Network Topology
      subnet_nodes: "10.2.128.0/22"
      allocation_pool_start: "10.2.128.100"
      allocation_pool_end: "10.2.128.200"
      
      # High Availability
      vrrp_ip: "10.2.128.10"
      vrrp_enabled: true
      
      # Load Balancing (cluster-wide decision)
      loadbalancer_provider: "ovn"  # ovn/octavia/metallb/cloud-native
      
      # DNS Infrastructure
      dns_nameservers: ["8.8.8.8", "8.8.4.4"]
      dns_zone_name: "k8s.example.com"
      
      # Time Synchronization
      ntp_servers: ["time.example.com"]
      
      # Security
      k8s_api_port_acl: ["0.0.0.0/0"]
```

**Kubernetes-Specific Networking** (remains in `Cluster.Kubernetes.Networking`):
```yaml
opencenter:
  cluster:
    kubernetes:
      networking:
        # Kubernetes Internal Networking (CNI-managed)
        subnet_pods: "10.42.0.0/16"
        subnet_services: "10.43.0.0/16"
        
        # CNI Plugin Selection
        network_plugin: "calico"  # calico/cilium/kube-ovn
```

**Rationale**:
- Infrastructure networking (nodes, VIPs, DNS) is provider/deployment-agnostic
- Kubernetes networking (pods, services) is deployment-method-specific
- Clear separation prevents confusion

#### 2.3 Provider-Specific Settings Isolation

**Recommendation**: Move provider-specific flags to provider-specific sections.

**Before**:
```yaml
opencenter:
  cluster:
    networking:
      use_octavia: true  # ❌ OpenStack-specific in generic config
      use_designate: true  # ❌ OpenStack-specific in generic config
```

**After**:
```yaml
opencenter:
  infrastructure:
    provider: "openstack"
    cloud:
      openstack:
        networking:
          use_octavia: true  # ✅ Provider-specific
          use_designate: true  # ✅ Provider-specific
          vlan:
            id: "100"
            mtu: 1500
            provider: "physnet1"
```

**Rationale**:
- Generic configs remain provider-agnostic
- Provider-specific optimizations are isolated
- Easier to add new providers without polluting core config

### 3. Provider-Agnostic Core Configuration

**Recommendation**: Define a provider-agnostic infrastructure interface that all providers must implement.

**Core Infrastructure Interface**:
```go
type InfrastructureCore struct {
    Provider string  // openstack, aws, gcp, azure, baremetal, vsphere
    
    // Provider-agnostic networking
    Networking InfrastructureNetworking
    
    // Provider-agnostic compute
    Compute InfrastructureCompute
    
    // Provider-agnostic storage
    Storage InfrastructureStorage
    
    // Provider-specific configuration (polymorphic)
    ProviderConfig ProviderConfig  // interface implemented by each provider
}

type InfrastructureNetworking struct {
    // Network topology
    SubnetNodes         string
    AllocationPoolStart string
    AllocationPoolEnd   string
    
    // High availability
    VRRPIP      string
    VRRPEnabled bool
    
    // Load balancing
    LoadbalancerProvider string  // ovn, metallb, cloud-native
    
    // DNS
    DNSNameservers []string
    DNSZoneName    string
    
    // Time sync
    NTPServers []string
    
    // Security
    K8sAPIPortACL []string
}

type ProviderConfig interface {
    Validate() error
    GetProviderName() string
}

// Provider-specific implementations
type OpenStackProviderConfig struct {
    AuthURL string
    Region  string
    // ... OpenStack-specific fields
    Networking OpenStackNetworking  // OpenStack-specific networking extensions
}

type AWSProviderConfig struct {
    Region string
    VPCID  string
    // ... AWS-specific fields
}
```

**Rationale**:
- Clear contract for what all providers must support
- Provider-specific extensions are isolated
- Easier to validate cross-provider configurations
- Supports multi-cloud scenarios

### 4. Deployment Method Abstraction

**Recommendation**: Separate deployment method configuration from infrastructure configuration.

**Structure**:
```yaml
opencenter:
  infrastructure:
    provider: "openstack"
    networking:
      vrrp_ip: "10.2.128.10"
    cloud:
      openstack:
        # OpenStack-specific infrastructure
  
  deployment:
    method: "kubespray"  # kubespray, talos, eks, gke, aks
    
    kubespray:
      version: "v2.29.1"
      # Kubespray-specific settings
      # References: infrastructure.networking.vrrp_ip
    
    talos:
      enabled: false
      # Talos-specific settings
      # References: infrastructure.networking.vrrp_ip
```

**Rationale**:
- Infrastructure (where to deploy) is separate from deployment method (how to deploy)
- Supports scenarios like "Talos on AWS" or "Kubespray on OpenStack"
- Deployment methods reference infrastructure settings via explicit paths
- Easier to add new deployment methods (e.g., Cluster API, Rancher)

### 5. Shared Resource References

**Recommendation**: Implement explicit reference syntax for shared resources and clarify service organization.

#### 5.1 Configuration References

**Reference Syntax**:
```yaml
# Define once
opencenter:
  infrastructure:
    networking:
      vrrp_ip: "10.2.128.10"

# Reference in multiple places
opencenter:
  cluster:
    kubernetes:
      api_server:
        vip: "${infrastructure.networking.vrrp_ip}"  # Explicit reference
  
  deployment:
    kubespray:
      keepalived:
        virtual_ip: "${infrastructure.networking.vrrp_ip}"  # Same reference
    
    talos:
      control_plane:
        vip: "${infrastructure.networking.vrrp_ip}"  # Same reference
```

**Implementation Options**:

**Option A: Template Resolution** (recommended for YAML)
- Use `${path.to.value}` syntax in YAML
- Resolve references during config loading
- Validate that referenced paths exist

**Option B: Pointer/Reference Fields** (Go structs)
```go
type KubernetesConfig struct {
    APIServerVIP *string  // Pointer to Infrastructure.Networking.VRRPIP
}
```

**Option C: Computed Properties** (runtime resolution)
```go
func (k *KubernetesConfig) GetAPIServerVIP(infra *Infrastructure) string {
    return infra.Networking.VRRPIP
}
```

**Rationale**:
- Eliminates duplication at source
- Single source of truth is enforced
- Validation can ensure references are valid
- Clear dependency graph for configuration

#### 5.2 Services Organization

**Current Issue**: Two separate service collections (`Services` and `ManagedService`) with unclear distinction.

**Recommendation**: Consolidate into single `Services` collection with service metadata indicating management type.

**Option A: Single Collection with Type Field**
```yaml
opencenter:
  services:
    alert-proxy:
      enabled: false
      management_type: "managed"  # managed, self-hosted, external
      # ... service-specific config
    
    loki:
      enabled: false
      management_type: "self-hosted"
      # ... service-specific config
```

**Option B: Nested Structure by Management Type**
```yaml
opencenter:
  services:
    managed:
      alert-proxy:
        enabled: false
        # ... service-specific config
    
    self_hosted:
      loki:
        enabled: false
        # ... service-specific config
    
    external:
      # Services hosted outside cluster but integrated
```

**Option C: Keep Separate but Document Clearly**
```yaml
opencenter:
  # Self-hosted services deployed and managed by opencenter
  services:
    loki:
      enabled: false
  
  # Managed services provided by external vendors/platforms
  # Typically require external credentials and service tokens
  managed_services:
    alert-proxy:
      enabled: false
```

**Recommendation**: Use **Option C** with clear documentation:
- `services` - Self-hosted services deployed in-cluster via GitOps
- `managed_services` - External/vendor-managed services requiring integration credentials
- Add `management_type` field to BaseConfig for future flexibility

**Rationale**:
- Preserves existing structure (minimal migration)
- Clear semantic distinction for users
- Different services may have different lifecycle patterns
- Managed services often require different secret handling

### 6. Provider-Specific Defaults and Regional Configuration

**Problem**: Infrastructure providers and regions have vastly different optimal defaults, but current system uses static defaults.

#### 6.1 Provider-Region Default Registry

**Recommendation**: Implement a provider-region default registry that supplies context-aware defaults.

**Structure**:
```go
// Provider-Region Default Registry
type ProviderDefaults interface {
    GetImageID(osVersion string) string
    GetAvailabilityZones() []string
    GetNTPServers() []string
    GetDNSNameservers() []string
    GetDefaultStorageClass() string
    GetDefaultFlavors() FlavorDefaults
}

type FlavorDefaults struct {
    Bastion string
    Master  string
    Worker  string
}

// OpenStack-specific defaults by region
type OpenStackRegionDefaults struct {
    Region string
    Images map[string]string  // osVersion -> imageID
    AvailabilityZones []string
    NTPServers []string
    DNSNameservers []string
    StorageClasses []string
    Flavors FlavorDefaults
}

// Registry of provider-region defaults
var ProviderRegionDefaults = map[string]map[string]ProviderDefaults{
    "openstack": {
        "sjc3": OpenStackSJC3Defaults,
        "dfw3": OpenStackDFW3Defaults,
        "iad3": OpenStackIAD3Defaults,
    },
    "aws": {
        "us-east-1": AWSUSEast1Defaults,
        "us-west-2": AWSUSWest2Defaults,
        "eu-west-1": AWSEUWest1Defaults,
    },
    "gcp": {
        "us-central1": GCPUSCentral1Defaults,
        "europe-west1": GCPEuropeWest1Defaults,
    },
}
```

**Usage in defaultConfig()**:
```go
func defaultConfig(name string, provider string, region string) Config {
    // Get provider-region specific defaults
    defaults := ProviderRegionDefaults[provider][region]
    
    cfg := Config{
        OpenCenter: SimplifiedOpenCenter{
            Infrastructure: Infrastructure{
                Provider: provider,
                Cloud: CloudConfig{
                    OpenStack: SimplifiedOpenStackCloud{
                        Region: region,
                        ImageID: defaults.GetImageID("24"),  // Dynamic based on region
                        AvailabilityZone: defaults.GetAvailabilityZones()[0],
                        // ... other region-specific defaults
                    },
                },
                Networking: InfrastructureNetworking{
                    NTPServers: defaults.GetNTPServers(),
                    DNSNameservers: defaults.GetDNSNameservers(),
                },
            },
            Storage: StorageConfig{
                DefaultStorageClass: defaults.GetDefaultStorageClass(),
            },
        },
    }
    
    return cfg
}
```

**Configuration File Format**:
```yaml
# Provider-region defaults can be overridden in CLI config
defaults:
  providers:
    openstack:
      regions:
        sjc3:
          images:
            ubuntu-24: "799dcf97-3656-4361-8187-13ab1b295e33"
            ubuntu-22: "a1234567-1234-1234-1234-123456789abc"
          availability_zones: ["az1", "az2", "az3"]
          ntp_servers: ["time.sjc3.rackspace.com", "time2.sjc3.rackspace.com"]
          dns_nameservers: ["8.8.8.8", "8.8.4.4"]
          storage_classes: ["csi-cinder-sc-delete", "csi-cinder-sc-retain"]
          flavors:
            bastion: "gp.0.2.2"
            master: "gp.0.4.8"
            worker: "gp.0.4.16"
        
        dfw3:
          images:
            ubuntu-24: "b9876543-4321-4321-4321-ba9876543210"
          availability_zones: ["az1", "az2"]
          ntp_servers: ["time.dfw3.rackspace.com", "time2.dfw3.rackspace.com"]
          # ... DFW3-specific defaults
    
    aws:
      regions:
        us-east-1:
          images:
            ubuntu-24: "ami-0c55b159cbfafe1f0"
          availability_zones: ["us-east-1a", "us-east-1b", "us-east-1c"]
          # ... AWS-specific defaults
```

**Rationale**:
- Eliminates hardcoded region-specific values
- Supports multi-region deployments
- Allows users to override defaults in CLI config
- Enables provider-specific optimizations
- Reduces configuration errors from wrong image IDs, AZs, etc.

#### 6.2 Default Resolution Order

**Recommendation**: Establish clear precedence for default value resolution.

**Resolution Order** (highest to lowest precedence):
1. **Explicit cluster config** - User-specified values in cluster YAML
2. **CLI config overrides** - User's `~/.config/opencenter/config.yaml` defaults
3. **Provider-region defaults** - Built-in provider-region registry
4. **Provider defaults** - Built-in provider-wide defaults
5. **Global defaults** - Built-in global fallback defaults

**Example Resolution**:
```yaml
# User's cluster config (highest precedence)
opencenter:
  infrastructure:
    cloud:
      openstack:
        image_id: "custom-image-123"  # ✅ Used (explicit)
        availability_zone: ""  # Empty, falls through to next level

# User's CLI config (~/.config/opencenter/config.yaml)
defaults:
  providers:
    openstack:
      regions:
        sjc3:
          availability_zones: ["custom-az1"]  # ✅ Used (CLI override)
          flavors:
            master: ""  # Empty, falls through

# Built-in provider-region defaults
ProviderRegionDefaults["openstack"]["sjc3"]:
  Flavors.Master: "gp.0.4.8"  # ✅ Used (provider-region default)

# Result:
# image_id: "custom-image-123" (from cluster config)
# availability_zone: "custom-az1" (from CLI config)
# flavor_master: "gp.0.4.8" (from provider-region defaults)
```

**Rationale**:
- Clear, predictable behavior
- Users can override at multiple levels
- Supports organization-wide standards (CLI config)
- Supports per-cluster customization (cluster config)

### 7. Service Configuration Polymorphism

**Problem**: Services need provider-specific configuration but lack polymorphic structure.

#### 7.1 Service Provider Adapters

**Recommendation**: Implement provider adapter pattern for services with provider-specific needs.

**Pattern**: Service configurations include a `provider` field that selects the appropriate adapter configuration.

**Example: cert-manager DNS Challenge Providers**

**Current Structure** (insufficient):
```yaml
services:
  cert-manager:
    enabled: true
    email: "admin@example.com"
    letsencrypt_server: "https://acme-v02.api.letsencrypt.org/directory"
    # Missing: DNS provider configuration
```

**Proposed Structure**:
```yaml
services:
  cert-manager:
    enabled: true
    email: "admin@example.com"
    letsencrypt_server: "https://acme-v02.api.letsencrypt.org/directory"
    
    # DNS challenge provider (polymorphic)
    dns_challenge:
      provider: "route53"  # route53, cloudflare, designate, google-cloud-dns
      
      # Provider-specific configuration
      route53:
        region: "us-east-1"
        hosted_zone_id: "Z1234567890ABC"
        # Credentials from secrets.cert_manager.aws_*
      
      cloudflare:
        # Alternative provider config (not used when provider=route53)
        email: "admin@example.com"
        # Credentials from secrets.cert_manager.cloudflare_api_token
      
      designate:
        # OpenStack Designate config (not used when provider=route53)
        auth_url: "${infrastructure.cloud.openstack.auth_url}"
        region: "${infrastructure.cloud.openstack.region}"
```

**Go Struct Definition**:
```go
type CertManagerConfig struct {
    BaseConfig `yaml:",inline"`
    
    Email             string `yaml:"email"`
    LetsEncryptServer string `yaml:"letsencrypt_server"`
    
    // DNS challenge provider configuration
    DNSChallenge DNSChallengeConfig `yaml:"dns_challenge"`
}

type DNSChallengeConfig struct {
    Provider string `yaml:"provider"` // route53, cloudflare, designate, google-cloud-dns
    
    // Provider-specific configs (only one used based on Provider field)
    Route53    *Route53Config    `yaml:"route53,omitempty"`
    CloudFlare *CloudFlareConfig `yaml:"cloudflare,omitempty"`
    Designate  *DesignateConfig  `yaml:"designate,omitempty"`
    GoogleDNS  *GoogleDNSConfig  `yaml:"google_dns,omitempty"`
}

type Route53Config struct {
    Region       string `yaml:"region"`
    HostedZoneID string `yaml:"hosted_zone_id"`
}

type CloudFlareConfig struct {
    Email string `yaml:"email"`
}

type DesignateConfig struct {
    AuthURL string `yaml:"auth_url"`
    Region  string `yaml:"region"`
}
```

**Validation**:
```go
func (c *CertManagerConfig) Validate() error {
    if !c.Enabled {
        return nil
    }
    
    // Validate that selected provider config is present
    switch c.DNSChallenge.Provider {
    case "route53":
        if c.DNSChallenge.Route53 == nil {
            return errors.New("route53 configuration required when provider=route53")
        }
        // Validate Route53-specific fields
    case "cloudflare":
        if c.DNSChallenge.CloudFlare == nil {
            return errors.New("cloudflare configuration required when provider=cloudflare")
        }
        // Validate CloudFlare-specific fields
    case "designate":
        if c.DNSChallenge.Designate == nil {
            return errors.New("designate configuration required when provider=designate")
        }
        // Validate Designate-specific fields
    default:
        return fmt.Errorf("unsupported DNS challenge provider: %s", c.DNSChallenge.Provider)
    }
    
    return nil
}
```

**Rationale**:
- Explicit provider selection
- Type-safe provider-specific configuration
- Validation ensures correct provider config is present
- Clear in YAML which provider is active
- Supports multiple providers without ambiguity

#### 7.2 Infrastructure-Aware Service Defaults

**Recommendation**: Services should automatically select appropriate provider based on infrastructure configuration.

**Pattern**: Service defaults are derived from infrastructure provider when not explicitly specified.

**Example: Automatic Provider Selection**:
```go
func defaultCertManagerConfig(infra Infrastructure) *services.CertManagerConfig {
    cfg := &services.CertManagerConfig{
        BaseConfig: services.BaseConfig{
            Enabled: false,
        },
        Email:             "admin@example.com",
        LetsEncryptServer: "https://acme-v02.api.letsencrypt.org/directory",
        DNSChallenge: services.DNSChallengeConfig{},
    }
    
    // Auto-select DNS provider based on infrastructure
    switch infra.Provider {
    case "aws":
        cfg.DNSChallenge.Provider = "route53"
        cfg.DNSChallenge.Route53 = &services.Route53Config{
            Region: infra.Cloud.AWS.Region,
        }
    
    case "openstack":
        // Check if Designate is enabled
        if infra.Cloud.OpenStack.Networking.UseDesignate {
            cfg.DNSChallenge.Provider = "designate"
            cfg.DNSChallenge.Designate = &services.DesignateConfig{
                AuthURL: infra.Cloud.OpenStack.AuthURL,
                Region:  infra.Cloud.OpenStack.Region,
            }
        } else {
            // Default to CloudFlare for OpenStack without Designate
            cfg.DNSChallenge.Provider = "cloudflare"
            cfg.DNSChallenge.CloudFlare = &services.CloudFlareConfig{}
        }
    
    case "gcp":
        cfg.DNSChallenge.Provider = "google-cloud-dns"
        cfg.DNSChallenge.GoogleDNS = &services.GoogleDNSConfig{
            Project: infra.Cloud.GCP.ProjectID,
        }
    }
    
    return cfg
}
```

**User Override**:
```yaml
# User can override auto-selected provider
opencenter:
  infrastructure:
    provider: "openstack"  # Would normally default to designate or cloudflare
  
  services:
    cert-manager:
      enabled: true
      dns_challenge:
        provider: "cloudflare"  # ✅ User override - use CloudFlare instead
        cloudflare:
          email: "admin@example.com"
```

**Rationale**:
- Reduces configuration burden (smart defaults)
- Infrastructure-aware service configuration
- Users can still override when needed
- Prevents misconfiguration (e.g., Route53 on OpenStack)

#### 7.3 Service Provider Registry

**Recommendation**: Create a registry of supported service providers with validation.

**Structure**:
```go
// Service provider registry
type ServiceProviderRegistry struct {
    providers map[string]map[string]ServiceProviderInfo
}

type ServiceProviderInfo struct {
    Name                string
    SupportedInfra      []string  // Which infrastructure providers support this
    RequiredSecrets     []string  // Which secrets are required
    ConfigValidator     func(any) error
    TemplateGenerator   func(any) (string, error)
}

var ServiceProviders = ServiceProviderRegistry{
    providers: map[string]map[string]ServiceProviderInfo{
        "cert-manager": {
            "route53": {
                Name:           "AWS Route53",
                SupportedInfra: []string{"aws"},
                RequiredSecrets: []string{"cert_manager.aws_access_key", "cert_manager.aws_secret_access_key"},
                ConfigValidator: validateRoute53Config,
            },
            "cloudflare": {
                Name:           "CloudFlare DNS",
                SupportedInfra: []string{"aws", "openstack", "gcp", "azure", "baremetal"},
                RequiredSecrets: []string{"cert_manager.cloudflare_api_token"},
                ConfigValidator: validateCloudFlareConfig,
            },
            "designate": {
                Name:           "OpenStack Designate",
                SupportedInfra: []string{"openstack"},
                RequiredSecrets: []string{}, // Uses OpenStack credentials
                ConfigValidator: validateDesignateConfig,
            },
        },
        "velero": {
            "aws-s3": {
                Name:           "AWS S3",
                SupportedInfra: []string{"aws"},
                RequiredSecrets: []string{"velero.aws_access_key", "velero.aws_secret_access_key"},
            },
            "openstack-swift": {
                Name:           "OpenStack Swift",
                SupportedInfra: []string{"openstack"},
                RequiredSecrets: []string{"velero.swift_password"},
            },
        },
    },
}
```

**Validation with Registry**:
```go
func (c *CertManagerConfig) Validate(infra Infrastructure) error {
    if !c.Enabled {
        return nil
    }
    
    // Check if provider is supported
    providerInfo, exists := ServiceProviders.GetProvider("cert-manager", c.DNSChallenge.Provider)
    if !exists {
        return fmt.Errorf("unsupported DNS challenge provider: %s", c.DNSChallenge.Provider)
    }
    
    // Check if provider is compatible with infrastructure
    if !providerInfo.SupportsInfrastructure(infra.Provider) {
        return fmt.Errorf("DNS provider %s is not supported on infrastructure provider %s", 
            c.DNSChallenge.Provider, infra.Provider)
    }
    
    // Validate provider-specific configuration
    if providerInfo.ConfigValidator != nil {
        if err := providerInfo.ConfigValidator(c.DNSChallenge); err != nil {
            return fmt.Errorf("invalid %s configuration: %w", c.DNSChallenge.Provider, err)
        }
    }
    
    return nil
}
```

**Rationale**:
- Centralized provider compatibility information
- Prevents invalid provider-infrastructure combinations
- Clear documentation of required secrets per provider
- Extensible for new providers
- Enables better error messages

#### 7.4 Example: Complete Service Provider Configuration

**cert-manager with Multiple Provider Options**:
```yaml
opencenter:
  infrastructure:
    provider: "aws"
    cloud:
      aws:
        region: "us-east-1"
  
  services:
    cert-manager:
      enabled: true
      email: "admin@example.com"
      letsencrypt_server: "https://acme-v02.api.letsencrypt.org/directory"
      
      dns_challenge:
        provider: "route53"  # Auto-selected based on infrastructure.provider=aws
        
        route53:
          region: "us-east-1"  # Auto-populated from infrastructure
          hosted_zone_id: "Z1234567890ABC"

secrets:
  cert_manager:
    aws_access_key: "AKIAIOSFODNN7EXAMPLE"
    aws_secret_access_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
```

**velero with Provider-Specific Storage**:
```yaml
opencenter:
  infrastructure:
    provider: "openstack"
  
  services:
    velero:
      enabled: true
      
      storage:
        provider: "openstack-swift"  # Auto-selected based on infrastructure
        
        openstack_swift:
          container: "prod-cluster-backups"
          auth_url: "${infrastructure.cloud.openstack.auth_url}"
          region: "${infrastructure.cloud.openstack.region}"
        
        # Alternative AWS S3 config (not used)
        aws_s3:
          bucket: "prod-cluster-backups"
          region: "us-east-1"

secrets:
  velero:
    swift_password: "secret-password"
```

**Rationale**:
- Clear provider selection
- Infrastructure-aware defaults
- Type-safe configuration
- Validation prevents misconfigurations
- Supports multi-provider scenarios

## Future State Architecture

### Proposed Structure

```yaml
schema_version: "v2.0.0"

opencenter:
  # Cluster Identity & Metadata
  meta:
    name: "prod-cluster"
    organization: "acme-corp"
    environment: "production"
    region: "us-east-1"
  
  # Cluster-Level Configuration
  cluster:
    domain: "k8s.acme.com"
    admin_email: "admin@acme.com"
    
    # Kubernetes Configuration (deployment-agnostic)
    kubernetes:
      version: "1.33.5"
      api_port: 443
      
      # Kubernetes-specific networking (CNI-managed)
      networking:
        subnet_pods: "10.42.0.0/16"
        subnet_services: "10.43.0.0/16"
        network_plugin: "calico"
      
      # Kubernetes security policies
      security:
        k8s_hardening: true
        pod_security_exemptions: []
  
  # Infrastructure Configuration (provider-agnostic core + provider-specific)
  infrastructure:
    provider: "openstack"  # openstack, aws, gcp, azure, baremetal, vsphere
    
    # Provider-Agnostic Networking
    networking:
      # Physical network topology
      subnet_nodes: "10.2.128.0/22"
      allocation_pool_start: "10.2.128.100"
      allocation_pool_end: "10.2.128.200"
      
      # High availability
      vrrp_ip: "10.2.128.10"
      vrrp_enabled: true
      
      # Load balancing (cluster-wide)
      loadbalancer_provider: "ovn"
      
      # DNS infrastructure
      dns_nameservers: ["8.8.8.8"]
      dns_zone_name: "k8s.acme.com"
      
      # Time synchronization
      ntp_servers: ["time.acme.com"]
      
      # Security
      k8s_api_port_acl: ["10.0.0.0/8"]
    
    # Provider-Agnostic Compute
    compute:
      ssh_user: "ubuntu"
      ssh_authorized_keys: []
      os_version: "24"
      
      # Node pools (provider-agnostic)
      control_plane:
        count: 3
        flavor: "gp.0.4.8"  # Provider translates to instance type
      
      workers:
        count: 2
        flavor: "gp.0.4.16"
    
    # Provider-Specific Configuration
    cloud:
      openstack:
        auth_url: "https://identity.example.com/v3"
        region: "RegionOne"
        tenant_name: "production"
        
        # OpenStack-specific networking extensions
        networking:
          use_octavia: true
          use_designate: true
          floating_ip_pool: "PUBLICNET"
          vlan:
            id: "100"
            mtu: 1500
            provider: "physnet1"
        
        # OpenStack-specific compute
        compute:
          availability_zone: "az1"
          server_group_affinity: ["anti-affinity"]
      
      aws:
        # AWS-specific configuration (when provider: "aws")
        region: "us-east-1"
        vpc_id: "vpc-12345"
        private_subnets: []
        public_subnets: []
  
  # Deployment Method Configuration
  deployment:
    method: "kubespray"  # kubespray, talos, eks, gke, aks, cluster-api
    
    kubespray:
      version: "v2.29.1"
      # Kubespray-specific settings
      # Implicitly references: infrastructure.networking.vrrp_ip
    
    talos:
      enabled: false
      version: "v1.8.0"
      # Talos-specific settings
      # Implicitly references: infrastructure.networking.vrrp_ip
  
  # Services (unchanged)
  services:
    loki:
      enabled: true
      storage_type: "swift"
      bucket_name: "prod-cluster-loki"
      # ... service-specific config
    
    tempo:
      enabled: false
      storage_type: "s3"
      # ... service-specific config
    
    kube-prometheus-stack:
      enabled: true
      prometheus_volume_size: 50
      # ... service-specific config
  
  # Managed services (vendor/platform-managed)
  managed_services:
    alert-proxy:
      enabled: false
      # Requires external service credentials
      # ... service-specific config

# OpenTofu Configuration (unchanged)
opentofu:
  enabled: true
  backend:
    type: "s3"

# Secrets (unchanged)
secrets:
  sops_age_key_file: ""
  
  # Service-specific secrets
  loki:
    swift_password: ""
  
  tempo:
    access_key: ""
    secret_key: ""
  
  alert_proxy:
    core_device_id: ""
    account_service_token: ""
    core_account_number: ""
  # ...
```

### Configuration Resolution Order

1. **Load Base Config**: Read YAML file
2. **Resolve References**: Replace `${path.to.value}` with actual values
3. **Apply Provider Defaults**: Merge provider-specific defaults
4. **Apply Deployment Defaults**: Merge deployment-method defaults
5. **Validate Hierarchy**: Ensure required fields at each level
6. **Validate Cross-References**: Ensure referenced paths exist
7. **Validate Provider Constraints**: Provider-specific validation
8. **Validate Deployment Constraints**: Deployment-method validation

### Migration Path

#### Phase 1: Add New Fields (Backward Compatible)

1. Add `Infrastructure.Networking` struct with all networking fields
2. Keep existing fields in `Cluster.Networking` (deprecated)
3. Config loader populates both old and new fields
4. Validation warns about deprecated fields

#### Phase 2: Update Code to Use New Fields

1. Update all code to read from `Infrastructure.Networking`
2. Remove reads from deprecated locations
3. Keep deprecated fields in structs (for backward compat)

#### Phase 3: Migration Tool

1. Provide `opencenter cluster migrate-config` command
2. Reads old config format
3. Writes new config format
4. Validates both formats produce same result

#### Phase 4: Deprecation

1. Mark old fields as deprecated in schema
2. Validation errors (not warnings) for deprecated fields
3. Documentation updated to new format

#### Phase 5: Removal

1. Remove deprecated fields from structs
2. Remove backward compatibility code
3. Bump schema version to v2.0.0

## Implementation Priorities

### Priority 1: Critical Duplication (Immediate)

**Target**: Eliminate VRRP IP duplication

**Tasks**:
1. Add `Infrastructure.Networking.VRRPIP` field
2. Update Kubespray templates to read from new location
3. Update Talos templates to read from new location
4. Update OpenStack provider to read from new location
5. Deprecate old fields
6. Add migration guide

**Impact**: High - Prevents configuration inconsistencies in HA setups

### Priority 2: Networking Consolidation (Short-term)

**Target**: Consolidate all infrastructure networking into `Infrastructure.Networking`

**Tasks**:
1. Move `SubnetNodes`, `AllocationPool*` to `Infrastructure.Networking`
2. Move `DNSNameservers`, `NTPServers` to `Infrastructure.Networking`
3. Move `LoadbalancerProvider` to `Infrastructure.Networking`
4. Update all references
5. Add validation for new structure

**Impact**: Medium - Improves configuration clarity

### Priority 3: Provider Isolation (Medium-term)

**Target**: Remove provider-specific flags from generic configs

**Tasks**:
1. Move `UseOctavia`, `UseDesignate` to `OpenStackProviderConfig`
2. Move `VLAN` to provider-specific networking
3. Create provider interface
4. Implement provider-specific validation

**Impact**: Medium - Enables cleaner multi-provider support

### Priority 4: Deployment Abstraction (Long-term)

**Target**: Separate deployment method from infrastructure

**Tasks**:
1. Create `Deployment` top-level section
2. Move Kubespray config to `Deployment.Kubespray`
3. Move Talos config to `Deployment.Talos`
4. Implement reference resolution
5. Support multiple deployment methods

**Impact**: Low - Enables advanced scenarios (multi-deployment)

### Priority 5: Reference System (Long-term)

**Target**: Implement explicit reference syntax

**Tasks**:
1. Design reference syntax (`${path}` or alternative)
2. Implement reference resolver
3. Add validation for references
4. Update documentation
5. Migrate existing configs

**Impact**: Low - Quality of life improvement

### Priority 6: Services Organization Clarity (Documentation)

**Target**: Document services vs managed_services distinction

**Tasks**:
1. Add documentation explaining services vs managed_services
2. Add `management_type` field to BaseConfig (optional, for future use)
3. Update service registration examples
4. Document when to use each collection
5. Add validation to prevent services in wrong collection

**Impact**: Low - Improves clarity for service developers

### Priority 7: Provider-Region Defaults Registry (Medium-term)

**Target**: Implement provider-region default registry for context-aware defaults

**Tasks**:
1. Design ProviderDefaults interface
2. Create provider-region default registry structure
3. Implement OpenStack region defaults (sjc3, dfw3, iad3)
4. Implement AWS region defaults (us-east-1, us-west-2, eu-west-1)
5. Update defaultConfig() to use registry
6. Add CLI config support for default overrides
7. Document default resolution order
8. Add validation for provider-region combinations

**Impact**: High - Eliminates hardcoded region-specific values, reduces configuration errors

### Priority 8: Service Provider Polymorphism (Long-term)

**Target**: Implement provider adapter pattern for services with provider-specific needs

**Tasks**:
1. Design service provider adapter pattern
2. Implement cert-manager DNS challenge providers (Route53, CloudFlare, Designate)
3. Implement velero storage providers (S3, Swift, GCS)
4. Create service provider registry
5. Add infrastructure-aware service defaults
6. Implement provider compatibility validation
7. Update service templates to use provider-specific configs
8. Document service provider configuration patterns

**Impact**: Medium - Enables proper multi-cloud service configuration, prevents misconfigurations

---

## Conclusion

The proposed hierarchical configuration model addresses the current duplication and ambiguity issues by establishing clear ownership of settings at appropriate levels. The migration path ensures backward compatibility while moving toward a cleaner, more maintainable structure that supports multi-provider and multi-deployment scenarios.

**Immediate Action**: Start with Priority 1 (VRRP IP consolidation) as it has the highest impact on configuration correctness and is the most straightforward to implement.

**Long-term Vision**: A configuration system where:
- Each setting has exactly one authoritative location
- Provider-specific settings are isolated from generic configs
- Deployment methods reference infrastructure settings explicitly
- Multi-cloud and multi-deployment scenarios are first-class citizens
- Services are clearly organized by management type (self-hosted vs managed)
- Service-specific secrets are properly scoped and validated
