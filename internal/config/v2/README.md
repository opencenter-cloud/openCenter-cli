# V2 Configuration Package

This package implements the v2 cluster configuration schema for opencenter-cli.

## Overview

The v2 schema redesigns opencenter-cli's configuration system to eliminate duplication, establish clear ownership hierarchies, isolate provider-specific settings, and support advanced deployment methods like Kamaji hosted control planes.

## Key Design Principles

1. **Single Source of Truth**: Each setting defined exactly once at the appropriate hierarchy level
2. **Provider Isolation**: Provider-specific settings isolated under `infrastructure.cloud.<provider>`
3. **Deployment Abstraction**: Deployment method (how) separated from infrastructure provider (where)
4. **Reference Resolution**: Explicit `${path.to.value}` syntax for shared resources
5. **Context-Aware Defaults**: Provider-region registry supplies intelligent defaults
6. **Backward Compatibility**: v1 and v2 coexist during migration period

## Package Structure

### Core Configuration (`config.go`)

- `Config`: Root v2 configuration structure
- `OpenCenterConfig`: Main opencenter configuration with five domains
- `MetaConfig`: Cluster identity and organizational context
- `SecretsConfig`: Secrets configuration
- `GitOpsConfig`: GitOps repository configuration
- `OpenTofuConfig`: OpenTofu backend configuration

### Infrastructure Domain (`infrastructure.go`)

- `InfrastructureConfig`: Provider-agnostic infrastructure with provider-specific extensions
- `NetworkingConfig`: Infrastructure networking (subnet_nodes, VRRP IP, DNS, NTP)
- `ComputeConfig`: Compute resources (flavors, node counts, worker pools)
- `StorageConfig`: Storage configuration (boot volumes, additional devices)
- `CloudConfig`: Polymorphic provider-specific configuration
- Provider-specific configs: `OpenStackCloudConfig`, `AWSCloudConfig`, `GCPCloudConfig`, `AzureCloudConfig`, `VMwareCloudConfig`

### Cluster Domain (`cluster.go`)

- `ClusterConfig`: Kubernetes-specific configuration independent of infrastructure
- `KubernetesConfig`: Kubernetes cluster configuration
- `NetworkPluginConfig`: CNI plugin configuration (Calico, Cilium, Kube-OVN)
- `StoragePluginConfig`: CSI plugin configuration (vSphere CSI, Cinder CSI, AWS EBS CSI, etc.)
- `KubernetesSecurityConfig`: Kubernetes security configuration
- `OIDCConfig`: OIDC authentication configuration

### Deployment Domain (`deployment.go`)

- `DeploymentConfig`: Deployment method configuration
- `KubesprayConfig`: Kubespray deployment configuration
- `TalosConfig`: Talos Linux deployment configuration
- `KamajiConfig`: Kamaji hosted control plane configuration
- `KamajiControlPlane`: Kamaji control plane configuration
- `ClusterAPIConfig`: Cluster API configuration
- `KamajiWorkerPool`: Kamaji worker pool configuration with mixed OS support

### Services (`services.go`)

- `ServiceMap`: Polymorphic map of service configurations
- `BaseServiceConfig`: Common fields shared by all services
- Custom YAML marshaling/unmarshaling using service registry

### Provider Validation (`provider.go`)

- `Provider` interface: Provider-specific validation
- `OpenStackProvider`: OpenStack-specific validation
- `AWSProvider`: AWS-specific validation
- `GCPProvider`: GCP-specific validation
- `AzureProvider`: Azure-specific validation

### Deployment Validation (`deployment_validator.go`)

- `DeploymentMethod` interface: Deployment-method-specific validation
- `KubesprayDeployment`: Kubespray deployment validation
- `TalosDeployment`: Talos deployment validation
- `KamajiDeployment`: Kamaji deployment validation
- `ValidateKamajiControlPlane`: Kamaji control plane validation
- `ValidateKamajiWorkerPool`: Kamaji worker pool validation
- `ValidateClusterAPIProviders`: Cluster API provider validation

