---
title: Provider Comparison and Selection Guide
doc_type: explanation
category: architecture
tags: [providers, infrastructure, openstack, aws, kind, talos, comparison]
related:
  - ../how-to/cluster-init.md
  - ../reference/configuration-schema.md
  - ./configuration-system.md
---

# Provider Comparison and Selection Guide


## Table of Contents

- [Overview](#overview)
- [Supported Providers](#supported-providers)
- [Provider Comparison Matrix](#provider-comparison-matrix)
- [Decision Framework](#decision-framework)
- [Provider-Specific Considerations](#provider-specific-considerations)
- [Migration Between Providers](#migration-between-providers)
- [Conclusion](#conclusion)
- [Related Documentation](#related-documentation)
This document explains the different infrastructure providers supported by openCenter, their characteristics, trade-offs, and guidance for choosing the right provider for your use case.

## Overview

openCenter supports multiple infrastructure providers to accommodate different deployment scenarios, from local development to production cloud deployments. Each provider has distinct characteristics, capabilities, and operational considerations.

## Supported Providers

### OpenStack

**Primary Use Case**: Production Kubernetes clusters on OpenStack-based clouds (Rackspace, private clouds)

**Architecture**:
- Uses OpenTofu/Terraform for infrastructure provisioning
- Leverages Kubespray (Ansible) for Kubernetes installation
- Integrates with OpenStack services: Nova (compute), Neutron (networking), Cinder (storage), Barbican (secrets)
- Supports OpenStack Cloud Controller Manager (CCM) and Container Storage Interface (CSI)

**Key Features**:
- **Mature Integration**: Full support for OpenStack networking, storage, and load balancing
- **Barbican Secrets**: Native integration with OpenStack Barbican for secrets management
- **Flexible Networking**: Supports multiple network plugins (Calico, Cilium, Kube-OVN)
- **Production Ready**: Battle-tested for enterprise workloads
- **High Availability**: Built-in support for multi-master clusters with anti-affinity rules

**Configuration Requirements**:
```yaml
opencenter:
  infrastructure:
    provider: openstack
    cloud:
      openstack:
        auth_url: "https://identity.example.com/v3"
        region: "RegionOne"
        application_credential_id: "..."
        application_credential_secret: "..."
        tenant_name: "my-project"
```

**When to Choose OpenStack**:
- Deploying on Rackspace or private OpenStack clouds
- Need enterprise-grade storage and networking features
- Require integration with existing OpenStack infrastructure
- Want Barbican-based secrets management
- Production workloads requiring high availability

**Limitations**:
- Requires OpenStack cloud access
- More complex setup than managed Kubernetes services
- Requires understanding of OpenStack concepts (projects, networks, flavors)

---

### AWS

**Primary Use Case**: Production Kubernetes clusters on Amazon Web Services

**Architecture**:
- Uses OpenTofu/Terraform for infrastructure provisioning
- Leverages Kubespray (Ansible) for Kubernetes installation
- Integrates with AWS services: EC2, VPC, EBS, Route53
- Supports AWS Cloud Controller Manager and EBS CSI driver

**Key Features**:
- **AWS Native**: Deep integration with AWS services
- **Scalability**: Leverage AWS's global infrastructure
- **Managed Services**: Optional integration with EKS for control plane
- **Cost Optimization**: Support for spot instances and auto-scaling
- **DNS Integration**: Route53 for cluster DNS management

**Configuration Requirements**:
```yaml
opencenter:
  infrastructure:
    provider: aws
    cloud:
      aws:
        profile: "default"
        region: "us-east-1"
        vpc_id: "vpc-12345678"
        private_subnets: ["subnet-abc", "subnet-def"]
        public_subnets: ["subnet-ghi", "subnet-jkl"]
```

**When to Choose AWS**:
- Deploying on Amazon Web Services
- Need AWS-specific integrations (EBS, ELB, Route53)
- Want to leverage AWS's global infrastructure
- Require integration with other AWS services
- Production workloads on AWS

**Limitations**:
- AWS-specific, not portable to other clouds
- Costs can escalate with resource usage
- Requires AWS account and IAM permissions
- Network configuration more complex than OpenStack

---

### Kind (Kubernetes in Docker)

**Primary Use Case**: Local development, testing, and CI/CD pipelines

**Architecture**:
- Runs Kubernetes nodes as Docker containers
- Single-node or multi-node clusters on local machine
- No cloud provider integration
- Minimal resource requirements

**Key Features**:
- **Fast Setup**: Cluster ready in seconds
- **Lightweight**: Runs on developer laptops
- **Disposable**: Easy to create and destroy clusters
- **CI/CD Friendly**: Perfect for automated testing
- **No Cloud Costs**: Runs entirely locally

**Configuration Requirements**:
```yaml
opencenter:
  infrastructure:
    provider: kind
  cluster:
    kubernetes:
      master_count: 1
      worker_count: 2
```

**When to Choose Kind**:
- Local development and testing
- CI/CD pipeline testing
- Learning Kubernetes and openCenter
- Validating configurations before production deployment
- No cloud access available

**Limitations**:
- Not suitable for production workloads
- Limited to single machine resources
- No cloud provider features (load balancers, persistent volumes)
- Networking differs from production environments
- No high availability

---

### Talos Linux

**Primary Use Case**: Secure, immutable Kubernetes infrastructure with minimal attack surface

**Architecture**:
- API-driven Linux distribution designed for Kubernetes
- Immutable infrastructure (no SSH, no shell access)
- Uses Pulumi for infrastructure provisioning
- Integrates with OpenStack or bare metal
- Built-in disk encryption and secure boot

**Key Features**:
- **Security First**: Minimal attack surface, no SSH access
- **Immutable**: Read-only root filesystem, declarative configuration
- **API-Driven**: All operations via gRPC API
- **Disk Encryption**: Built-in LUKS encryption with Barbican integration
- **Secure Boot**: TPM and vTPM support for verified boot
- **KubePrism**: Built-in load balancer for API server HA

**Configuration Requirements**:
```yaml
opencenter:
  infrastructure:
    provider: openstack  # or baremetal
  talos:
    enabled: true
    version: "v1.8.0"
    machine_config:
      disk_encryption: true
      app_armor_enabled: true
      seccomp_enabled: true
    security_config:
      vtpm_enabled: true
      image_verification: true
      mfa_required: true
```

**When to Choose Talos**:
- Maximum security requirements (defense, finance, healthcare)
- Compliance requirements (PCI-DSS, HIPAA, FedRAMP)
- Immutable infrastructure preferred
- Want to eliminate SSH attack surface
- Need disk encryption and secure boot
- Prefer declarative, API-driven operations

**Limitations**:
- Steeper learning curve (no SSH access)
- Debugging requires different approach (API-based)
- Fewer community resources than traditional distributions
- Requires Pulumi (different from OpenTofu workflow)
- More complex initial setup

---

### Bare Metal

**Primary Use Case**: On-premises deployments, edge computing, custom hardware

**Architecture**:
- Pre-provisioned physical or virtual machines
- Uses Kubespray (Ansible) for Kubernetes installation
- No cloud provider integration
- Manual or PXE-based node provisioning

**Key Features**:
- **Full Control**: Complete control over hardware and network
- **Cost Effective**: No cloud provider fees
- **Performance**: Direct hardware access, no virtualization overhead
- **Compliance**: Keep data on-premises for regulatory requirements
- **Custom Hardware**: Support for specialized hardware (GPUs, FPGAs)

**Configuration Requirements**:
```yaml
opencenter:
  infrastructure:
    provider: baremetal
  cluster:
    kubernetes:
      master_nodes:
        - id: "master-1"
          name: "master-1.example.com"
          access_ip_v4: "192.168.1.10"
      worker_nodes:
        - id: "worker-1"
          name: "worker-1.example.com"
          access_ip_v4: "192.168.1.20"
```

**When to Choose Bare Metal**:
- On-premises deployment requirements
- Edge computing scenarios
- Custom or specialized hardware
- Regulatory requirements for data locality
- Want to avoid cloud provider costs
- Need maximum performance

**Limitations**:
- Manual hardware provisioning required
- No cloud provider features (auto-scaling, managed load balancers)
- Higher operational overhead
- Requires physical infrastructure management
- Network configuration more complex

---

## Provider Comparison Matrix

| Feature | OpenStack | AWS | Kind | Talos | Bare Metal |
|---------|-----------|-----|------|-------|------------|
| **Production Ready** | ✅ Yes | ✅ Yes | ❌ No | ✅ Yes | ✅ Yes |
| **High Availability** | ✅ Yes | ✅ Yes | ❌ No | ✅ Yes | ✅ Yes |
| **Auto-Scaling** | ⚠️ Limited | ✅ Yes | ❌ No | ⚠️ Limited | ❌ No |
| **Load Balancers** | ✅ Native | ✅ Native | ❌ No | ✅ KubePrism | ⚠️ MetalLB |
| **Persistent Storage** | ✅ Cinder | ✅ EBS | ⚠️ Local | ✅ Cinder | ⚠️ Local/NFS |
| **Secrets Management** | ✅ Barbican | ✅ KMS | ❌ No | ✅ Barbican | ⚠️ External |
| **Setup Time** | ⏱️ 30-60 min | ⏱️ 30-60 min | ⏱️ 2-5 min | ⏱️ 45-90 min | ⏱️ 60+ min |
| **Cost** | 💰 Cloud fees | 💰 Cloud fees | 🆓 Free | 💰 Cloud fees | 💰 Hardware |
| **Security** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| **Complexity** | 🔧 Medium | 🔧 Medium | 🔧 Low | 🔧 High | 🔧 High |

---

## Decision Framework

### Choose OpenStack if:
- ✅ You're deploying on Rackspace or private OpenStack cloud
- ✅ You need Barbican secrets management
- ✅ You want mature, production-ready Kubernetes
- ✅ You need flexible networking options
- ✅ You require enterprise storage features

### Choose AWS if:
- ✅ You're deploying on Amazon Web Services
- ✅ You need AWS service integrations
- ✅ You want global infrastructure reach
- ✅ You need auto-scaling capabilities
- ✅ You prefer AWS-native tooling

### Choose Kind if:
- ✅ You're developing or testing locally
- ✅ You need fast, disposable clusters
- ✅ You're running CI/CD pipelines
- ✅ You're learning Kubernetes
- ✅ You don't need production features

### Choose Talos if:
- ✅ Security is your top priority
- ✅ You need compliance certifications
- ✅ You want immutable infrastructure
- ✅ You prefer API-driven operations
- ✅ You need disk encryption and secure boot

### Choose Bare Metal if:
- ✅ You have on-premises requirements
- ✅ You need custom hardware support
- ✅ You want to avoid cloud costs
- ✅ You have regulatory data locality requirements
- ✅ You need maximum performance

---

## Provider-Specific Considerations

### OpenStack Considerations

**Networking**:
- Requires understanding of Neutron networking concepts
- Must configure floating IP pools, networks, and subnets
- Supports multiple network plugins (Calico, Cilium, Kube-OVN)

**Storage**:
- Cinder volumes for persistent storage
- Boot-from-volume support for worker nodes
- Multiple volume types (HA-Standard, HA-Performance)

**Authentication**:
- Application credentials recommended over username/password
- Supports Keystone v3 authentication
- Domain-scoped or project-scoped credentials

**Preflight Checks**:
```go
// From internal/cloud/openstack/preflight.go
func PreflightOpenStack(authURL string) []string {
    var warnings []string
    // Check presence of openstack CLI
    if _, err := exec.LookPath("openstack"); err != nil {
        warnings = append(warnings, "openstack CLI not found")
    }
    // Check auth URL configured
    if authURL == "" {
        warnings = append(warnings, "auth_url is empty")
    }
    return warnings
}
```

### AWS Considerations

**Networking**:
- Requires VPC, subnets, and security groups
- Must configure public and private subnets
- Route53 for DNS management

**IAM Permissions**:
- Requires IAM role or access keys
- Needs permissions for EC2, VPC, EBS, Route53
- Consider using IAM roles for EC2 instances

**Cost Management**:
- Monitor EC2 instance costs
- Consider spot instances for worker nodes
- Use auto-scaling to optimize costs

### Talos Considerations

**Operational Model**:
- No SSH access (by design)
- All operations via `talosctl` CLI
- API-driven configuration management

**Security Features**:
- Disk encryption with LUKS
- Barbican integration for key management
- TPM/vTPM support for secure boot
- Image verification with signatures

**Learning Curve**:
- Different debugging approach (no shell access)
- Requires understanding of Talos API
- Declarative configuration model

---

## Migration Between Providers

openCenter configurations are designed to be provider-agnostic where possible, but migration between providers requires careful planning:

### Configuration Portability

**Portable Elements**:
- Kubernetes version and configuration
- Service definitions (cert-manager, monitoring, etc.)
- GitOps repository structure
- SOPS-encrypted secrets

**Provider-Specific Elements**:
- Cloud provider configuration (auth, networking)
- Storage classes and persistent volumes
- Load balancer configuration
- DNS and ingress setup

### Migration Strategy

1. **Export Configuration**: Save current cluster configuration
2. **Update Provider Settings**: Modify `infrastructure.provider` and cloud-specific settings
3. **Validate Configuration**: Run `openCenter cluster validate`
4. **Test in Development**: Deploy to Kind or test environment first
5. **Migrate Workloads**: Use Velero or manual migration
6. **Update DNS**: Point DNS to new cluster
7. **Decommission Old Cluster**: After validation

---

## Conclusion

Provider selection is a critical architectural decision that impacts security, operations, cost, and capabilities. Consider your requirements carefully:

- **Production workloads**: OpenStack, AWS, or Talos
- **Development/testing**: Kind
- **Maximum security**: Talos
- **On-premises**: Bare Metal or Talos
- **Cloud-native**: OpenStack or AWS

The openCenter CLI abstracts many provider differences, but understanding each provider's characteristics helps you make informed decisions and operate clusters effectively.

---

## Related Documentation

- [How-To: Initialize a Cluster](../how-to/cluster-init.md)
- [Reference: Configuration Schema](../reference/configuration-schema.md)
- [Explanation: Configuration System](./configuration-system.md)
- [How-To: Validate Configuration](../how-to/cluster-validate.md)
