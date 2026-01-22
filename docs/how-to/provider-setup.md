# Provider Setup


## Table of Contents

- [Task Summary](#task-summary)
- [Prerequisites](#prerequisites)
- [OpenStack Setup](#openstack-setup)
- [AWS Setup](#aws-setup)
- [Kind Setup](#kind-setup)
- [Talos Setup](#talos-setup)
- [Multi-Provider Considerations](#multi-provider-considerations)
- [Next Steps](#next-steps)
- [Related Documentation](#related-documentation)
- [External Resources](#external-resources)
**doc_type: how-to**

Configure cloud provider credentials, networking, and permissions for openCenter cluster deployments.

## Task Summary

Set up authentication and infrastructure prerequisites for your chosen cloud provider. This guide covers credential configuration, network setup, and validation for OpenStack, AWS, Kind, and Talos providers.

## Prerequisites

- openCenter CLI installed (`mise run build`)
- Cloud provider account with appropriate permissions
- Network access to provider API endpoints
- For OpenStack: OpenStack CLI tools (`python3-openstackclient`)
- For AWS: AWS CLI v2
- For Kind: Docker or Podman
- For Talos: Pulumi CLI

## OpenStack Setup

### 1. Create Application Credentials

Application credentials are the recommended authentication method. They provide scoped access without exposing your user password.

**Create credentials via CLI:**

```bash
openstack application credential create \
  --description "openCenter cluster management" \
  --role member \
  opencenter-app-cred
```

Save the output. You'll need the `id` and `secret` values.

**Create credentials via Horizon dashboard:**

1. Navigate to Identity → Application Credentials
2. Click "Create Application Credential"
3. Name: `opencenter-app-cred`
4. Description: `openCenter cluster management`
5. Roles: Select `member` (or appropriate role for your project)
6. Click "Create Application Credential"
7. Download the credentials file or copy the ID and secret

**Expected outcome:** You have an application credential ID and secret that can create compute instances, networks, and security groups.

### 2. Configure OpenStack Authentication

Add credentials to your cluster configuration file:

```yaml
infrastructure:
  provider: openstack
  cloud:
    openstack:
      auth_url: https://openstack.example.com:5000/v3
      region: RegionOne
      application_credential_id: <your-app-cred-id>
      application_credential_secret: <your-app-cred-secret>
      tenant_name: my-project
      domain: Default
```

**Alternative: Environment variables**

Export credentials for CLI operations:

```bash
export OS_AUTH_URL="https://openstack.example.com:5000/v3"
export OS_REGION_NAME="RegionOne"
export OS_APPLICATION_CREDENTIAL_ID="<your-app-cred-id>"
export OS_APPLICATION_CREDENTIAL_SECRET="<your-app-cred-secret>"
```

Or use the openCenter credentials helper:

```bash
openCenter cluster credentials export --provider openstack --format env
eval $(openCenter cluster credentials export --provider openstack --format env)
```

**Alternative: clouds.yaml file**

Create `~/.config/openstack/clouds.yaml`:

```yaml
clouds:
  opencenter:
    auth:
      auth_url: https://openstack.example.com:5000/v3
      application_credential_id: <your-app-cred-id>
      application_credential_secret: <your-app-cred-secret>
    region_name: RegionOne
```

Then reference it in your configuration:

```yaml
infrastructure:
  cloud:
    openstack:
      auth_url: https://openstack.example.com:5000/v3
      region: RegionOne
```

### 3. Configure OpenStack Networking

Identify your network resources:

```bash
# List available networks
openstack network list

# List floating IP pools
openstack floating ip pool list

# List availability zones
openstack availability zone list
```

Add network configuration to your cluster file:

```yaml
infrastructure:
  cloud:
    openstack:
      networking:
        # External network for floating IPs
        floating_ip_pool: public
        floating_network_id: <external-network-id>
        
        # Optional: Use existing network
        network_id: <your-network-id>
        subnet_id: <your-subnet-id>
        
        # Optional: Router for external connectivity
        router_external_network_id: <external-network-id>
        
        # Optional: DNS zone for cluster DNS
        designate:
          dns_zone_name: cluster.example.com
```

**If creating a new network**, omit `network_id` and `subnet_id`. openCenter will create them.

**Expected outcome:** Your cluster can reach the internet and expose services via floating IPs.

### 4. Configure Compute Resources

Specify images and availability zones:

```yaml
infrastructure:
  cloud:
    openstack:
      # Ubuntu 22.04 image for cluster nodes
      image_id: <ubuntu-22.04-image-id>
      
      # Optional: Windows worker nodes
      image_id_windows: <windows-server-image-id>
      
      # Availability zone for node placement
      availability_zone: nova
```

Find available images:

```bash
openstack image list --long | grep -i ubuntu
```

### 5. Verify Required Permissions

Test that your credentials have the necessary permissions:

```bash
# Test compute access
openstack server list

# Test network access
openstack network list

# Test security group access
openstack security group list

# Test floating IP access
openstack floating ip list
```

**Required permissions:**
- Create/delete compute instances
- Create/delete networks and subnets
- Create/delete security groups and rules
- Allocate/release floating IPs
- Create/delete volumes (for persistent storage)
- Create/delete load balancers (if using Octavia)

### 6. Validate OpenStack Configuration

Run preflight checks:

```bash
openCenter cluster validate <cluster-name>
```

The validator checks:
- OpenStack CLI availability
- Authentication URL configuration
- API connectivity
- Required quota availability
- Network resource existence

**Expected outcome:** Validation passes with no errors. Warnings about quota are informational.

### 7. OpenStack Security Best Practices

**Use application credentials instead of user passwords:**
- Scoped to specific projects
- Can be revoked without changing user password
- Support role-based access control

**Rotate credentials regularly:**

```bash
# Create new credential
openstack application credential create opencenter-app-cred-2

# Update cluster configuration with new credential
# Test cluster operations
# Delete old credential
openstack application credential delete opencenter-app-cred
```

**Use least privilege:**
- Grant only `member` role, not `admin`
- Scope credentials to specific projects
- Set expiration dates for temporary access

**Protect credential files:**

```bash
chmod 600 ~/.config/openstack/clouds.yaml
chmod 600 ~/.config/openCenter/clusters/<org>/<cluster>/.<cluster>-config.yaml
```

### Common OpenStack Issues

**Issue: "Authentication failed"**

Check that:
- `auth_url` includes `/v3` suffix
- Application credential ID and secret are correct
- Credentials haven't expired
- Project/tenant name matches your OpenStack project

**Issue: "Network not found"**

Verify network exists:

```bash
openstack network show <network-id>
```

If using network name instead of ID, ensure it's unique in your project.

**Issue: "Quota exceeded"**

Check current quota usage:

```bash
openstack quota show
openstack server list
openstack volume list
```

Request quota increase from your OpenStack administrator.

**Issue: "Floating IP allocation failed"**

Verify floating IP pool exists and has available IPs:

```bash
openstack floating ip pool list
openstack floating ip list --status DOWN
```

---

## AWS Setup

**Status:** In development. Configuration may change.

### 1. Create IAM User or Role

Create an IAM user with programmatic access:

**Via AWS Console:**

1. Navigate to IAM → Users
2. Click "Add users"
3. Username: `opencenter-deployer`
4. Access type: "Programmatic access"
5. Attach policies: `AmazonEC2FullAccess`, `AmazonVPCFullAccess`
6. Create user and save access key ID and secret

**Via AWS CLI:**

```bash
aws iam create-user --user-name opencenter-deployer

aws iam attach-user-policy \
  --user-name opencenter-deployer \
  --policy-arn arn:aws:iam::aws:policy/AmazonEC2FullAccess

aws iam attach-user-policy \
  --user-name opencenter-deployer \
  --policy-arn arn:aws:iam::aws:policy/AmazonVPCFullAccess

aws iam create-access-key --user-name opencenter-deployer
```

**Expected outcome:** You have an AWS access key ID and secret access key.

### 2. Configure AWS Credentials

**Option 1: AWS CLI configuration**

```bash
aws configure --profile opencenter
# Enter access key ID
# Enter secret access key
# Enter default region (e.g., us-east-1)
# Enter output format (json)
```

**Option 2: Environment variables**

```bash
export AWS_ACCESS_KEY_ID="<your-access-key-id>"
export AWS_SECRET_ACCESS_KEY="<your-secret-access-key>"
export AWS_DEFAULT_REGION="us-east-1"
```

**Option 3: Cluster configuration file**

```yaml
infrastructure:
  provider: aws
  cloud:
    aws:
      profile: opencenter
      region: us-east-1
```

### 3. Configure AWS Networking

**Option 1: Use existing VPC**

```yaml
infrastructure:
  cloud:
    aws:
      vpc_id: vpc-0123456789abcdef0
      private_subnets:
        - subnet-0123456789abcdef0
        - subnet-0123456789abcdef1
      public_subnets:
        - subnet-0fedcba9876543210
        - subnet-0fedcba9876543211
```

Find your VPC and subnets:

```bash
aws ec2 describe-vpcs
aws ec2 describe-subnets --filters "Name=vpc-id,Values=<vpc-id>"
```

**Option 2: Create new VPC**

Omit `vpc_id` and subnet lists. openCenter will create a new VPC with public and private subnets across multiple availability zones.

### 4. Verify AWS Permissions

Test credentials:

```bash
aws sts get-caller-identity
aws ec2 describe-instances
aws ec2 describe-vpcs
```

**Required permissions:**
- EC2: Create/delete instances, security groups, key pairs
- VPC: Create/delete VPCs, subnets, route tables, internet gateways
- ELB: Create/delete load balancers (for HA control plane)
- IAM: Create/attach instance profiles (for node IAM roles)

### 5. AWS Security Best Practices

**Use IAM roles instead of access keys when possible:**
- Attach roles to EC2 instances running openCenter
- Use AWS STS for temporary credentials
- Avoid long-lived access keys

**Enable MFA for IAM users:**

```bash
aws iam enable-mfa-device \
  --user-name opencenter-deployer \
  --serial-number arn:aws:iam::123456789012:mfa/opencenter-deployer \
  --authentication-code1 123456 \
  --authentication-code2 789012
```

**Rotate access keys regularly:**

```bash
# Create new key
aws iam create-access-key --user-name opencenter-deployer

# Update configuration with new key
# Test operations
# Delete old key
aws iam delete-access-key --user-name opencenter-deployer --access-key-id <old-key-id>
```

### Common AWS Issues

**Issue: "Access denied"**

Verify IAM permissions:

```bash
aws iam get-user-policy --user-name opencenter-deployer --policy-name <policy-name>
```

Ensure policies are attached correctly.

**Issue: "VPC limit exceeded"**

Check VPC quota:

```bash
aws service-quotas get-service-quota \
  --service-code vpc \
  --quota-code L-F678F1CE
```

Request quota increase via AWS Support.

**Issue: "Subnet has no available IP addresses"**

Choose larger subnet CIDR blocks or use additional subnets.

---

## Kind Setup

Kind runs Kubernetes clusters in Docker containers. It's designed for local development and testing.

### 1. Install Docker or Podman

**Docker (recommended):**

```bash
# macOS
brew install docker

# Linux
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
```

**Podman (alternative):**

```bash
# macOS
brew install podman

# Linux
sudo apt-get install podman  # Debian/Ubuntu
sudo dnf install podman      # Fedora/RHEL
```

Verify installation:

```bash
docker version
# or
podman version
```

**Expected outcome:** Docker or Podman is running and accessible without sudo.

### 2. Configure Kind Cluster

Create a minimal configuration:

```yaml
infrastructure:
  provider: kind

cluster:
  name: dev-cluster
  kubernetes_version: "1.30.4"
```

**Optional: Customize Kind settings**

```yaml
infrastructure:
  provider: kind

# Kind-specific settings (optional)
kind:
  # Number of worker nodes
  workers: 2
  
  # Enable local registry
  registry_enabled: true
  registry_port: 5001
  
  # Enable ingress controller
  ingress_enabled: true
  ingress_controller: nginx
  
  # Custom networking
  service_subnet: "10.96.0.0/16"
  pod_subnet: "10.244.0.0/16"
  
  # Port mappings for host access
  port_mappings:
    - containerPort: 80
      hostPort: 8080
    - containerPort: 443
      hostPort: 8443
```

### 3. Verify Docker Resources

Ensure Docker has sufficient resources:

**Docker Desktop:**
1. Open Docker Desktop preferences
2. Resources → Advanced
3. Set CPUs: 4+ cores
4. Set Memory: 8+ GB
5. Apply & Restart

**Check current limits:**

```bash
docker info | grep -E 'CPUs|Total Memory'
```

### 4. Validate Kind Configuration

```bash
openCenter cluster validate dev-cluster
```

Checks:
- Docker/Podman availability
- Docker daemon running
- Sufficient disk space
- Network connectivity

**Expected outcome:** Validation passes. Kind clusters create quickly (2-5 minutes).

### 5. Kind Networking Considerations

**Accessing services from host:**

Kind clusters use Docker bridge networking. Services are accessible via:
- NodePort services on `localhost:<nodePort>`
- Port mappings defined in configuration
- Ingress controller (if enabled) on mapped ports

**Accessing external services from cluster:**

Kind containers can reach:
- Host services via `host.docker.internal` (Docker Desktop)
- Host services via `172.17.0.1` (Linux Docker)
- Internet (if Docker has internet access)

**Multi-cluster networking:**

Kind clusters are isolated by default. For multi-cluster testing, use:
- Shared Docker networks
- Service mesh (Istio, Linkerd)
- Cluster federation

### 6. Kind Limitations

**Not for production:**
- No HA control plane
- Limited resource isolation
- Single-host only
- No persistent storage guarantees

**Use Kind for:**
- Local development
- CI/CD testing
- Configuration validation
- Learning Kubernetes

### Common Kind Issues

**Issue: "Cannot connect to Docker daemon"**

Start Docker:

```bash
# macOS/Windows
# Start Docker Desktop application

# Linux
sudo systemctl start docker
```

**Issue: "Insufficient disk space"**

Clean up Docker resources:

```bash
docker system prune -a --volumes
```

**Issue: "Port already in use"**

Find and stop conflicting process:

```bash
lsof -i :<port>
kill <pid>
```

Or change port mappings in Kind configuration.

---

## Talos Setup

Talos Linux is an immutable, minimal OS designed for Kubernetes. openCenter uses Pulumi for Talos provisioning.

### 1. Install Pulumi

```bash
# macOS
brew install pulumi

# Linux
curl -fsSL https://get.pulumi.com | sh

# Windows
choco install pulumi
```

Verify installation:

```bash
pulumi version
```

**Expected outcome:** Pulumi CLI is available and shows version 3.x or later.

### 2. Configure Pulumi Backend

**Option 1: Pulumi Cloud (recommended for getting started)**

```bash
pulumi login
```

Follow the browser authentication flow.

**Option 2: Self-hosted backend (OpenStack Swift)**

```yaml
infrastructure:
  provider: talos
  cloud:
    openstack:
      # OpenStack credentials for Swift backend
      auth_url: https://openstack.example.com:5000/v3
      application_credential_id: <app-cred-id>
      application_credential_secret: <app-cred-secret>

talos:
  pulumi_config:
    stack_name: production
    swift_container: pulumi-state
    swift_prefix: talos-clusters
```

**Option 3: Local filesystem (development only)**

```bash
pulumi login --local
```

State stored in `~/.pulumi`.

### 3. Configure Talos Provider Settings

```yaml
infrastructure:
  provider: talos

talos:
  # Talos version
  version: v1.7.0
  
  # Machine configuration
  machine_config:
    apparmor_enabled: true
    seccomp_enabled: true
    disk_encryption: true
    kubeprism_enabled: true
  
  # Network configuration
  network_config:
    management_subnet: "10.0.1.0/24"
    control_subnet: "10.0.2.0/24"
    data_subnet: "10.0.3.0/24"
    wireguard_port: 51820
    talos_api_port: 50000
  
  # Security configuration
  security_config:
    vtpm_enabled: true
    image_verification: true
    mfa_required: true
    audit_log_enabled: true
```

### 4. Configure Infrastructure Provider

Talos runs on top of infrastructure providers. Configure the underlying provider:

**OpenStack:**

```yaml
infrastructure:
  provider: talos
  cloud:
    openstack:
      auth_url: https://openstack.example.com:5000/v3
      application_credential_id: <app-cred-id>
      application_credential_secret: <app-cred-secret>
      region: RegionOne
      networking:
        floating_ip_pool: public
```

**AWS (planned):**

```yaml
infrastructure:
  provider: talos
  cloud:
    aws:
      profile: opencenter
      region: us-east-1
      vpc_id: vpc-0123456789abcdef0
```

### 5. Verify Talos Prerequisites

```bash
# Check Pulumi
pulumi version

# Check infrastructure provider access
openstack server list  # For OpenStack
aws ec2 describe-instances  # For AWS

# Validate configuration
openCenter cluster validate <cluster-name>
```

**Expected outcome:** All tools are available and infrastructure provider is accessible.

### 6. Talos Security Features

**Disk encryption:**

Talos supports full-disk encryption with keys stored in:
- TPM 2.0 (if available)
- OpenStack Barbican (key management service)
- External KMS

Configure in cluster YAML:

```yaml
talos:
  machine_config:
    disk_encryption: true
  security_config:
    vtpm_enabled: true
    barbican_key_id: <key-id>  # For OpenStack Barbican
```

**Image verification:**

Talos verifies image signatures before boot:

```yaml
talos:
  image_signature: <signature-hash>
  security_config:
    image_verification: true
```

**Audit logging:**

Enable comprehensive audit logs:

```yaml
talos:
  security_config:
    audit_log_enabled: true
  machine_config:
    log_destination: "https://logs.example.com/talos"
```

### 7. Talos Networking

Talos uses multiple network segments for isolation:

- **Management network**: Talos API access (port 50000)
- **Control plane network**: Kubernetes API and etcd
- **Data network**: Pod-to-pod communication
- **WireGuard**: Encrypted overlay network (optional)

Configure network topology:

```yaml
talos:
  network_config:
    management_subnet: "10.0.1.0/24"
    control_subnet: "10.0.2.0/24"
    data_subnet: "10.0.3.0/24"
    wireguard_port: 51820
    allowed_cidrs:
      - "10.0.0.0/8"
      - "192.168.0.0/16"
```

### Common Talos Issues

**Issue: "Pulumi state locked"**

Another operation is in progress. Wait or force unlock:

```bash
pulumi cancel
# or
pulumi state unlock
```

**Issue: "Image verification failed"**

Verify image signature matches:

```bash
curl -L https://github.com/siderolabs/talos/releases/download/v1.7.0/talos-amd64.iso.sha256
```

Update `image_signature` in configuration.

**Issue: "TPM not available"**

Disable vTPM if infrastructure doesn't support it:

```yaml
talos:
  security_config:
    vtpm_enabled: false
```

Use alternative key storage (Barbican, external KMS).

---

## Multi-Provider Considerations

### Secrets Management

All providers use SOPS with Age encryption:

```bash
# Generate Age key (done automatically by openCenter)
openCenter cluster init <cluster-name>

# Key stored at:
# ~/.config/openCenter/clusters/<org>/<cluster>/secrets/age/<cluster>-key.txt
```

Secrets are encrypted in Git and decrypted during deployment.

### GitOps Repository Structure

All providers generate the same GitOps repository structure:

```
gitops/
├── applications/
│   └── overlays/<cluster>/
└── infrastructure/
    └── clusters/<cluster>/
```

Provider-specific manifests are in `infrastructure/clusters/<cluster>/`.

### Configuration File Location

Cluster configurations are stored in organization-based directories:

```
~/.config/openCenter/clusters/<organization>/<cluster>/.<cluster>-config.yaml
```

Set organization in cluster configuration:

```yaml
metadata:
  organization: my-org
  cluster_name: prod-cluster
```

### Validation Across Providers

Run validation before deployment:

```bash
openCenter cluster validate <cluster-name>
```

Validation checks:
- Schema compliance
- Provider-specific requirements
- Network connectivity
- Quota availability
- Credential validity

---

## Next Steps

After configuring your provider:

1. **Initialize cluster configuration:**
   ```bash
   openCenter cluster init <cluster-name> --provider <provider>
   ```

2. **Validate configuration:**
   ```bash
   openCenter cluster validate <cluster-name>
   ```

3. **Generate GitOps repository:**
   ```bash
   openCenter cluster setup <cluster-name>
   ```

4. **Deploy cluster:**
   ```bash
   openCenter cluster bootstrap <cluster-name>
   ```

## Related Documentation

- [Getting Started Tutorial](../tutorials/getting-started.md) - Complete walkthrough
- [Secrets Management](secrets-management.md) - SOPS and Age key management
- [Configuration Reference](../reference/configuration.md) - Complete YAML schema
- [Provider Documentation](../providers/README.md) - Provider-specific details
- [Troubleshooting](troubleshooting.md) - Common issues and solutions

## External Resources

- [OpenStack Application Credentials](https://docs.openstack.org/keystone/latest/user/application_credentials.html)
- [AWS IAM Best Practices](https://docs.aws.amazon.com/IAM/latest/UserGuide/best-practices.html)
- [Kind Quick Start](https://kind.sigs.k8s.io/docs/user/quick-start/)
- [Talos Documentation](https://www.talos.dev/docs/)
- [Pulumi Getting Started](https://www.pulumi.com/docs/get-started/)
