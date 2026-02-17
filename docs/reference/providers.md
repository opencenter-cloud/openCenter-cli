# Infrastructure Providers Reference

**Purpose:** Complete reference of supported infrastructure providers, requirements, and configuration for quick lookup.

This reference documents all infrastructure providers supported by openCenter with their specific requirements and configurations.

## Provider Overview

| Provider | Status | Provisioning | Deployment | Storage | Load Balancer |
|----------|--------|--------------|------------|---------|---------------|
| OpenStack | Production | Terraform | Kubespray | Cinder CSI | Octavia, OVN |
| VMware | Production | Manual | Kubespray | vSphere CSI | MetalLB |
| Kind | Development | Automatic | Built-in | Local | None |
| AWS | Experimental | Terraform | Kubespray | EBS CSI | AWS ELB |
| Baremetal | Planned | Manual | Kubespray | Local | MetalLB |
| Talos | Planned | Pulumi | Talos | Provider CSI | Provider LB |

## OpenStack Provider

**Status:** Production Ready  
**Default:** Yes  
**Provisioning:** Terraform/OpenTofu  
**Deployment:** Kubespray

### Requirements

- OpenStack cloud with Keystone v3
- Application credentials or user credentials
- Floating IP pool
- Sufficient quotas:
  - Instances: 5+ (3 masters, 2 workers minimum)
  - vCPUs: 20+ (4 vCPUs per master, 4 per worker)
  - RAM: 80GB+ (8GB per master, 16GB per worker)
  - Volumes: 5+ (40GB per node)
  - Floating IPs: 1+ (for API access)
  - Security groups: 2+

### Configuration

```yaml
opencenter:
  infrastructure:
    provider: openstack
    cloud:
      openstack:
        auth_url: "https://identity.api.rackspacecloud.com/v3"
        region: "sjc3"
        application_credential_id: "your-app-cred-id"
        application_credential_secret: "your-app-cred-secret"
        domain: "Default"
        availability_zone: "az1"
        project_domain_name: "rackspace_cloud_domain"
        user_domain_name: "rackspace_cloud_domain"
        image_id: "799dcf97-3656-4361-8187-13ab1b295e33"
        networking:
          floating_network_id: "your-floating-network-id"
          floating_ip_pool: "PUBLICNET"
          router_external_network_id: "your-external-network-id"
          k8s_api_port_acl:
            - "10.0.0.0/8"
            - "192.168.1.0/24"

secrets:
  global:
    openstack:
      application_credential_id: "your-app-cred-id"
      application_credential_secret: "your-app-cred-secret"
```

### Features

**Supported:**
- Automatic VM provisioning
- Floating IP management
- Security group configuration
- Cinder volume integration
- Octavia load balancer
- Designate DNS integration
- Server groups (anti-affinity)
- Multiple availability zones

**Services:**
- openstack-ccm (cloud controller)
- openstack-csi (Cinder CSI driver)
- Octavia or OVN load balancer

### Network Configuration

**Load Balancer Options:**

1. **Octavia (Recommended for Production):**
   ```yaml
   opencenter:
     cluster:
       networking:
         use_octavia: true
         loadbalancer_provider: "octavia"
   ```

2. **OVN (No External Dependency):**
   ```yaml
   opencenter:
     cluster:
       networking:
         use_octavia: false
         loadbalancer_provider: "ovn"
         vrrp_enabled: true
         vrrp_ip: "10.0.0.10"
   ```

### Validation

```bash
# Validate OpenStack credentials
openstack token issue

# Check quotas
openstack quota show

# List available images
openstack image list

# List networks
openstack network list

# Validate configuration
opencenter cluster validate --check-provider
```

### Troubleshooting

**Common Issues:**

1. **Quota Exceeded:**
   - Increase OpenStack quotas
   - Reduce node count
   - Use smaller flavors

2. **Image Not Found:**
   - Verify image ID exists: `openstack image show <id>`
   - Use correct image for region

3. **Network Not Found:**
   - Verify network IDs: `openstack network list`
   - Check network accessibility

## VMware vSphere Provider

**Status:** Production Ready  
**Default:** No  
**Provisioning:** Manual (pre-provisioned VMs)  
**Deployment:** Kubespray

### Requirements

- vSphere 7.0 or later
- Pre-provisioned VMs with:
  - Ubuntu 24.04 LTS
  - SSH access configured
  - Network connectivity
  - Sufficient resources (4 vCPU, 8GB RAM minimum per node)
