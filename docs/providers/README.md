# Provider Documentation


## Table of Contents

- [Purpose](#purpose)
- [Supported Providers](#supported-providers)
- [Provider Comparison](#provider-comparison)
- [Provider Selection Guide](#provider-selection-guide)
- [Provider Documentation Structure](#provider-documentation-structure)
- [Available Documentation](#available-documentation)
- [Common Provider Concepts](#common-provider-concepts)
- [Provider-Specific Configuration](#provider-specific-configuration)
- [Migration Between Providers](#migration-between-providers)
- [Provider Development](#provider-development)
- [Getting Help](#getting-help)
- [Related Documentation](#related-documentation)
- [External Resources](#external-resources)
**doc_type: reference**

This directory contains provider-specific documentation for openCenter. Each provider has its own setup requirements, configuration options, and operational characteristics.

## Purpose

Provider documentation explains how openCenter integrates with different infrastructure platforms. Use this section to understand provider capabilities, choose the right provider for your use case, and configure provider-specific settings.

## Supported Providers

### Production-Ready Providers

**OpenStack**
- **Status**: Production-ready
- **Maturity**: Stable
- **Use Case**: Private cloud deployments, on-premises infrastructure
- **Documentation**: [openstack/](openstack/)
- **Provisioning**: Terraform/OpenTofu
- **Kubernetes**: Kubespray (Ansible-based)
- **Networking**: Calico CNI, Neutron integration
- **HA Support**: VRRP or Octavia load balancer

**Talos Linux**
- **Status**: Production-ready
- **Maturity**: Stable
- **Use Case**: Immutable infrastructure, security-focused deployments
- **Documentation**: [talos/](talos/)
- **Provisioning**: Pulumi
- **Kubernetes**: Native Talos control plane
- **Networking**: Configurable CNI (Cilium, Calico, Flannel)
- **HA Support**: Built-in control plane HA

### Development Providers

**Kind (Kubernetes in Docker)**
- **Status**: Development only
- **Maturity**: Stable for local testing
- **Use Case**: Local development, CI/CD testing, quick prototyping
- **Documentation**: [kind/](kind/) (planned)
- **Provisioning**: Docker containers
- **Kubernetes**: Kind cluster
- **Networking**: Docker bridge networking
- **HA Support**: Not applicable (single-node or multi-node simulation)

### In Development

**AWS**
- **Status**: In development
- **Maturity**: Alpha
- **Use Case**: Public cloud deployments, AWS-native services
- **Documentation**: [aws/](aws/) (planned)
- **Provisioning**: Terraform/OpenTofu (planned)
- **Kubernetes**: EKS or self-managed (TBD)
- **Networking**: VPC, AWS CNI
- **HA Support**: Multi-AZ deployment

### Legacy Support

**Kubespray**
- **Status**: Legacy (maintenance mode)
- **Maturity**: Stable but deprecated
- **Use Case**: Existing Kubespray deployments, migration path
- **Documentation**: [kubespray/](kubespray/)
- **Provisioning**: Terraform + Ansible
- **Kubernetes**: Kubespray playbooks
- **Networking**: Calico, Cilium, or other CNI
- **HA Support**: VRRP or load balancer

**Note**: New deployments should use Talos or OpenStack providers. Kubespray remains supported for existing clusters but will be phased out in future releases.

## Provider Comparison

### Feature Matrix

| Feature | OpenStack | Talos | Kind | AWS | Kubespray |
|---------|-----------|-------|------|-----|-----------|
| **Production Ready** | ✅ | ✅ | ❌ | 🚧 | ⚠️ |
| **HA Control Plane** | ✅ | ✅ | ❌ | 🚧 | ✅ |
| **Auto-scaling** | ⚠️ | ⚠️ | ❌ | 🚧 | ❌ |
| **Immutable OS** | ❌ | ✅ | N/A | 🚧 | ❌ |
| **Security Hardening** | ✅ | ✅ | ❌ | 🚧 | ✅ |
| **Certificate Rotation** | ✅ | ✅ | ❌ | 🚧 | ✅ |
| **In-place Upgrades** | ✅ | ✅ | ❌ | 🚧 | ✅ |
| **Multi-region** | ✅ | ✅ | ❌ | 🚧 | ✅ |
| **Local Development** | ❌ | ❌ | ✅ | ❌ | ❌ |
| **Cost** | Variable | Variable | Free | Variable | Variable |

**Legend**: ✅ Supported | ⚠️ Partial | ❌ Not supported | 🚧 In development | N/A Not applicable

### Provisioning Methods

| Provider | Infrastructure | Kubernetes | Configuration |
|----------|----------------|------------|---------------|
| OpenStack | Terraform | Kubespray (Ansible) | YAML + Terraform |
| Talos | Pulumi | Talos native | YAML + Pulumi |
| Kind | Docker CLI | Kind | YAML + Docker |
| AWS | Terraform (planned) | TBD | YAML + Terraform |
| Kubespray | Terraform | Ansible | YAML + Terraform |

### Operational Characteristics

| Provider | Deployment Time | Complexity | Maintenance Overhead |
|----------|----------------|------------|---------------------|
| OpenStack | 15-30 min | Medium | Medium |
| Talos | 10-20 min | Low | Low |
| Kind | 2-5 min | Very Low | Very Low |
| AWS | TBD | TBD | TBD |
| Kubespray | 20-40 min | High | High |

## Provider Selection Guide

Choose a provider based on your requirements:

**Use OpenStack when:**
- Running on private cloud infrastructure
- Need integration with existing OpenStack services
- Require traditional VM-based deployments
- Have existing OpenStack expertise

**Use Talos when:**
- Security and immutability are priorities
- Want minimal OS attack surface
- Need fast, reliable upgrades
- Prefer declarative infrastructure

**Use Kind when:**
- Developing locally
- Running CI/CD tests
- Prototyping cluster configurations
- Learning Kubernetes

**Use AWS when (future):**
- Deploying to AWS public cloud
- Need AWS-native service integration
- Require global multi-region deployment
- Want managed control plane option

**Use Kubespray when:**
- Maintaining existing Kubespray clusters
- Migrating to newer providers
- Need maximum CNI flexibility

For detailed comparison and decision criteria, see [Provider Selection Guide](../explanation/provider-comparison.md) (planned).

## Provider Documentation Structure

Each provider directory follows this structure:

```
providers/<provider>/
├── README.md              # Provider overview and quick start
├── setup.md              # Detailed setup and configuration
├── networking.md         # Network architecture and options
├── security.md           # Security features and hardening
├── troubleshooting.md    # Common issues and solutions
├── best-practices.md     # Production recommendations
└── examples/             # Configuration examples
```

Not all providers have complete documentation yet. Check the [Documentation Checklist](../CONTENT_CHECKLIST.md) for current status.

## Available Documentation

### OpenStack Provider

- [README.md](openstack/README.md) - Overview and architecture (planned)
- [setup.md](openstack/setup.md) - Setup guide (planned)
- [networking.md](openstack/networking.md) - Network configuration (planned)
- [troubleshooting.md](openstack/troubleshooting.md) - Common issues (planned)

### Talos Provider

- [README.md](talos/README.md) - Specification index
- [design.md](talos/design.md) - Product intent and vision
- [architecture.md](talos/architecture.md) - System blueprint
- [requirements.md](talos/requirements.md) - Product requirements
- [implementation.md](talos/implementation.md) - Implementation details

### Kubespray Provider

- [README.md](kubespray/README.md) - Complete guide with architecture

### Kind Provider

- Documentation planned (see [CONTENT_CHECKLIST.md](../CONTENT_CHECKLIST.md))

### AWS Provider

- Documentation planned (see [CONTENT_CHECKLIST.md](../CONTENT_CHECKLIST.md))

## Common Provider Concepts

### Infrastructure Provisioning

All providers use infrastructure-as-code tools:
- **Terraform/OpenTofu**: OpenStack, AWS, Kubespray
- **Pulumi**: Talos
- **Docker CLI**: Kind

Configuration is generated from the cluster YAML file by openCenter.

### Kubernetes Installation

Providers use different methods to install Kubernetes:
- **Kubespray**: Ansible playbooks (OpenStack, Kubespray providers)
- **Talos**: Native control plane (Talos provider)
- **Kind**: Pre-built images (Kind provider)
- **EKS/Self-managed**: TBD (AWS provider)

### Networking

All providers support multiple CNI options:
- **Calico**: Full-featured, policy-rich
- **Cilium**: eBPF-based, high performance
- **Flannel**: Simple, lightweight
- **AWS CNI**: AWS-specific (AWS provider only)

Network configuration is provider-specific. See individual provider docs for details.

### High Availability

Production providers support HA control planes:
- **OpenStack**: VRRP or Octavia load balancer
- **Talos**: Built-in HA with shared VIP
- **AWS**: Multi-AZ with ELB (planned)
- **Kubespray**: VRRP or external load balancer

Kind does not support true HA (development only).

### Secrets Management

All providers integrate with SOPS for secrets encryption:
- Age keys generated per cluster
- Secrets encrypted in Git
- Automatic decryption during deployment

See [Secrets Management](../how-to/secrets-management.md) for details.

## Provider-Specific Configuration

Each provider has unique configuration options in the cluster YAML:

```yaml
# OpenStack example
provider: openstack
openstack:
  auth_url: https://openstack.example.com:5000/v3
  project_name: my-project
  flavor_master: gp.0.4.4
  flavor_worker: gp.0.4.8
  network_name: my-network

# Talos example
provider: talos
talos:
  version: v1.6.0
  install_disk: /dev/sda
  control_plane_vip: 10.0.0.10

# Kind example
provider: kind
kind:
  version: v0.20.0
  workers: 2
```

See provider-specific documentation for complete configuration reference.

## Migration Between Providers

Migrating between providers requires careful planning:

1. **Backup cluster state** (etcd, persistent volumes)
2. **Export application manifests** from GitOps repo
3. **Create new cluster** with target provider
4. **Restore applications** to new cluster
5. **Migrate workloads** with minimal downtime
6. **Decommission old cluster** after validation

See [Migration Guide](../how-to/migration.md) (planned) for detailed procedures.

## Provider Development

To add a new provider:

1. Create provider directory under `internal/cloud/<provider>/`
2. Implement provider interface (preflight checks, validation)
3. Add provisioning templates to `internal/gitops/templates/`
4. Create provider documentation in `docs/providers/<provider>/`
5. Add BDD tests in `tests/features/`
6. Update this README with provider details

See [Provider Development Guide](../dev/provider-development.md) (planned) for implementation details.

## Getting Help

**Provider-specific issues:**
- Check provider troubleshooting docs
- Review provider GitHub issues
- Consult provider community forums

**openCenter integration issues:**
- Check [Troubleshooting Guide](../how-to/troubleshooting.md)
- Review [GitHub Issues](https://github.com/rackerlabs/openCenter-cli/issues)
- Ask in community Slack/Discord

**Security concerns:**
- Review [Security Model](../explanation/security-model.md) (planned)
- Check provider security documentation
- Follow responsible disclosure process

## Related Documentation

- [Getting Started Tutorial](../tutorials/getting-started.md) (planned)
- [Configuration Reference](../reference/configuration.md)
- [Architecture Overview](../explanation/architecture.md) (planned)
- [Provider Selection Guide](../explanation/provider-comparison.md) (planned)
- [Troubleshooting Guide](../how-to/troubleshooting.md)

## External Resources

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [OpenStack Documentation](https://docs.openstack.org/)
- [Talos Documentation](https://www.talos.dev/docs/)
- [Kind Documentation](https://kind.sigs.k8s.io/)
- [Kubespray Documentation](https://kubespray.io/)

---

**Last Updated**: January 2026  
**Maintained By**: openCenter Team
