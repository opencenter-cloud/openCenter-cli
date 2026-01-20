# Deploy and Destroy Clusters

This guide covers the complete workflow for deploying and destroying Kubernetes clusters using openCenter CLI on OpenStack.

## Prerequisites

- openCenter CLI installed and built (`mise run build`)
- OpenStack credentials configured in `~/.config/openstack/clouds.yaml`
- Terraform installed (`mise use -g terraform@latest && mise install terraform`)
- SSH access to target infrastructure

## Discovering OpenStack Resources

Before deploying a cluster, you need to identify the correct resource IDs for your target OpenStack region.

### Finding Available Flavors

List all available compute flavors in your region:

```bash
openstack --os-cloud <cloud-name> flavor list --format json | jq -r '.[] | "\(.Name) - \(.VCPUs) vCPUs, \(.RAM)MB RAM"'
```

For general-purpose flavors (gp prefix):

```bash
openstack --os-cloud <cloud-name> flavor list --format json | jq -r '.[] | select(.Name | startswith("gp.")) | .Name' | sort
```

**Example output for DFW3:**
```
gp.5.1.2    # 1 vCPU, 2GB RAM
gp.5.2.2    # 2 vCPU, 2GB RAM
gp.5.4.8    # 4 vCPU, 8GB RAM
gp.5.4.16   # 4 vCPU, 16GB RAM
```

**Recommended flavors:**
- Bastion: `gp.5.2.2` (2 vCPU, 2GB RAM)
- Master nodes: `gp.5.4.8` (4 vCPU, 8GB RAM)
- Worker nodes: `gp.5.4.16` (4 vCPU, 16GB RAM)

### Finding Ubuntu Images

List available Ubuntu images:

```bash
openstack --os-cloud <cloud-name> image list --format json | jq -r '.[] | select(.Name | contains("Ubuntu")) | "\(.ID) \(.Name)"'
```

For Ubuntu 24.04 specifically:

```bash
openstack --os-cloud <cloud-name> image list --format json | jq -r '.[] | select(.Name | contains("Ubuntu 24.04")) | "\(.ID) \(.Name)"'
```

**Example for DFW3:**
```
ec458631-309a-4b7d-846c-cd2ccc601137 Ubuntu 24.04
```

### Finding External Network ID

List networks to find the external/public network:

```bash
openstack --os-cloud <cloud-name> network list --format json | jq -r '.[] | select(.["Router Type"] == "External") | "\(.ID) \(.Name)"'
```

Or search by common names:

```bash
openstack --os-cloud <cloud-name> network list --format json | jq -r '.[] | select(.Name | test("PUBLIC|EXTERNAL|public|external"; "i")) | "\(.ID) \(.Name)"'
```

**Example for DFW3:**
```
82be3711-cd97-4f7c-8bbd-59f5524a949e PUBLICNET
```

### Verifying Region-Specific Resources

Different OpenStack regions may have different resource IDs and naming conventions. Always verify resources for your target region:

```bash
# Check your current region
openstack --os-cloud <cloud-name> configuration show | grep region

# List all available regions
openstack --os-cloud <cloud-name> region list
```

## Deploying a Cluster

### Step 1: Create Cluster Configuration

Create a YAML configuration file for your cluster (e.g., `my-cluster-config.yaml`):

```yaml
opencenter:
  meta:
    name: my-cluster
    organization: my-org
    env: prod
    region: dfw3
  
  infrastructure:
    provider: openstack
    os_version: "24"
    ssh_user: ubuntu
    
    cloud:
      openstack:
        auth_url: https://keystone.api.dfw3.rackspacecloud.com/v3/
        region: DFW3
        availability_zone: az1
        domain: rackspace_cloud_domain
        project_domain_name: rackspace_cloud_domain
        user_domain_name: rackspace_cloud_domain
        image_id: ec458631-309a-4b7d-846c-cd2ccc601137  # Ubuntu 24.04 for DFW3
        insecure: false
        
        networking:
          router_external_network_id: 82be3711-cd97-4f7c-8bbd-59f5524a949e  # PUBLICNET for DFW3
          floating_ip_pool: PUBLICNET
  
  cluster:
    cluster_name: my-cluster
    base_domain: k8s.example.com
    
    kubernetes:
      version: 1.34.3
      kubespray_version: v2.29.1
      master_count: 3
      worker_count: 3
      
      flavor_bastion: gp.5.2.2
      flavor_master: gp.5.4.8
      flavor_worker: gp.5.4.16
      
      subnet_pods: 10.42.0.0/16
      subnet_services: 10.43.0.0/16
      
      network_plugin:
        calico:
          enabled: true
          cni_iface: enp3s0
          encapsulation_type: VXLAN
          nat_outgoing: true
    
    networking:
      dns_nameservers:
        - 8.8.8.8
        - 8.8.4.4
      ntp_servers:
        - time.dfw3.rackspace.com
        - time2.dfw3.rackspace.com
      subnet_nodes: 10.2.128.0/22

networking:
  subnet_nodes: 10.2.128.0/22

opentofu:
  enabled: true
  backend:
    type: local
    local:
      path: ./terraform.tfstate
```