## Property-Based Tests

The package includes comprehensive property-based tests using gopter:

### Property 1: Configuration Structure Invariants (`config_property_test.go`)

Validates that structural invariants hold for all valid v2 configurations:
- VRRP IP only in `infrastructure.networking.vrrp_ip`
- Provider-specific settings only in matching cloud section
- Infrastructure networking fields only in `infrastructure.networking`
- Compute configuration only in `infrastructure.compute`
- Storage configuration only in `infrastructure.storage`

**Validates: Requirements 1.1, 1.2, 2.1, 3.1, 4.1**

### Property 8: Kamaji Deployment Constraints (`deployment_property_test.go`)

Validates Kamaji deployment constraints:
- `master_count` must be zero
- `vrrp_enabled` must be false
- `kube_vip_enabled` must be false
- Control plane replicas must be odd (1, 3, 5, 7)
- At least one worker pool must be defined
- Bootstrap provider must match OS (ubuntu/windows→kubeadm, talos→talos)
- Talos worker pools require `talos_version`
- Autoscaling constraints are valid

**Validates: Requirements 10.2, 10.3, 10.8, 10.10, 10.11, 10.12**

### Property 12: Provider-Deployment Compatibility (`deployment_property_test.go`)

Validates provider-deployment compatibility:
- Valid combinations are accepted (OpenStack+Kubespray, AWS+EKS, etc.)
- Invalid combinations are rejected
- Kubespray supports all providers
- Talos does not support baremetal
- Kamaji does not support baremetal
- Deployment methods requiring masters reject `master_count=0`

**Validates: Requirements 5.7**

### Property 13: Multiple Provider Section Rejection (`provider_property_test.go`)

Validates that only one provider section can be populated:
- Single provider section is valid
- Multiple provider sections are rejected
- Provider mismatch is rejected

**Validates: Requirements 4.7**

## Running Tests

```bash
# Run all v2 tests
go test ./internal/config/v2/... -v

# Run specific property test
go test ./internal/config/v2/... -v -run TestProperty_ConfigurationStructureInvariants

# Run all property tests
go test ./internal/config/v2/... -v -run Property
```

## Usage Example

```go
package main

import (
    "github.com/rackerlabs/opencenter-cli/internal/config/v2"
)

func main() {
    cfg := &v2.Config{
        SchemaVersion: "2.0",
        OpenCenter: v2.OpenCenterConfig{
            Meta: v2.MetaConfig{
                Name:         "my-cluster",
                Organization: "my-org",
                Env:          "production",
                Region:       "sjc3",
            },
            Cluster: v2.ClusterConfig{
                ClusterName: "my-cluster",
                BaseDomain:  "example.com",
                ClusterFQDN: "my-cluster.example.com",
                AdminEmail:  "admin@example.com",
                Kubernetes: v2.KubernetesConfig{
                    Version:        "1.28.0",
                    APIPort:        6443,
                    SubnetPods:     "10.233.64.0/18",
                    SubnetServices: "10.233.0.0/18",
                },
            },
            Infrastructure: v2.InfrastructureConfig{
                Provider: "openstack",
                // ... infrastructure configuration
            },
        },
    }

    // Validate provider configuration
    provider, err := v2.GetProvider(cfg.OpenCenter.Infrastructure.Provider)
    if err != nil {
        panic(err)
    }
    
    if err := provider.ValidateConfig(&cfg.OpenCenter.Infrastructure); err != nil {
        panic(err)
    }
}
```

## Next Steps

Phase 4 (Intelligence) will implement:
- Reference resolution system
- Service provider polymorphism
- Service dependency validation
- Required secrets validation

Phase 5 (Bridge) will implement:
- v1 to v2 migration tooling
- Schema version detection
- Backward compatibility support
