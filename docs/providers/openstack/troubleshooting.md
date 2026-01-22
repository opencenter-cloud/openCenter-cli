---
title: OpenStack Troubleshooting Guide
doc_type: how-to
provider: openstack
category: troubleshooting
last_updated: 2025-01-XX
---

# OpenStack Troubleshooting Guide


## Table of Contents

- [Quick Diagnostics](#quick-diagnostics)
- [Authentication Issues](#authentication-issues)
- [Quota and Resource Issues](#quota-and-resource-issues)
- [Network Connectivity Issues](#network-connectivity-issues)
- [Image and Flavor Issues](#image-and-flavor-issues)
- [Provisioning Failures](#provisioning-failures)
- [Performance Issues](#performance-issues)
- [Configuration Validation Errors](#configuration-validation-errors)
- [Getting Help](#getting-help)
- [Related Documentation](#related-documentation)
- [Additional Resources](#additional-resources)
This guide provides solutions to common OpenStack-specific issues encountered when deploying and managing Kubernetes clusters with openCenter.

## Quick Diagnostics

### Run Preflight Checks

Before troubleshooting, run preflight checks to identify common issues:

```bash
# Run preflight checks for active cluster
mise run build
./bin/openCenter cluster preflight

# Run preflight checks for specific cluster
./bin/openCenter cluster preflight my-cluster

# Run preflight checks for organization/cluster
./bin/openCenter cluster preflight myorg/my-cluster
```

**Preflight checks verify:**

- ✅ Required CLI tools (git, kubectl, talosctl)
- ✅ OpenStack CLI availability
- ✅ Authentication URL configuration
- ✅ Environment variables (OS_* variables)

### Check Cluster Configuration

```bash
# Validate cluster configuration
./bin/openCenter cluster validate my-cluster

# View cluster configuration
cat ~/.config/openCenter/clusters/myorg/.my-cluster-config.yaml
```

## Authentication Issues

### Problem: "openstack CLI not found"

**Symptom:**
```
openstack CLI not found: please install the OpenStack client tools and configure OS_* environment variables or clouds.yaml
```

**Solution:**

1. **Install OpenStack CLI tools:**

```bash
# Using pip (recommended)
pip install python-openstackclient

# Using system package manager (Ubuntu/Debian)
sudo apt-get install python3-openstackclient

# Using system package manager (RHEL/CentOS)
sudo yum install python3-openstackclient

# Verify installation
openstack --version
```

2. **Configure authentication** (choose one method):

**Method A: Environment variables**

```bash
export OS_AUTH_URL="https://identity.example.com/v3"
export OS_PROJECT_NAME="my-project"
export OS_USERNAME="my-username"
export OS_PASSWORD="my-password"
export OS_USER_DOMAIN_NAME="Default"
export OS_PROJECT_DOMAIN_NAME="Default"
export OS_IDENTITY_API_VERSION="3"
export OS_INTERFACE="public"
```

**Method B: Application credentials (recommended)**

```bash
export OS_AUTH_URL="https://identity.example.com/v3"
export OS_APPLICATION_CREDENTIAL_ID="abc123..."
export OS_APPLICATION_CREDENTIAL_SECRET="secret123..."
export OS_AUTH_TYPE="v3applicationcredential"
```

**Method C: clouds.yaml file**

Create `~/.config/openstack/clouds.yaml`:

```yaml
clouds:
  mycloud:
    auth:
      auth_url: https://identity.example.com/v3
      application_credential_id: "abc123..."
      application_credential_secret: "secret123..."
    region_name: RegionOne
    interface: public
    identity_api_version: 3
```

Then use:
```bash
export OS_CLOUD=mycloud
```

3. **Test authentication:**

```bash
openstack token issue
openstack server list
```

### Problem: "auth_url is empty; authentication may fail"

**Symptom:**
```
cloud.openstack.auth_url is empty; authentication may fail
```

**Solution:**

Update your cluster configuration file:

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        # Add authentication URL
        auth_url: "https://identity.example.com/v3"
        region: "RegionOne"
```

**Find your auth URL:**

```bash
# From environment
echo $OS_AUTH_URL

# From clouds.yaml
grep auth_url ~/.config/openstack/clouds.yaml

# From OpenStack dashboard
# Look for "Identity" or "Keystone" endpoint in API Access section
```

### Problem: Authentication fails with "Unauthorized"

**Symptom:**
```
Error: 401 Unauthorized
The request you have made requires authentication.
```

**Possible causes and solutions:**

1. **Expired credentials:**
   ```bash
   # Re-authenticate
   unset OS_TOKEN
   openstack token issue
   ```

2. **Wrong domain:**
   ```yaml
   opencenter:
     infrastructure:
       cloud:
         openstack:
           # Ensure correct domain names
           user_domain_name: "rackspace_cloud_domain"
           project_domain_name: "rackspace_cloud_domain"
   ```

3. **Application credential issues:**
   ```bash
   # Verify application credential is valid
   openstack application credential show <credential-id>
   
   # Create new application credential if needed
   openstack application credential create my-cluster-cred \
     --role member \
     --expiration "2025-12-31T23:59:59"
   ```

4. **Password expired:**
   - Log into OpenStack dashboard
   - Update password
   - Update configuration with new credentials

### Problem: SSL/TLS Certificate Errors

**Symptom:**
```
Error: SSL certificate verify failed
certificate verify failed: unable to get local issuer certificate
```

**Solutions:**

**Option 1: Add CA certificate (recommended for production)**

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        # Path to CA certificate file
        ca: "/path/to/ca-bundle.crt"
```

Or set environment variable:
```bash
export OS_CACERT="/path/to/ca-bundle.crt"
```

**Option 2: Disable SSL verification (NOT recommended for production)**

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        # Only use for testing/development
        insecure: true
```

Or set environment variable:
```bash
export OS_INSECURE=true
```

**Get CA certificate:**

```bash
# Download from OpenStack endpoint
openssl s_client -showcerts -connect identity.example.com:443 \
  </dev/null 2>/dev/null | openssl x509 -outform PEM > ca-cert.pem

# Or get from system administrator
```

## Quota and Resource Issues

### Problem: Quota Exceeded

**Symptom:**
```
Error: Quota exceeded for resources
Requested: instances=5, cores=20, ram=40960
Available: instances=3, cores=12, ram=24576
```

**Solution:**

1. **Check current quota:**

```bash
# View compute quota
openstack quota show

# View detailed usage
openstack quota show --usage

# View network quota
openstack quota show --network
```

2. **Request quota increase:**

Contact your OpenStack administrator with:
- Current quota limits
- Requested quota limits
- Business justification
- Expected timeline

3. **Optimize resource usage:**

```yaml
opencenter:
  cluster:
    kubernetes:
      # Reduce node count
      master_count: 3  # Instead of 5
      worker_count: 2  # Instead of 5
      
      # Use smaller flavors
      flavor_master: "gp.0.2.4"   # Instead of gp.0.4.8
      flavor_worker: "gp.0.2.8"   # Instead of gp.0.4.16
```

4. **Clean up unused resources:**

```bash
# List all instances
openstack server list --all-projects

# Delete old instances
openstack server delete <instance-id>

# List floating IPs
openstack floating ip list

# Delete unused floating IPs
openstack floating ip delete <floating-ip-id>

# List volumes
openstack volume list

# Delete unused volumes
openstack volume delete <volume-id>
```

### Problem: Insufficient Floating IPs

**Symptom:**
```
Error: No more floating IPs available in pool
```

**Solution:**

1. **Check floating IP availability:**

```bash
# List floating IPs
openstack floating ip list

# Check quota
openstack quota show --network | grep floating
```

2. **Release unused floating IPs:**

```bash
# Find unassociated floating IPs
openstack floating ip list --status DOWN

# Delete unused floating IPs
openstack floating ip delete <floating-ip-id>
```

3. **Request additional floating IPs:**

Contact your OpenStack administrator to increase floating IP quota.

4. **Use fewer floating IPs:**

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        # Disable bastion to save one floating IP
        disable_bastion: true
```

### Problem: Volume Creation Fails

**Symptom:**
```
Error: Volume creation failed
No valid host was found. There are not enough hosts available.
```

**Possible causes and solutions:**

1. **Volume quota exceeded:**
   ```bash
   # Check volume quota
   openstack quota show | grep volumes
   openstack quota show | grep gigabytes
   ```

2. **Volume type not available:**
   ```yaml
   opencenter:
     storage:
       # Use available volume type
       worker_volume_type: "HA-Standard"  # Instead of "HA-Performance"
   ```
   
   ```bash
   # List available volume types
   openstack volume type list
   ```

3. **Insufficient storage capacity:**
   - Contact administrator to add storage capacity
   - Or reduce volume sizes:
   ```yaml
   opencenter:
     storage:
       worker_volume_size: 40  # Instead of 100
   ```

## Network Connectivity Issues

### Problem: Cannot Reach Instances

**Symptom:**
- SSH connection times out
- Cannot ping instances
- Instances have no network connectivity

**Diagnostic steps:**

1. **Verify instance status:**

```bash
# Check instance status
openstack server show <instance-name>

# Check console log for errors
openstack console log show <instance-name>

# Access console (if available)
openstack console url show <instance-name>
```

2. **Check network configuration:**

```bash
# Verify network exists
openstack network show <network-id>

# Verify subnet configuration
openstack subnet show <subnet-id>

# Check router configuration
openstack router show <router-name>

# Verify router has gateway set
openstack router show <router-name> -f json | jq '.external_gateway_info'

# Check router ports
openstack port list --router <router-name>
```

3. **Verify security groups:**

```bash
# List security groups
openstack security group list

# Show security group rules
openstack security group show <security-group-name>

# Check if SSH (port 22) is allowed
openstack security group rule list <security-group-name> | grep 22
```

**Solutions:**

**Add missing security group rules:**

```bash
# Allow SSH from specific CIDR
openstack security group rule create \
  --protocol tcp \
  --dst-port 22 \
  --remote-ip 203.0.113.0/24 \
  <security-group-name>

# Allow ICMP (ping)
openstack security group rule create \
  --protocol icmp \
  <security-group-name>

# Allow Kubernetes API (port 6443)
openstack security group rule create \
  --protocol tcp \
  --dst-port 6443 \
  --remote-ip 0.0.0.0/0 \
  <security-group-name>
```

**Fix router configuration:**

```bash
# Set external gateway
openstack router set \
  --external-gateway <external-network-id> \
  <router-name>

# Add subnet to router
openstack router add subnet <router-name> <subnet-id>
```

**Verify floating IP association:**

```bash
# List floating IPs
openstack floating ip list

# Associate floating IP to instance
openstack server add floating ip <instance-name> <floating-ip>
```

### Problem: DNS Resolution Fails

**Symptom:**
- Cannot resolve domain names from instances
- `nslookup` or `dig` commands fail

**Solution:**

1. **Check DNS configuration in subnet:**

```bash
# View subnet DNS servers
openstack subnet show <subnet-id> -f json | jq '.dns_nameservers'
```

2. **Update subnet DNS servers:**

```bash
# Set DNS servers
openstack subnet set \
  --dns-nameserver 8.8.8.8 \
  --dns-nameserver 8.8.4.4 \
  <subnet-id>
```

3. **Configure DNS in cluster config:**

```yaml
opencenter:
  cluster:
    networking:
      dns_nameservers:
        - "8.8.8.8"
        - "8.8.4.4"
        # Or use internal DNS servers
        # - "10.0.0.10"
        # - "10.0.0.11"
```

4. **Test DNS resolution:**

```bash
# From instance
ssh ubuntu@<instance-ip>
nslookup google.com
dig google.com
```

### Problem: Load Balancer Creation Fails

**Symptom:**
```
Error: Load balancer creation failed
Octavia service is not available
```

**Solutions:**

1. **Verify Octavia is available:**

```bash
# Check if Octavia service exists
openstack service list | grep octavia

# List load balancers
openstack loadbalancer list
```

2. **Use alternative load balancer provider:**

```yaml
opencenter:
  cluster:
    kubernetes:
      # Switch to OVN-based load balancing
      loadbalancer_provider: "ovn"
      
      networking:
        # Disable Octavia
        use_octavia: false
```

3. **Use VRRP for API HA:**

```yaml
networking:
  # Enable VRRP instead of Octavia
  vrrp_enabled: true
  vrrp_ip: "10.0.1.100"  # Must be in node subnet

opencenter:
  cluster:
    kubernetes:
      networking:
        use_octavia: false
```

4. **Check Octavia quota:**

```bash
# Check load balancer quota
openstack quota show --network | grep loadbalancer
```

### Problem: Floating IP Pool Not Found

**Symptom:**
```
Error: External network 'PUBLICNET' not found
```

**Solution:**

1. **List available networks:**

```bash
# List all networks
openstack network list

# List external networks only
openstack network list --external
```

2. **Update configuration with correct network:**

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        networking:
          # Use network name
          floating_ip_pool: "public"
          
          # Or use network UUID
          floating_network_id: "723f8fa2-dbf7-4cec-8d5f-017e62c12f79"
```

3. **Verify network is external:**

```bash
# Check network details
openstack network show <network-name> -f json | jq '.router:external'
# Should return: true
```

## Image and Flavor Issues

### Problem: Image Not Found

**Symptom:**
```
Error: Image not found: 799dcf97-3656-4361-8187-13ab1b295e33
```

**Solution:**

1. **List available images:**

```bash
# List all images
openstack image list

# Search for Ubuntu images
openstack image list | grep -i ubuntu

# Search for specific version
openstack image list | grep "Ubuntu 24.04"
```

2. **Update configuration with correct image:**

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        # Use correct image ID
        image_id: "new-image-uuid"
        
        # For Windows nodes
        image_id_windows: "windows-image-uuid"
```

3. **Verify image is active:**

```bash
# Check image status
openstack image show <image-id> -f json | jq '.status'
# Should return: "active"
```

4. **Check image visibility:**

```bash
# Check if image is public or shared
openstack image show <image-id> -f json | jq '.visibility'
```

### Problem: Flavor Not Available

**Symptom:**
```
Error: Flavor 'gp.0.4.8' not found
```

**Solution:**

1. **List available flavors:**

```bash
# List all flavors
openstack flavor list

# Show flavor details
openstack flavor show <flavor-name>
```

2. **Update configuration with available flavor:**

```yaml
opencenter:
  cluster:
    kubernetes:
      # Use available flavors
      flavor_master: "m1.medium"
      flavor_worker: "m1.large"
      flavor_bastion: "m1.small"
```

3. **Check flavor quota:**

```bash
# Verify you have quota for the flavor
openstack quota show | grep cores
openstack quota show | grep ram
```

## Provisioning Failures

### Problem: Instance Stuck in BUILD Status

**Symptom:**
- Instance remains in BUILD status for extended period
- Provisioning never completes

**Diagnostic steps:**

```bash
# Check instance status
openstack server show <instance-name>

# Check console log for errors
openstack console log show <instance-name> | tail -50

# Check instance fault (if any)
openstack server show <instance-name> -f json | jq '.fault'
```

**Common causes and solutions:**

1. **Insufficient resources:**
   - Check compute node capacity
   - Try different availability zone
   - Use smaller flavor

2. **Image issues:**
   - Verify image is not corrupted
   - Try different image
   - Check image format (qcow2, raw, etc.)

3. **Network issues:**
   - Verify network and subnet exist
   - Check DHCP agent is running
   - Verify port creation succeeded

4. **Timeout:**
   ```bash
   # Delete and recreate instance
   openstack server delete <instance-name>
   # Re-run cluster setup
   ```

### Problem: Cloud-Init Fails

**Symptom:**
- Instance boots but cloud-init fails
- SSH keys not injected
- User data not applied

**Diagnostic steps:**

```bash
# Check cloud-init logs
ssh ubuntu@<instance-ip>
sudo cat /var/log/cloud-init.log
sudo cat /var/log/cloud-init-output.log

# Check cloud-init status
cloud-init status --long
```

**Solutions:**

1. **Verify metadata service:**
   ```bash
   # From instance
   curl http://169.254.169.254/latest/meta-data/
   ```

2. **Check user data:**
   ```bash
   # View user data
   openstack server show <instance-name> -f json | jq '.user_data'
   ```

3. **Verify SSH key:**
   ```yaml
   opencenter:
     cluster:
       ssh_authorized_keys:
         - "ssh-rsa AAAAB3NzaC1yc2E... user@host"
   ```

4. **Re-run cloud-init:**
   ```bash
   # From instance
   sudo cloud-init clean
   sudo cloud-init init
   sudo cloud-init modules --mode=config
   sudo cloud-init modules --mode=final
   ```

### Problem: Terraform/OpenTofu Errors

**Symptom:**
```
Error: Error creating OpenStack server
```

**Diagnostic steps:**

1. **Check Terraform state:**
   ```bash
   # View state
   cd <gitops-repo>/infrastructure/clusters/<cluster-name>
   terraform state list
   
   # Show specific resource
   terraform state show <resource-name>
   ```

2. **Enable debug logging:**
   ```bash
   export TF_LOG=DEBUG
   export TF_LOG_PATH=./terraform-debug.log
   terraform apply
   ```

3. **Validate configuration:**
   ```bash
   terraform validate
   terraform plan
   ```

**Solutions:**

1. **Refresh state:**
   ```bash
   terraform refresh
   ```

2. **Import existing resources:**
   ```bash
   # Import instance
   terraform import openstack_compute_instance_v2.master <instance-id>
   ```

3. **Remove corrupted state:**
   ```bash
   # Remove resource from state
   terraform state rm <resource-name>
   
   # Re-import or recreate
   terraform import <resource-name> <resource-id>
   ```

4. **Clean and retry:**
   ```bash
   # Remove lock file
   rm .terraform.lock.hcl
   
   # Re-initialize
   terraform init
   terraform plan
   terraform apply
   ```

## Performance Issues

### Problem: Slow Instance Creation

**Possible causes:**

1. **Image download time:**
   - Use smaller images
   - Pre-cache images on compute nodes
   - Use local image mirror

2. **Volume creation time:**
   - Use faster volume type (SSD instead of HDD)
   - Reduce volume size if possible
   - Check storage backend performance

3. **Network configuration:**
   - Simplify network topology
   - Reduce number of security group rules
   - Check Neutron agent performance

### Problem: Network Performance Issues

**Diagnostic steps:**

```bash
# Test network throughput between nodes
kubectl run -it --rm iperf-server --image=networkstatic/iperf3 -- -s

# From another terminal
kubectl run -it --rm iperf-client --image=networkstatic/iperf3 -- \
  -c <iperf-server-ip> -t 30
```

**Solutions:**

1. **Adjust MTU:**
   ```yaml
   opencenter:
     infrastructure:
       cloud:
         openstack:
           networking:
             vlan:
               mtu: 1450  # Reduce for encapsulation overhead
   ```

2. **Change encapsulation:**
   ```yaml
   opencenter:
     cluster:
       kubernetes:
         network_plugin:
           calico:
             # Try different encapsulation
             encapsulation_type: "IPIP"  # Or "None" for best performance
   ```

3. **Enable jumbo frames (if supported):**
   ```yaml
   opencenter:
     infrastructure:
       cloud:
         openstack:
           networking:
             vlan:
               mtu: 9000
   ```

## Configuration Validation Errors

### Problem: Schema Validation Fails

**Symptom:**
```
Error: Configuration validation failed
Field 'opencenter.infrastructure.cloud.openstack.auth_url' is required
```

**Solution:**

1. **Run validation:**
   ```bash
   ./bin/openCenter cluster validate my-cluster
   ```

2. **Check required fields:**
   - `auth_url`: OpenStack authentication URL
   - `region`: OpenStack region name
   - `application_credential_id` and `application_credential_secret`: Authentication credentials

3. **Fix configuration:**
   ```yaml
   opencenter:
     infrastructure:
       cloud:
         openstack:
           auth_url: "https://identity.example.com/v3"
           region: "RegionOne"
           application_credential_id: "abc123..."
           application_credential_secret: "secret123..."
   ```

### Problem: Network CIDR Overlap

**Symptom:**
```
Error: Pod network overlaps with service network
```

**Solution:**

Ensure networks don't overlap:

```yaml
opencenter:
  cluster:
    kubernetes:
      # Pod network
      subnet_pods: "10.42.0.0/16"
      
      # Service network (must not overlap)
      subnet_services: "10.43.0.0/16"
      
      networking:
        # Node network (must not overlap with pods or services)
        subnet_nodes: "10.0.1.0/24"
```

**Common non-overlapping ranges:**
- Pods: `10.42.0.0/16`
- Services: `10.43.0.0/16`
- Nodes: `10.0.0.0/16` or `192.168.0.0/16`

## Getting Help

### Collect Diagnostic Information

When reporting issues, collect:

1. **Preflight check output:**
   ```bash
   ./bin/openCenter cluster preflight my-cluster > preflight.log 2>&1
   ```

2. **Validation output:**
   ```bash
   ./bin/openCenter cluster validate my-cluster > validation.log 2>&1
   ```

3. **OpenStack environment:**
   ```bash
   openstack --version > openstack-info.log
   openstack token issue >> openstack-info.log 2>&1
   openstack quota show >> openstack-info.log 2>&1
   ```

4. **Cluster configuration (sanitized):**
   ```bash
   # Remove sensitive data before sharing
   cat ~/.config/openCenter/clusters/myorg/.my-cluster-config.yaml | \
     sed 's/application_credential_secret:.*/application_credential_secret: REDACTED/' | \
     sed 's/password:.*/password: REDACTED/'
   ```

5. **Terraform/OpenTofu logs:**
   ```bash
   cd <gitops-repo>/infrastructure/clusters/<cluster-name>
   terraform show > terraform-state.log
   ```

### Enable Debug Logging

```bash
# Enable OpenStack client debug logging
export OS_DEBUG=1

# Enable Terraform debug logging
export TF_LOG=DEBUG
export TF_LOG_PATH=./terraform-debug.log

# Run commands with verbose output
./bin/openCenter cluster setup my-cluster --verbose
```

### Common Log Locations

- **openCenter logs**: `./openCenter.log` (if enabled)
- **Terraform logs**: `<gitops-repo>/infrastructure/clusters/<cluster-name>/terraform-debug.log`
- **Cloud-init logs**: `/var/log/cloud-init.log` (on instances)
- **Kubernetes logs**: `kubectl logs -n kube-system <pod-name>`
- **OpenStack logs**: Varies by deployment (ask administrator)

## Related Documentation

- [OpenStack Networking Guide](./networking.md)
- [OpenStack Getting Started](./getting-started.md)
- [Configuration Reference](../../reference/configuration.md)
- [Security Best Practices](../../reference/security.md)

## Additional Resources

- [OpenStack Documentation](https://docs.openstack.org/)
- [OpenStack CLI Reference](https://docs.openstack.org/python-openstackclient/latest/)
- [Terraform OpenStack Provider](https://registry.terraform.io/providers/terraform-provider-openstack/openstack/latest/docs)
- [OpenStack Community Support](https://ask.openstack.org/)
