# OpenStack Setup Guide


## Table of Contents

- [Task Summary](#task-summary)
- [Prerequisites](#prerequisites)
- [Step 1: Configure Authentication](#step-1-configure-authentication)
- [Step 2: Plan Network Configuration](#step-2-plan-network-configuration)
- [Step 3: Select Compute Resources](#step-3-select-compute-resources)
- [Step 4: Configure Storage](#step-4-configure-storage)
- [Step 5: Configure Load Balancing](#step-5-configure-load-balancing)
- [Step 6: Set Up Security](#step-6-set-up-security)
- [Step 7: Create Configuration File](#step-7-create-configuration-file)
- [Step 8: Validate Configuration](#step-8-validate-configuration)
- [Common Configurations](#common-configurations)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)
- [Related Documentation](#related-documentation)
**doc_type: how-to**

Configure openCenter to deploy Kubernetes clusters on OpenStack infrastructure. This guide walks through authentication setup, network planning, compute resource selection, storage configuration, and load balancing options.

## Task Summary

Set up openCenter to provision Kubernetes clusters on OpenStack by configuring authentication credentials, network topology, compute flavors, storage volumes, and load balancing. The result is a validated cluster configuration ready for deployment.

## Prerequisites

Before starting, you need:

- **OpenStack account** with project/tenant access
- **API access** to your OpenStack cloud (auth URL and credentials)
- **Sufficient quotas** for your cluster size:
  - Instances: 5+ (3 masters, 2+ workers)
  - vCPUs: 20+ (4 per master, 4+ per worker)
  - RAM: 40GB+ (8GB per master, 8GB+ per worker)
  - Floating IPs: 1+ (for API access)
  - Security groups: 2+
  - Volumes: 5+ (boot volumes for nodes)
- **openCenter installed** (see [Getting Started](../../tutorials/getting-started.md))
- **OpenStack CLI tools** (optional but recommended for verification)

Check your quotas:
```bash
openstack quota show
```

## Step 1: Configure Authentication

OpenStack supports two authentication methods: application credentials (recommended) or username/password.

### Option A: Application Credentials (Recommended)

Application credentials provide scoped, revocable access without exposing your password.

Create an application credential:
```bash
openstack application credential create opencenter-cluster \
  --description "openCenter cluster provisioning" \
  --role member
```

Save the output. You'll need the `id` and `secret`.

Add to your cluster configuration:
```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        auth_url: "https://identity.example.com:5000/v3"
        region: "RegionOne"
        application_credential_id: "abc123..."
        application_credential_secret: "xyz789..."
        tenant_name: "my-project"
```

### Option B: Username and Password

If application credentials aren't available, use username/password authentication.

Add to your cluster configuration:
```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        auth_url: "https://identity.example.com:5000/v3"
        region: "RegionOne"
        tenant_name: "my-project"
        user_domain_name: "Default"
        project_domain_name: "Default"
```

Store credentials in environment variables:
```bash
export OS_USERNAME="your-username"
export OS_PASSWORD="your-password"
```

Reference them in your config:
```yaml
# openCenter expands environment variables in config files
user_name: "${OS_USERNAME}"
user_password: "${OS_PASSWORD}"
```

### Verify Authentication

Test your credentials:
```bash
openstack --os-auth-url https://identity.example.com:5000/v3 \
  --os-project-name my-project \
  --os-application-credential-id abc123... \
  --os-application-credential-secret xyz789... \
  server list
```

You should see your instances (or an empty list if none exist).

## Step 2: Plan Network Configuration

Design your cluster's network topology. You need to define CIDR ranges, external network access, and DNS settings.

### Network CIDR Ranges

Choose non-overlapping CIDR blocks:

```yaml
opencenter:
  cluster:
    kubernetes:
      subnet_pods: "10.42.0.0/16"      # Pod network (Calico/Cilium)
      subnet_services: "10.43.0.0/16"  # Service network (ClusterIP)
```

Default ranges work for most deployments. Change them if they conflict with existing networks.

### External Network Access

Identify your OpenStack external network:
```bash
openstack network list --external
```

Note the network ID and floating IP pool name.

Configure in your cluster:
```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        networking:
          floating_ip_pool: "PUBLICNET"
          router_external_network_id: "723f8fa2-dbf7-4cec-8d5f-017e62c12f79"
```

### DNS and NTP Configuration

Set DNS nameservers and NTP servers:
```yaml
opencenter:
  cluster:
    networking:
      dns_nameservers:
        - "8.8.8.8"
        - "8.8.4.4"
      ntp_servers:
        - "time.example.com"
        - "time2.example.com"
```

Use your organization's DNS and NTP servers if available.

### Network Isolation (Optional)

For existing networks, specify network and subnet IDs:
```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        networking:
          network_id: "a1b2c3d4-..."
          subnet_id: "e5f6g7h8-..."
```

Leave empty to create new networks automatically.

## Step 3: Select Compute Resources

Choose instance flavors for your cluster nodes. Flavors define CPU, RAM, and disk allocation.

### List Available Flavors

```bash
openstack flavor list
```

Look for flavors with:
- **Masters**: 4+ vCPUs, 8+ GB RAM
- **Workers**: 4+ vCPUs, 8+ GB RAM (adjust based on workload)

### Configure Node Flavors

```yaml
opencenter:
  cluster:
    kubernetes:
      flavor_master: "gp.0.4.8"    # 4 vCPU, 8GB RAM
      flavor_worker: "gp.0.4.16"   # 4 vCPU, 16GB RAM
      master_count: 3              # HA control plane
      worker_count: 2              # Minimum for workloads
```

### Availability Zones

Distribute nodes across availability zones for resilience:
```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        availability_zone: "az1"
```

For multi-AZ deployments, use additional server pools (see Advanced Configuration).

### Server Group Affinity

Control node placement with server group policies:
```yaml
opencenter:
  infrastructure:
    server_group_affinity:
      - "anti-affinity"  # Spread nodes across hosts
```

Options:
- `anti-affinity`: Nodes on different physical hosts (recommended for HA)
- `affinity`: Nodes on same physical host (testing only)
- `soft-anti-affinity`: Prefer different hosts, allow same if needed
- `soft-affinity`: Prefer same host, allow different if needed

## Step 4: Configure Storage

Set up boot volumes and persistent storage for your cluster.

### Boot Volume Configuration

Configure boot volumes for worker nodes:
```yaml
opencenter:
  storage:
    worker_volume_size: 40                    # GB
    worker_volume_type: "HA-Standard"         # Volume type
    worker_volume_destination_type: "volume"  # Boot from volume
    worker_volume_source_type: "image"        # Create from image
    default_storage_class: "csi-cinder-sc-delete"
```

### List Volume Types

```bash
openstack volume type list
```

Common types:
- `HA-Standard`: Replicated, standard performance
- `HA-Performance`: Replicated, high IOPS
- `Standard`: Non-replicated, standard performance

Choose `HA-*` types for production.

### Image Selection

Specify the OS image for your nodes:
```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        image_id: "799dcf97-3656-4361-8187-13ab1b295e33"  # Ubuntu 24.04
```

Find available images:
```bash
openstack image list --property os_distro=ubuntu
```

openCenter supports Ubuntu 22.04 and 24.04.

### Additional Block Devices (Optional)

Attach extra volumes to worker nodes:
```yaml
opencenter:
  storage:
    additional_block_devices:
      - volume_size: 100
        volume_type: "HA-Performance"
        device_name: "/dev/vdb"
```

## Step 5: Configure Load Balancing

Choose a load balancing method for the Kubernetes API server.

### Option A: OVN Load Balancer (Recommended)

OVN provides built-in load balancing without additional infrastructure:
```yaml
opencenter:
  cluster:
    kubernetes:
      loadbalancer_provider: "ovn"
      kube_vip_enabled: true
```

This creates a virtual IP (VIP) managed by kube-vip on the control plane.

### Option B: Octavia Load Balancer

Use OpenStack Octavia for external load balancing:
```yaml
opencenter:
  cluster:
    kubernetes:
      loadbalancer_provider: "octavia"
      kube_vip_enabled: false
```

Requires Octavia service in your OpenStack cloud. Check availability:
```bash
openstack loadbalancer list
```

### Option C: VRRP (Legacy)

VRRP provides failover without external load balancers:
```yaml
opencenter:
  cluster:
    kubernetes:
      loadbalancer_provider: "vrrp"
      kube_vip_enabled: true
```

Configure the VIP address:
```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        vrrp_ip: "10.0.0.100"  # Must be in your subnet range
```

## Step 6: Set Up Security

Configure SSH keys and security groups for cluster access.

### SSH Key Configuration

openCenter generates SSH keys automatically during initialization. To use existing keys:

```yaml
secrets:
  ssh_key:
    private: "/path/to/private-key"
    public: "/path/to/public-key.pub"
    cypher: "ed25519"
```

Add authorized keys for cluster access:
```yaml
opencenter:
  cluster:
    ssh_authorized_keys:
      - "ssh-ed25519 AAAAC3... user@host"
      - "ssh-rsa AAAAB3... admin@host"
```

### API Access Control

Restrict Kubernetes API access by IP:
```yaml
opencenter:
  cluster:
    networking:
      k8s_api_port_acl:
        - "203.0.113.0/24"    # Office network
        - "198.51.100.50/32"  # VPN gateway
```

Use `0.0.0.0/0` to allow access from anywhere (not recommended for production).

### Security Hardening

Enable OS and Kubernetes hardening:
```yaml
opencenter:
  cluster:
    networking:
      security:
        os_hardening: true
    kubernetes:
      security:
        k8s_hardening: true
        pod_security_exemptions:
          - "kube-system"
          - "trivy-temp"
```

## Step 7: Create Configuration File

Initialize a cluster configuration with your settings:

```bash
./bin/openCenter cluster init my-openstack-cluster \
  --opencenter.infrastructure.cloud.openstack.auth_url="https://identity.example.com:5000/v3" \
  --opencenter.infrastructure.cloud.openstack.region="RegionOne" \
  --opencenter.infrastructure.cloud.openstack.tenant_name="my-project" \
  --opencenter.cluster.kubernetes.flavor_master="gp.0.4.8" \
  --opencenter.cluster.kubernetes.flavor_worker="gp.0.4.16" \
  --opencenter.cluster.kubernetes.master_count=3 \
  --opencenter.cluster.kubernetes.worker_count=2
```

This creates a configuration file at:
```
~/.config/openCenter/clusters/opencenter/.my-openstack-cluster-config.yaml
```

Edit the file to add authentication credentials and other settings.

## Step 8: Validate Configuration

Run preflight checks to verify your configuration:

```bash
./bin/openCenter cluster validate my-openstack-cluster
```

The validator checks:
- Schema compliance
- Required fields
- Network CIDR conflicts
- OpenStack CLI availability
- Authentication URL format

Fix any errors before proceeding.

### Manual Verification

Test OpenStack connectivity:
```bash
# List compute resources
openstack server list

# Check network access
openstack network list

# Verify quotas
openstack quota show

# Test image access
openstack image show 799dcf97-3656-4361-8187-13ab1b295e33
```

## Common Configurations

### Development Cluster (Minimal Resources)

Single master, minimal workers for testing:
```yaml
opencenter:
  cluster:
    kubernetes:
      master_count: 1
      worker_count: 1
      flavor_master: "gp.0.2.4"   # 2 vCPU, 4GB RAM
      flavor_worker: "gp.0.2.4"
```

Not recommended for production. No HA control plane.

### Production Cluster (High Availability)

Three masters, multiple workers, anti-affinity:
```yaml
opencenter:
  cluster:
    kubernetes:
      master_count: 3
      worker_count: 3
      flavor_master: "gp.0.4.8"
      flavor_worker: "gp.0.8.32"  # 8 vCPU, 32GB RAM
  infrastructure:
    server_group_affinity:
      - "anti-affinity"
    cloud:
      openstack:
        networking:
          floating_ip_pool: "PUBLICNET"
  storage:
    worker_volume_type: "HA-Performance"
```

### High-Security Cluster (Private Network)

Private network with bastion host and restricted API access:
```yaml
opencenter:
  cluster:
    networking:
      k8s_api_port_acl:
        - "10.0.0.0/8"  # Internal network only
      security:
        os_hardening: true
    kubernetes:
      flavor_bastion: "gp.0.2.2"  # Bastion host
      security:
        k8s_hardening: true
  infrastructure:
    cloud:
      openstack:
        networking:
          network_id: "private-net-id"
          subnet_id: "private-subnet-id"
```

Access the cluster through the bastion host.

## Troubleshooting

### Authentication Failures

**Symptom**: `openstack CLI not found` or `authentication may fail` warnings.

**Solution**: Install OpenStack client tools:
```bash
pip install python-openstackclient
```

Configure environment variables or `clouds.yaml`:
```yaml
# ~/.config/openstack/clouds.yaml
clouds:
  mycloud:
    auth:
      auth_url: https://identity.example.com:5000/v3
      application_credential_id: "abc123..."
      application_credential_secret: "xyz789..."
    region_name: RegionOne
```

Test:
```bash
openstack --os-cloud mycloud server list
```

### Network Configuration Issues

**Symptom**: `floating_ip_pool is empty` or network creation fails.

**Solution**: Verify external network exists:
```bash
openstack network list --external
```

Check router configuration:
```bash
openstack router list
openstack router show <router-id>
```

Ensure your project has access to the external network.

### Quota Exceeded

**Symptom**: Deployment fails with quota errors.

**Solution**: Check current usage:
```bash
openstack quota show
openstack server list
openstack volume list
```

Request quota increase from your OpenStack administrator or reduce cluster size:
```yaml
opencenter:
  cluster:
    kubernetes:
      master_count: 1  # Reduce from 3
      worker_count: 1  # Reduce from 2+
```

### Image Not Found

**Symptom**: `image_id` validation fails or deployment can't find image.

**Solution**: List available images:
```bash
openstack image list --property os_distro=ubuntu
```

Update configuration with valid image ID:
```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        image_id: "valid-image-uuid"
```

### Flavor Not Available

**Symptom**: Deployment fails with "flavor not found" error.

**Solution**: List available flavors:
```bash
openstack flavor list
```

Choose flavors that exist in your cloud:
```yaml
opencenter:
  cluster:
    kubernetes:
      flavor_master: "m1.medium"  # Use actual flavor name
      flavor_worker: "m1.large"
```

### Load Balancer Issues

**Symptom**: API server unreachable or VIP not assigned.

**Solution**: For OVN load balancer, verify kube-vip is enabled:
```yaml
opencenter:
  cluster:
    kubernetes:
      loadbalancer_provider: "ovn"
      kube_vip_enabled: true
```

For Octavia, check service availability:
```bash
openstack loadbalancer list
```

Switch to VRRP if Octavia is unavailable:
```yaml
opencenter:
  cluster:
    kubernetes:
      loadbalancer_provider: "vrrp"
```

## Next Steps

After validating your configuration:

1. **Generate GitOps repository**: Run `openCenter cluster setup` to create infrastructure manifests
2. **Review generated files**: Check Terraform configurations in your GitOps repository
3. **Deploy cluster**: Run `openCenter cluster bootstrap` to provision infrastructure
4. **Verify deployment**: Access your cluster with `kubectl` after bootstrap completes

See [OpenStack Deployment Tutorial](../../tutorials/openstack-deployment.md) (planned) for complete deployment workflow.

## Related Documentation

- [Getting Started Tutorial](../../tutorials/getting-started.md)
- [Configuration Reference](../../reference/configuration.md)
- [OpenStack Provider Overview](README.md) (planned)
- [OpenStack Networking Guide](networking.md) (planned)
- [Troubleshooting Guide](../../how-to/troubleshooting.md)

---

**Last Updated**: January 2026  
**Maintained By**: openCenter Team