### Step 2: Initialize the Cluster

Initialize the cluster configuration, which generates SSH keys, SOPS keys, and GitOps structure:

```bash
./bin/openCenter cluster init my-cluster --config my-cluster-config.yaml
```

This creates:
- Cluster configuration at `~/.config/openCenter/clusters/<organization>/.my-cluster-config.yaml`
- SSH keypair at `~/.config/openCenter/clusters/<organization>/secrets/ssh/`
- SOPS Age key at `~/.config/openCenter/clusters/<organization>/secrets/age/keys/`
- GitOps directory structure at `~/.config/openCenter/clusters/<organization>/`

### Step 3: Validate Configuration

Validate the cluster configuration:

```bash
./bin/openCenter cluster validate my-cluster
```

Run preflight checks to verify OpenStack connectivity and resources:

```bash
./bin/openCenter cluster preflight my-cluster
```

### Step 4: Render GitOps Templates

Generate the GitOps repository with Terraform and Kubernetes manifests:

```bash
./bin/openCenter cluster render my-cluster
```

This creates:
- Terraform configurations in `infrastructure/clusters/my-cluster/`
- Kubernetes manifests in `applications/overlays/my-cluster/`

### Step 5: Configure OpenStack Credentials

Create a `terraform.tfvars` file with your OpenStack credentials:

```bash
cat > ~/.config/openCenter/clusters/<organization>/infrastructure/clusters/my-cluster/terraform.tfvars << 'EOF'
os_application_credential_id = "your-credential-id"
os_application_credential_secret = "your-credential-secret"
EOF
```

**Note:** Get these credentials from your `~/.config/openstack/clouds.yaml` file.

### Step 6: Bootstrap the Cluster

Deploy the infrastructure and install Kubernetes:

```bash
mise exec -- ./bin/openCenter cluster bootstrap my-cluster
```

The bootstrap process:
1. Runs `make terraform` to prepare Terraform modules
2. Initializes Terraform (`terraform init`)
3. Applies Terraform configuration to provision infrastructure:
   - Network, subnet, router
   - Security groups
   - Floating IPs
   - SSH keypair
   - Bastion host (~25 minutes)
   - Master nodes (3x VMs)
   - Worker nodes (3x VMs with volumes)
4. Waits for cloud-init to complete on all nodes
5. Runs Kubespray Ansible playbook to install Kubernetes (~15-30 minutes)
6. Copies kubeconfig for cluster access

**Total deployment time:** 45-60 minutes

### Step 7: Access the Cluster

Once bootstrap completes, activate the cluster environment. openCenter detects your shell and provides the correct syntax.

**Bash/Zsh:**
```bash
eval $(openCenter cluster select my-cluster --activate --export-only)
```

**Fish:**
```fish
openCenter cluster select my-cluster --activate --export-only | source
```

**PowerShell:**
```powershell
openCenter cluster select my-cluster --activate --export-only | Invoke-Expression
```

**Override shell detection:**
```bash
# Force Fish syntax even if running in bash
openCenter cluster select my-cluster --activate --export-only --shell fish | source
```

This configures:
- `KUBECONFIG` pointing to your cluster
- `OPENCENTER_ACTIVE_CLUSTER` set to the cluster name
- `PATH` extended with cluster bin directory
- Cloud provider credentials (AWS/OpenStack)

Verify access:

```bash
kubectl get nodes
kubectl get pods -A
```

## Troubleshooting Deployment

### Common Issues

**Issue: Flavor not found**
```
Error: Can not find requested flavor
```
**Solution:** Verify flavors exist in your region using the discovery commands above. Update your config with correct flavor names.

**Issue: Image not found**
```
Error: Can not find requested image
```
**Solution:** Find the correct Ubuntu 24.04 image ID for your region and update `image_id` in your config.

**Issue: Multiple security groups with same name**
```
Error: Multiple security_group matches found
```
**Solution:** Delete duplicate security groups:
```bash
openstack --os-cloud <cloud-name> security group list | grep <cluster-name>
openstack --os-cloud <cloud-name> security group delete <duplicate-id>
```

**Issue: Terraform interpolation errors**
```
Error: Invalid input for allocation_pools. Reason: '${local.subnet_nodes_oct}.50' is not a valid IP
```
**Solution:** This is a template rendering bug. Fix the rendered `main.tf`:
```bash
sed -i 's/\$\${\(local\.subnet_nodes_oct\)}/${\1}/g' ~/.config/openCenter/clusters/<organization>/infrastructure/clusters/<cluster-name>/main.tf
```

**Issue: Terraform not found**
```
Error: terraform: executable file not found in $PATH
```
**Solution:** Install Terraform via mise:
```bash
mise use -g terraform@latest
mise install terraform
```

### Resuming Failed Bootstrap

If bootstrap fails, you can resume from a specific step:

```bash
# Resume from terraform apply
mise exec -- ./bin/openCenter cluster bootstrap my-cluster --from-step terraform-apply

# Restart entire bootstrap
mise exec -- ./bin/openCenter cluster bootstrap my-cluster --restart
```

