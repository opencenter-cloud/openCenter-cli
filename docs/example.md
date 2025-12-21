# Rendering main.tf Example

This document demonstrates how to generate a `main.tf` file using the openCenter CLI. The `main.tf` file contains the complete Terraform configuration for provisioning Kubernetes cluster infrastructure.

## Overview

The `main.tf` file is automatically generated during the cluster setup process and contains:

- **Local variables**: Cluster configuration values (networking, authentication, node specifications)
- **Infrastructure module**: OpenStack Nova resources (VMs, networks, security groups)
- **Kubespray module**: Kubernetes cluster deployment via Ansible
- **Calico module**: CNI network plugin configuration

## Complete Example: Generating the testdata/main.tf

To generate a main.tf file that matches the example in `testdata/main.tf`, use this exact command sequence:

### Step 1: Initialize Cluster Configuration

```bash
# Build the CLI
mise run build

# Initialize cluster with specific configuration matching testdata/main.tf
./bin/openCenter cluster init gdo.prod.sjc3 \
  --org opencenter \
  --opencenter.meta.env=prod \
  --opencenter.meta.region=sjc3 \
  --opencenter.infrastructure.provider=openstack \
  --opencenter.infrastructure.cloud.openstack.auth_url="https://keystone.api.sjc3.rackspacecloud.com/v3/" \
  --opencenter.infrastructure.cloud.openstack.region=SJC3 \
  --opencenter.infrastructure.cloud.openstack.tenant_name="f2823901-4194-40c7-9dc4-d56d2105e81a" \
  --opencenter.infrastructure.cloud.openstack.domain="rackspace_cloud_domain" \
  --opencenter.cluster.kubernetes.version="1.32.8" \
  --opencenter.cluster.nodes.master.count=3 \
  --opencenter.cluster.nodes.master.flavor="gp.0.4.8" \
  --opencenter.cluster.nodes.worker.count=4 \
  --opencenter.cluster.nodes.worker.flavor="gp.0.4.16" \
  --opencenter.cluster.networking.pod_subnet="10.42.0.0/16" \
  --opencenter.cluster.networking.service_subnet="10.43.0.0/16" \
  --opencenter.cluster.networking.node_subnet="10.2.128.0/22" \
  --opencenter.cluster.oidc.enabled=true \
  --opencenter.cluster.oidc.issuer_url="https://auth.gdo.prod.sjc3.k8s.opencenter.cloud/realms/opencenter" \
  --opencenter.cluster.oidc.client_id="opencenter"
```

### Step 2: Generate GitOps Repository with main.tf

```bash
# Setup GitOps repository with rendered templates
./bin/openCenter cluster setup gdo.prod.sjc3 --render
```

**Output location**: `~/.config/openCenter/clusters/opencenter/gitops/infrastructure/clusters/gdo.prod.sjc3/main.tf`

## Alternative Methods

### Method 1: Template Rendering Only

This renders templates without git initialization:

```bash
# Render templates only (no git operations)
./bin/openCenter cluster render gdo.prod.sjc3
```

### Method 2: Using Mise Task (if terraform-generate command exists)

```bash
# Note: This command may not be implemented yet
mise run terraform-generate gdo.prod.sjc3 ./terraform-output
```

## Key Configuration Values Explained

The command above sets specific values that generate the testdata/main.tf content:

| Flag | Value | Generates in main.tf |
|------|-------|---------------------|
| `gdo.prod.sjc3` | Cluster name | `cluster_name = "gdo.prod.sjc3"` |
| `--opencenter.infrastructure.cloud.openstack.auth_url` | `"https://keystone.api.sjc3.rackspacecloud.com/v3/"` | `openstack_auth_url = "https://keystone.api.sjc3.rackspacecloud.com/v3/"` |
| `--opencenter.infrastructure.cloud.openstack.region` | `SJC3` | `openstack_region = "SJC3"` |
| `--opencenter.infrastructure.cloud.openstack.tenant_name` | `"f2823901-4194-40c7-9dc4-d56d2105e81a"` | `openstack_tenant_name = "f2823901-4194-40c7-9dc4-d56d2105e81a"` |
| `--opencenter.cluster.kubernetes.version` | `"1.32.8"` | `kubernetes_version = "1.32.8"` |
| `--opencenter.cluster.nodes.master.count` | `3` | `master_count = 3` |
| `--opencenter.cluster.nodes.worker.count` | `4` | `worker_count = 4` |
| `--opencenter.cluster.networking.pod_subnet` | `"10.42.0.0/16"` | `subnet_pods = "10.42.0.0/16"` |
| `--opencenter.cluster.networking.service_subnet` | `"10.43.0.0/16"` | `subnet_services = "10.43.0.0/16"` |
| `--opencenter.cluster.networking.node_subnet` | `"10.2.128.0/22"` | `subnet_nodes = "10.2.128.0/22"` |
| `--opencenter.cluster.oidc.enabled` | `true` | `kube_oidc_auth_enabled = true` |
| `--opencenter.cluster.oidc.issuer_url` | `"https://auth.gdo.prod.sjc3.k8s.opencenter.cloud/realms/opencenter"` | `kube_oidc_url = "https://auth.gdo.prod.sjc3.k8s.opencenter.cloud/realms/opencenter"` |

## Generated main.tf Structure