- vCenter credentials
- Datastore access
- Network with DHCP or static IPs

### Configuration

```yaml
opencenter:
  infrastructure:
    provider: vmware
    cloud:
      vmware:
        vcenter_server: "vcenter.example.com"
        datacenter: "Datacenter1"
        datastore: "datastore1"
        nodes:
          - name: "master-1.example.com"
            ip: "192.168.1.10"
            role: "master"
          - name: "master-2.example.com"
            ip: "192.168.1.11"
            role: "master"
          - name: "master-3.example.com"
            ip: "192.168.1.12"
            role: "master"
          - name: "worker-1.example.com"
            ip: "192.168.1.20"
            role: "worker"
          - name: "worker-2.example.com"
            ip: "192.168.1.21"
            role: "worker"

secrets:
  vsphere_csi:
    vcenter_host: "vcenter.example.com"
    username: "administrator@vsphere.local"
    password: "your-vcenter-password"
    datacenters: "Datacenter1"
    insecure_flag: "false"
    port: "443"
```

### Features

**Supported:**
- Pre-provisioned VM deployment
- vSphere CSI driver
- Datastore integration
- VM folder organization
- Resource pool management

**Services:**
- vsphere-csi (vSphere CSI driver)
- MetalLB (load balancer)

**Not Supported:**
- Automatic VM provisioning
- Cloud controller manager
- Dynamic infrastructure scaling

### Network Configuration

**Load Balancer:**

MetalLB is required for LoadBalancer services:

```yaml
opencenter:
  cluster:
    kubernetes:
      loadbalancer_provider: "metallb"
```

Configure IP address pool after deployment:

```yaml
# applications/overlays/<cluster>/services/metallb/ipaddresspool.yaml
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: default
  namespace: metallb-system
spec:
  addresses:
  - 192.168.1.100-192.168.1.200
```

### Validation

```bash
# Test SSH connectivity
ssh ubuntu@192.168.1.10

# Verify vCenter connectivity
# (requires vCenter CLI or API access)

# Validate configuration
opencenter cluster validate
```

### Troubleshooting

**Common Issues:**

1. **SSH Connection Failed:**
   - Verify SSH keys are configured on VMs
   - Check network connectivity
   - Ensure firewall allows SSH (port 22)

2. **vSphere CSI Not Working:**
   - Verify vCenter credentials
   - Check datastore permissions
   - Ensure VMs have disk UUIDs enabled

3. **MetalLB Not Assigning IPs:**
   - Verify IP pool configuration
   - Check network routing
   - Ensure IPs are not in use

## Kind Provider

**Status:** Development Only  
**Default:** No  
**Provisioning:** Automatic (Docker containers)  
**Deployment:** Built-in

### Requirements

- Docker installed and running
- Sufficient resources:
  - CPU: 4+ cores
  - RAM: 8GB+
  - Disk: 20GB+
- Kind CLI installed

### Configuration

```yaml
opencenter:
  infrastructure:
    provider: kind
  cluster:
    kubernetes:
      version: "1.33.5"
      master_count: 1
      worker_count: 2
```

### Features

**Supported:**
- Fast cluster creation (< 1 minute)
- Multiple clusters on single host
- Port mapping for services
- Local registry integration

**Services:**
- Calico CNI
- Gateway API
- Basic platform services

**Not Supported:**
- Persistent storage (by default)
- Load balancer services
- Cloud provider integration
- Production workloads

### Network Configuration

**Load Balancer:**

No load balancer by default. Use NodePort or port mapping:

```yaml
opencenter:
  cluster:
    kubernetes:
      loadbalancer_provider: "none"
```

### Validation

```bash
# Verify Docker is running
docker ps

# Validate configuration
opencenter cluster validate
```

### Troubleshooting

**Common Issues:**

1. **Docker Not Running:**
   - Start Docker daemon
   - Check Docker permissions

2. **Insufficient Resources:**
   - Increase Docker resource limits
   - Reduce node count

3. **Port Conflicts:**
   - Check for port conflicts (6443, 80, 443)
   - Use different ports

## AWS Provider

**Status:** Experimental  
**Default:** No  
**Provisioning:** Terraform  
**Deployment:** Kubespray

### Requirements

- AWS account with credentials
- VPC with public and private subnets
- Sufficient quotas:
  - EC2 instances: 5+
  - EBS volumes: 5+
  - Elastic IPs: 1+
