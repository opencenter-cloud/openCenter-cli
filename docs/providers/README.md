# Infrastructure Providers

openCenter supports multiple infrastructure providers for deploying Kubernetes clusters.

## Supported Providers

### Production Ready

- **[OpenStack](./openstack.md)** - Primary target, full automation with Terraform/OpenTofu
- **[VMware vSphere](./vmware.md)** - Pre-provisioned VMs, treated as baremetal

### Development

- **Kind** - Local development clusters using Docker containers

### Planned

- **AWS** - Amazon Web Services (in development)
- **Baremetal** - Physical servers with manual provisioning
- **Talos** - Talos Linux with Pulumi-based provisioning

## Provider Comparison

| Provider | Provisioning | Deployment Methods | Storage | Load Balancer |
|----------|--------------|-------------------|---------|---------------|
| OpenStack | Terraform/OpenTofu | Kubespray, Talos, Kamaji | Cinder CSI | Octavia, OVN |
| VMware | Manual (pre-provisioned) | Kubespray, Kamaji | vSphere CSI | MetalLB |
| AWS | Terraform/OpenTofu | Kubespray, Talos, Kamaji | EBS CSI | AWS ELB |
| Baremetal | Manual | Kubespray | Local storage | MetalLB |
| Kind | Automatic | Built-in | Local storage | None |

## Choosing a Provider

### OpenStack
Use when:
- Deploying to Rackspace or other OpenStack clouds
- Need full infrastructure automation
- Require dynamic VM provisioning and scaling

### VMware
Use when:
- VMs are pre-provisioned by infrastructure team
- vSphere environment with existing VM templates
- Need vSphere CSI driver for persistent storage
- Cannot use cloud-init or automated provisioning

### Kind
Use when:
- Local development and testing
- CI/CD pipeline testing
- Learning Kubernetes and GitOps workflows

## Configuration Examples

### OpenStack

```yaml
opencenter:
  infrastructure:
    provider: openstack
    cloud:
      openstack:
        auth_url: https://identity.api.rackspacecloud.com/v3
        region: sjc3
        application_credential_id: "..."
        application_credential_secret: "..."
        image_id: "799dcf97-3656-4361-8187-13ab1b295e33"
```

### VMware

```yaml
opencenter:
  infrastructure:
    provider: vmware
    cloud:
      vmware:
        vcenter_server: vcenter.example.com
        datacenter: Datacenter1
        datastore: datastore1
        nodes:
          - name: master-1.example.com
            ip: 192.168.1.10
            role: master
```

### Kind

```yaml
opencenter:
  infrastructure:
    provider: kind
  cluster:
    kubernetes:
      version: 1.33.5
      master_count: 1
      worker_count: 2
```

## Provider-Specific Features

### OpenStack Features
- Automatic VM provisioning
- Floating IP management
- Security group configuration
- Cinder volume integration
- Octavia load balancer
- Designate DNS integration

### VMware Features
- vSphere CSI driver
- Datastore integration
- VM folder organization
- Resource pool management
- vCenter API integration

### Kind Features
- Fast cluster creation (< 1 minute)
- Multiple clusters on single host
- Port mapping for services
- Local registry integration

## Deployment Method Compatibility

| Provider | Kubespray | Talos | Kamaji |
|----------|-----------|-------|--------|
| OpenStack | ✓ | ✓ | ✓ |
| VMware | ✓ | ✗ | ✓ |
| AWS | ✓ | ✓ | ✓ |
| Baremetal | ✓ | ✗ | ✗ |
| Kind | ✓ | ✗ | ✗ |

Notes:
- Talos requires cloud-init support (not available for VMware/baremetal)
- Kamaji requires infrastructure provider for worker nodes

## Getting Started

1. Choose your provider based on infrastructure availability
2. Generate configuration template:
   ```bash
   opencenter cluster template --provider <provider> --out config.yaml
   ```
3. Follow provider-specific guide for detailed setup
4. Initialize cluster:
   ```bash
   opencenter cluster init my-cluster --provider <provider>
   ```

## Related Documentation

- [Deployment Methods](../deployment/)
- [Storage Configuration](../storage/)
- [Networking](../networking/)
- [Services](../services/)