### Viewing Bootstrap Logs

Bootstrap logs are saved to:
```
~/.config/openCenter/clusters/<organization>/infrastructure/clusters/<cluster-name>/logs/bootstrap-YYYY-MM-DD-TIMESTAMP.log
```

## Destroying a Cluster

### Using openCenter CLI (Recommended)

Destroy the cluster and all associated resources:

```bash
./bin/openCenter cluster destroy my-cluster
```

This will:
1. Prompt for confirmation
2. Run `terraform destroy` to remove all infrastructure
3. Delete the GitOps directory
4. Remove cluster configuration

To skip confirmation:

```bash
./bin/openCenter cluster destroy my-cluster --force
```

### Manual Cleanup (If CLI Fails)

If the destroy command fails, manually clean up OpenStack resources:

```bash
# Delete all VMs
openstack --os-cloud <cloud-name> server list --format json | \
  jq -r '.[] | select(.Name | startswith("<cluster-name>-")) | .ID' | \
  while read id; do openstack --os-cloud <cloud-name> server delete "$id"; done

# Delete security groups
openstack --os-cloud <cloud-name> security group list --format json | \
  jq -r '.[] | select(.Name | startswith("<cluster-name>-")) | .ID' | \
  while read id; do openstack --os-cloud <cloud-name> security group delete "$id"; done

# Delete server groups
openstack --os-cloud <cloud-name> server group list --format json | \
  jq -r '.[] | select(.Name | startswith("<cluster-name>-")) | .ID' | \
  while read id; do openstack --os-cloud <cloud-name> server group delete "$id"; done

# Delete ports
openstack --os-cloud <cloud-name> port list --format json | \
  jq -r '.[] | select(.Name | startswith("<cluster-name>-")) | .ID' | \
  while read id; do openstack --os-cloud <cloud-name> port delete "$id"; done

# Delete router (remove subnet first)
openstack --os-cloud <cloud-name> router remove subnet <cluster-name>-router <cluster-name>-k8s
openstack --os-cloud <cloud-name> router delete <cluster-name>-router

# Delete network
openstack --os-cloud <cloud-name> network delete <cluster-name>-k8s

# Delete floating IP
openstack --os-cloud <cloud-name> floating ip list --format json | \
  jq -r '.[] | select(.Description | contains("<cluster-name>")) | .ID' | \
  while read id; do openstack --os-cloud <cloud-name> floating ip delete "$id"; done

# Delete keypair
openstack --os-cloud <cloud-name> keypair delete <cluster-name>-key
```

### Verifying Cleanup

Verify all resources are deleted:

```bash
# Check for remaining servers
openstack --os-cloud <cloud-name> server list | grep <cluster-name>

# Check for remaining networks
openstack --os-cloud <cloud-name> network list | grep <cluster-name>

# Check for remaining security groups
openstack --os-cloud <cloud-name> security group list | grep <cluster-name>
```

## Region-Specific Examples

### DFW3 (Dallas)

```yaml
infrastructure:
  cloud:
    openstack:
      auth_url: https://keystone.api.dfw3.rackspacecloud.com/v3/
      region: DFW3
      image_id: ec458631-309a-4b7d-846c-cd2ccc601137
      networking:
        router_external_network_id: 82be3711-cd97-4f7c-8bbd-59f5524a949e
        floating_ip_pool: PUBLICNET

cluster:
  kubernetes:
    flavor_bastion: gp.5.2.2
    flavor_master: gp.5.4.8
    flavor_worker: gp.5.4.16
  
  networking:
    ntp_servers:
      - time.dfw3.rackspace.com
      - time2.dfw3.rackspace.com
```

### SJC3 (San Jose)

```yaml
infrastructure:
  cloud:
    openstack:
      auth_url: https://keystone.api.sjc3.rackspacecloud.com/v3/
      region: SJC3
      image_id: <find-using-discovery-commands>
      networking:
        router_external_network_id: <find-using-discovery-commands>
        floating_ip_pool: PUBLICNET

cluster:
  networking:
    ntp_servers:
      - time.sjc3.rackspace.com
      - time2.sjc3.rackspace.com
```

## Best Practices

1. **Always validate before deploying:** Run `cluster validate` and `cluster preflight` before bootstrap
2. **Use version control:** Commit your cluster configuration to Git
3. **Document region-specific resources:** Keep a record of image IDs, network IDs, and flavors per region
4. **Test in dev first:** Deploy to a dev environment before production
5. **Monitor bootstrap progress:** Watch the logs for any errors during deployment
6. **Clean up failed deployments:** Always destroy failed clusters to avoid orphaned resources
7. **Use application credentials:** Prefer OpenStack application credentials over username/password
8. **Backup configurations:** Keep backups of your cluster configs and secrets

## Next Steps

- [Monitoring Clusters](monitoring.md)
- [Upgrading Clusters](upgrading-clusters.md)
- [Backup and Recovery](backup-recovery.md)
- [Troubleshooting](troubleshooting.md)