- IAM permissions for EC2, VPC, ELB

### Configuration

```yaml
opencenter:
  infrastructure:
    provider: aws
    cloud:
      aws:
        profile: "default"
        region: "us-east-1"
        vpc_id: "vpc-12345678"
        private_subnets:
          - "subnet-12345678"
          - "subnet-87654321"
        public_subnets:
          - "subnet-abcdef12"

secrets:
  global:
    aws:
      infrastructure:
        access_key: "your-access-key"
        secret_access_key: "your-secret-key"
        region: "us-east-1"
```

### Features

**Supported:**
- Automatic EC2 provisioning
- EBS volume integration
- AWS ELB load balancer
- Security group management

**Services:**
- aws-ccm (cloud controller)
- aws-ebs-csi (EBS CSI driver)
- AWS ELB (load balancer)

**Limitations:**
- Experimental status
- Limited testing
- May have incomplete features

### Validation

```bash
# Validate AWS credentials
aws sts get-caller-identity

# Check VPC
aws ec2 describe-vpcs --vpc-ids vpc-12345678

# Validate configuration
opencenter cluster validate --check-provider
```

## Baremetal Provider

**Status:** Planned  
**Default:** No  
**Provisioning:** Manual  
**Deployment:** Kubespray

### Requirements

- Physical servers or VMs
- Ubuntu 24.04 LTS
- SSH access
- Network connectivity
- IPMI or BMC access (optional)

### Configuration

```yaml
opencenter:
  infrastructure:
    provider: baremetal
    nodes:
      - name: "node1.example.com"
        ip: "10.0.0.10"
        role: "master"
      - name: "node2.example.com"
        ip: "10.0.0.11"
        role: "master"
      - name: "node3.example.com"
        ip: "10.0.0.12"
        role: "master"
      - name: "node4.example.com"
        ip: "10.0.0.20"
        role: "worker"
```

### Features

**Planned:**
- Manual node provisioning
- Local storage
- MetalLB load balancer
- No cloud provider integration

## Talos Provider

**Status:** Planned  
**Default:** No  
**Provisioning:** Pulumi  
**Deployment:** Talos

### Requirements

- Talos Linux images
- Cloud provider or baremetal
- Pulumi CLI
- Talos CLI (talosctl)

### Configuration

```yaml
opencenter:
  infrastructure:
    provider: openstack  # or other provider
  talos:
    enabled: true
    version: "v1.8.0"
    image_url: "https://github.com/siderolabs/talos/releases/download/v1.8.0/openstack-amd64.raw.xz"
    machine_config:
      apparmor_enabled: true
      seccomp_enabled: true
      disk_encryption: true
```

### Features

**Planned:**
- Immutable OS
- API-driven configuration
- Enhanced security
- Fast boot times

## Provider Comparison

### Use Cases

**OpenStack:**
- Production deployments
- Multi-tenant environments
- Dynamic scaling
- Full automation

**VMware:**
- Existing vSphere infrastructure
- Pre-provisioned VMs
- Enterprise environments
- vSphere integration

**Kind:**
- Local development
- CI/CD testing
- Learning Kubernetes
- Quick prototyping

**AWS:**
- AWS-native deployments
- Cloud-first organizations
- AWS service integration
- Global reach

### Feature Matrix

| Feature | OpenStack | VMware | Kind | AWS |
|---------|-----------|--------|------|-----|
| Auto Provisioning | ✓ | ✗ | ✓ | ✓ |
| Cloud Controller | ✓ | ✗ | ✗ | ✓ |
| CSI Driver | ✓ | ✓ | ✗ | ✓ |
| Load Balancer | ✓ | MetalLB | ✗ | ✓ |
| DNS Integration | ✓ | ✗ | ✗ | ✓ |
| Production Ready | ✓ | ✓ | ✗ | Experimental |

## Switching Providers

**Warning:** Switching providers requires cluster rebuild. Configuration cannot be migrated between providers.

**Process:**

1. Backup data and configurations
2. Create new configuration with different provider
3. Deploy new cluster
4. Migrate workloads
5. Decommission old cluster

---

## Evidence

This reference is based on:

- Provider documentation: `docs/providers/README.md:1-200`
- Provider defaults: `internal/config/defaults.go:68-157`
- Session 2 facts inventory: B0 section 4
- Ecosystem provider comparison: Ecosystem.md