The command above generates a main.tf with this structure:

```hcl
locals {
  # Cluster identification
  cluster_name = "gdo.prod.sjc3"
  naming_prefix = "${local.cluster_name}-"
  
  # OpenStack authentication
  openstack_auth_url = "https://keystone.api.sjc3.rackspacecloud.com/v3/"
  openstack_region = "SJC3"
  openstack_tenant_name = "f2823901-4194-40c7-9dc4-d56d2105e81a"
  openstack_project_domain_name = "rackspace_cloud_domain"
  openstack_user_domain_name = "rackspace_cloud_domain"
  
  # Network configuration
  subnet_nodes = "10.2.128.0/22"
  subnet_pods = "10.42.0.0/16"
  subnet_services = "10.43.0.0/16"
  vrrp_ip = "10.2.128.10"
  
  # Node specifications
  master_count = 3
  worker_count = 4
  flavor_master = "gp.0.4.8"
  flavor_worker = "gp.0.4.16"
  
  # Kubernetes configuration
  kubernetes_version = "1.32.8"
  kubespray_version = "v2.28.1"
  network_plugin = "calico"
  
  # OIDC Authentication
  kube_oidc_auth_enabled = true
  kube_oidc_url = "https://auth.gdo.prod.sjc3.k8s.opencenter.cloud/realms/opencenter"
  kube_oidc_client_id = "opencenter"
}

module "openstack-nova" {
  source = "github.com/rackerlabs/openCenter-gitops-base.git//iac/cloud/openstack/openstack-nova?ref=main"
  # ... all local variables passed as module inputs
}

module "kubespray-cluster" {
  source = "github.com/rackerlabs/openCenter-gitops-base.git//iac/provider/kubespray?ref=main"
  # ... cluster deployment configuration
}

module "calico" {
  source = "github.com/rackerlabs/openCenter-gitops-base.git//iac/cni/calico?ref=main"
  # ... CNI configuration
}
```

## Configuration Sources

The main.tf values are populated from your cluster configuration file:

```yaml
# Example cluster configuration (~/.config/openCenter/clusters/org/my-cluster/.my-cluster-config.yaml)
openCenter:
  meta:
    name: my-cluster
    organization: my-org
  infrastructure:
    provider: openstack
    cloud:
      openstack:
        authURL: "https://keystone.api.region.provider.com/v3/"
        region: "REGION"
  cluster:
    kubernetes:
      version: "1.32.8"
    networking:
      podSubnet: "10.42.0.0/16"
      serviceSubnet: "10.43.0.0/16"
    nodes:
      master:
        count: 3
        flavor: "gp.0.4.8"
      worker:
        count: 4
        flavor: "gp.0.4.16"
```

## Verification

After generation, verify the main.tf file:

```bash
# Check if file exists and view content
ls -la ~/.config/openCenter/clusters/opencenter/gitops/infrastructure/clusters/gdo.prod.sjc3/main.tf

# View the generated content
cat ~/.config/openCenter/clusters/opencenter/gitops/infrastructure/clusters/gdo.prod.sjc3/main.tf

# Validate Terraform syntax
cd ~/.config/openCenter/clusters/opencenter/gitops/infrastructure/clusters/gdo.prod.sjc3/
terraform validate
```

## Common Use Cases

### Development/Testing
```bash
# Initialize test cluster with minimal configuration
./bin/openCenter cluster init test-cluster \
  --opencenter.infrastructure.provider=openstack \
  --opencenter.infrastructure.cloud.openstack.region=SJC3

# Render to inspect output
./bin/openCenter cluster render test-cluster
```

### Production Deployment
```bash
# Full production cluster setup
./bin/openCenter cluster init prod-cluster \
  --org production \
  --opencenter.meta.env=prod \
  --opencenter.cluster.kubernetes.version=1.32.8 \
  --opencenter.cluster.nodes.master.count=3 \
  --opencenter.cluster.nodes.worker.count=6

# Setup GitOps repository
./bin/openCenter cluster setup prod-cluster --render
```

### Configuration Updates
```bash
# Update existing cluster configuration
./bin/openCenter cluster update gdo.prod.sjc3 --opencenter.cluster.nodes.worker.count=6

# Re-render templates with updated values
./bin/openCenter cluster render gdo.prod.sjc3
```

## Troubleshooting

### Template Rendering Errors
If template rendering fails:

```bash
# Validate cluster configuration first
./bin/openCenter cluster validate my-cluster

# Check for missing required fields
./bin/openCenter cluster info my-cluster --json
```

### Missing Variables
Ensure all required configuration fields are populated:

```bash
# View current configuration
./bin/openCenter cluster info my-cluster

# Update missing fields
./bin/openCenter cluster update my-cluster --infrastructure.cloud.openstack.region=REGION
```

## Related Commands

- `openCenter cluster init` - Initialize cluster configuration
- `openCenter cluster validate` - Validate configuration before rendering
- `openCenter cluster setup` - Complete GitOps setup with git initialization
- `openCenter cluster render` - Render templates without git operations
- `openCenter cluster bootstrap` - Deploy infrastructure using generated main.tf

## See Also

- [Cluster Setup Documentation](reference/cluster/setup.md)
- [Template Rendering Documentation](reference/cluster/render.md)
- [Kubespray Provider Documentation](providers/kubespray/readme.md)