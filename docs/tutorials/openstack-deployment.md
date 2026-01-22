# Deploy Production Kubernetes on OpenStack


## Table of Contents

- [What You'll Build](#what-youll-build)
- [Prerequisites](#prerequisites)
- [Step 1: Gather OpenStack Information](#step-1-gather-openstack-information)
- [Step 2: Initialize Cluster Configuration](#step-2-initialize-cluster-configuration)
- [Step 3: Configure OpenStack Credentials](#step-3-configure-openstack-credentials)
- [Step 4: Configure Cluster Networking](#step-4-configure-cluster-networking)
- [Step 5: Configure Storage](#step-5-configure-storage)
- [Step 6: Enable SOPS Encryption](#step-6-enable-sops-encryption)
- [Step 7: Configure OpenTofu Backend](#step-7-configure-opentofu-backend)
- [Step 8: Validate Configuration](#step-8-validate-configuration)
- [Step 9: Generate GitOps Repository](#step-9-generate-gitops-repository)
- [Step 10: Bootstrap Infrastructure](#step-10-bootstrap-infrastructure)
- [Step 11: Verify Cluster Deployment](#step-11-verify-cluster-deployment)
- [Step 12: Access Cluster Nodes](#step-12-access-cluster-nodes)
- [What You Learned](#what-you-learned)
- [Next Steps](#next-steps)
- [Common Issues](#common-issues)
- [Production Considerations](#production-considerations)
- [Cleanup](#cleanup)
- [Summary](#summary)
- [Related Documentation](#related-documentation)
- [Getting Help](#getting-help)
**doc_type: tutorial**

Deploy a production-ready Kubernetes cluster on OpenStack in 45 minutes. You'll configure OpenStack credentials, set up networking, generate a GitOps repository, and bootstrap a highly available cluster.

## What You'll Build

By the end of this tutorial, you'll have:
- A 3-master, 2-worker Kubernetes cluster running on OpenStack
- Encrypted secrets managed with SOPS
- A complete GitOps repository with FluxCD
- Calico networking with OpenStack integration
- OpenStack Cloud Controller Manager and CSI driver
- Production-ready security hardening

## Prerequisites

Before starting, you need:
- **OpenStack account** with project access
- **OpenStack credentials** (application credentials or password)
- **openCenter installed** (see [Getting Started](getting-started.md))
- **OpenStack CLI tools** installed (`python3-openstackclient`)
- **Terraform or OpenTofu** installed (v1.6+)
- **Git** installed and configured
- **45 minutes** of time

### OpenStack Requirements

Your OpenStack project needs:
- Quota for 5 instances (3 masters, 2 workers)
- Floating IP quota (at least 1 for API access)
- Network with external connectivity
- Ubuntu 24.04 image available
- Flavors: minimum 4GB RAM for masters, 8GB for workers

### Verify OpenStack Access

Test your OpenStack connection:

```bash
openstack server list
openstack network list
openstack flavor list
```

If these commands work, you're ready to proceed.


## Step 1: Gather OpenStack Information

You need specific OpenStack details before initializing your cluster. Collect these values:

### Authentication Details

**Option A: Application Credentials (Recommended)**

Create application credentials for better security:

```bash
openstack application credential create opencenter-prod \
  --description "openCenter cluster deployment" \
  --role member
```

Save the output:
- `id`: Your application credential ID
- `secret`: Your application credential secret

**Option B: Password Authentication**

If application credentials aren't available, use your OpenStack password. You'll need:
- Username
- Password
- Project name
- User domain name
- Project domain name

### Network Information

Find your network details:

```bash
# List networks
openstack network list

# Find external network (for floating IPs)
openstack network list --external

# Get network ID
openstack network show <network-name> -f value -c id
```

Record these values:
- **Network ID**: Internal network for cluster VMs
- **External Network ID**: For floating IP allocation (usually named PUBLICNET or similar)
- **Subnet ID**: Subnet within your network

### Image and Flavor Information

Find available images and flavors:

```bash
# List Ubuntu images
openstack image list | grep -i ubuntu

# List available flavors
openstack flavor list
```

Record:
- **Image ID**: Ubuntu 24.04 image (or 22.04)
- **Master flavor**: Minimum 4GB RAM (e.g., `gp.0.4.8`)
- **Worker flavor**: Minimum 8GB RAM (e.g., `gp.0.4.16`)

### Region and Availability Zone

```bash
# Show current region
openstack configuration show | grep region

# List availability zones
openstack availability zone list
```

Record:
- **Region**: Your OpenStack region (e.g., `sjc3`, `dfw3`)
- **Availability Zone**: Where VMs will be created (e.g., `az1`)


## Step 2: Initialize Cluster Configuration

Create your cluster configuration with OpenStack-specific settings:

```bash
openCenter cluster init prod-k8s \
  --opencenter.meta.env=production \
  --opencenter.meta.region=sjc3 \
  --opencenter.infrastructure.provider=openstack \
  --opencenter.infrastructure.cloud.openstack.auth_url=https://identity.api.sjc3.rackspacecloud.com/v3 \
  --opencenter.infrastructure.cloud.openstack.region=sjc3 \
  --opencenter.infrastructure.cloud.openstack.tenant_name=my-project \
  --opencenter.infrastructure.cloud.openstack.availability_zone=az1 \
  --opencenter.cluster.kubernetes.version=1.33.5 \
  --opencenter.cluster.kubernetes.master_count=3 \
  --opencenter.cluster.kubernetes.worker_count=2 \
  --opencenter.gitops.git_dir=/home/user/gitops/prod-k8s
```

Replace these values with your OpenStack details:
- `auth_url`: Your OpenStack identity endpoint
- `region`: Your OpenStack region
- `tenant_name`: Your project name
- `availability_zone`: Your availability zone
- `git_dir`: Where to create the GitOps repository

You'll see output like:

```
Generated ed25519 SSH key pair at ~/.config/openCenter/clusters/opencenter/secrets/ssh/prod-k8s-production-sjc3
Created cluster configuration in organization 'opencenter' at '~/.config/openCenter/clusters/opencenter/infrastructure/clusters/prod-k8s'
GitOps repository root: /home/user/gitops/prod-k8s
SOPS key location: ~/.config/openCenter/clusters/opencenter/secrets/age/keys/prod-k8s-key.txt
```

The configuration file is at:
```
~/.config/openCenter/clusters/opencenter/.prod-k8s-config.yaml
```


## Step 3: Configure OpenStack Credentials

Edit your cluster configuration to add credentials and network details:

```bash
# Open the configuration file
vim ~/.config/openCenter/clusters/opencenter/.prod-k8s-config.yaml
```

### Add Authentication Credentials

**For Application Credentials:**

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        application_credential_id: "abc123def456..."
        application_credential_secret: "secret-value-here"
```

**For Password Authentication:**

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        domain: "default"
        user_domain_name: "rackspace_cloud_domain"
        project_domain_name: "rackspace_cloud_domain"

# Add password to secrets section
secrets:
  global:
    openstack:
      password: "your-openstack-password"
```

### Configure Network Settings

Update the networking section with your OpenStack network details:

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        networking:
          network_id: "a1b2c3d4-1234-5678-90ab-cdef12345678"
          subnet_id: "e5f6g7h8-1234-5678-90ab-cdef12345678"
          floating_ip_pool: "PUBLICNET"
          floating_network_id: "723f8fa2-dbf7-4cec-8d5f-017e62c12f79"
          router_external_network_id: "723f8fa2-dbf7-4cec-8d5f-017e62c12f79"
```

Replace with your actual network IDs from Step 1.

### Configure Image and Flavors

Update compute resources:

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        image_id: "799dcf97-3656-4361-8187-13ab1b295e33"  # Ubuntu 24.04
  cluster:
    kubernetes:
      flavor_master: "gp.0.4.8"   # 4 vCPU, 8GB RAM
      flavor_worker: "gp.0.4.16"  # 4 vCPU, 16GB RAM
```


## Step 4: Configure Cluster Networking

Set up Kubernetes networking and security:

```yaml
opencenter:
  cluster:
    networking:
      k8s_api_port_acl:
        - "0.0.0.0/0"  # Allow all (restrict in production)
      dns_nameservers:
        - "8.8.8.8"
        - "8.8.4.4"
      ntp_servers:
        - "time.sjc3.rackspace.com"
        - "time2.sjc3.rackspace.com"
    kubernetes:
      subnet_pods: "10.42.0.0/16"      # Pod network CIDR
      subnet_services: "10.43.0.0/16"  # Service network CIDR
      loadbalancer_provider: "ovn"     # or "octavia" for Octavia LB
      network_plugin:
        calico:
          enabled: true
          cni_iface: "enp3s0"
          encapsulation_type: "VXLAN"
          nat_outgoing: true
```

### Network Configuration Notes

**Load Balancer Provider:**
- `ovn`: Uses OVN (Open Virtual Network) for load balancing - faster, no extra resources
- `octavia`: Uses OpenStack Octavia - more features, requires Octavia service

**Calico Settings:**
- `cni_iface`: Network interface name (usually `enp3s0` or `eth0`)
- `encapsulation_type`: `VXLAN` (overlay) or `IPIP` (IP-in-IP)
- `nat_outgoing`: Enable NAT for pod traffic to external networks

**Security:**
- `k8s_api_port_acl`: Restrict to your IP range in production (e.g., `["203.0.113.0/24"]`)


## Step 5: Configure Storage

Set up persistent storage with OpenStack Cinder:

```yaml
opencenter:
  storage:
    default_storage_class: "csi-cinder-sc-delete"
    worker_volume_size: 40  # GB
    worker_volume_type: "HA-Standard"
    worker_volume_destination_type: "volume"
    worker_volume_source_type: "image"
```

### Storage Options

**Volume Types:**
- `HA-Standard`: High availability, standard performance
- `HA-Performance`: High availability, better IOPS
- `Standard`: Single-replica, lower cost

**Storage Classes:**
- `csi-cinder-sc-delete`: Volumes deleted when PVC is deleted
- `csi-cinder-sc-retain`: Volumes retained after PVC deletion

Check available volume types:

```bash
openstack volume type list
```


## Step 6: Enable SOPS Encryption

Configure SOPS to encrypt secrets in your GitOps repository:

The SOPS Age key was created during initialization. Configure it in your cluster config:

```yaml
secrets:
  sops_age_key_file: "/home/user/.config/openCenter/clusters/opencenter/secrets/age/keys/prod-k8s-key.txt"
```

Verify the key exists:

```bash
cat ~/.config/openCenter/clusters/opencenter/secrets/age/keys/prod-k8s-key.txt
```

You should see an Age key starting with `AGE-SECRET-KEY-`.

### Add SSH Keys

Configure SSH access to cluster nodes:

```yaml
opencenter:
  cluster:
    ssh_authorized_keys:
      - "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExampleKeyHere user@workstation"
```

Add your public SSH key (from `~/.ssh/id_ed25519.pub` or similar).


## Step 7: Configure OpenTofu Backend

Set up state storage for Terraform/OpenTofu:

### Option A: Local Backend (Development)

```yaml
opentofu:
  enabled: true
  path: "opentofu"
  backend:
    type: "local"
    local:
      path: "/home/user/gitops/prod-k8s/terraform.tfstate"
```

### Option B: S3 Backend (Production)

For production, use remote state storage:

```yaml
opentofu:
  enabled: true
  path: "opentofu"
  backend:
    type: "s3"
    s3:
      bucket: "prod-k8s-terraform-state"
      key: "prod-k8s/terraform.tfstate"
      region: "us-east-1"
      endpoint: "https://swift.api.sjc3.rackspacecloud.com"

secrets:
  global:
    aws:
      infrastructure:
        access_key: "your-s3-access-key"
        secret_access_key: "your-s3-secret-key"
        region: "us-east-1"
```

For OpenStack Swift as S3 backend, use the Swift endpoint and Swift credentials.


## Step 8: Validate Configuration

Check that your configuration is valid before deployment:

```bash
openCenter cluster validate prod-k8s
```

You should see:

```
Validation successful.
```

### Common Validation Errors

**"auth_url is empty"**
- Add `opencenter.infrastructure.cloud.openstack.auth_url` to your config

**"network_id is required"**
- Add `opencenter.infrastructure.cloud.openstack.networking.network_id`

**"SOPS key file not found"**
- Check `secrets.sops_age_key_file` path is correct
- Verify the file exists and is readable

**"invalid CIDR range"**
- Check `subnet_pods` and `subnet_services` don't overlap
- Ensure CIDRs are valid (e.g., `10.42.0.0/16`)

If validation fails, read the error messages carefully. They indicate which fields need correction.


## Step 9: Generate GitOps Repository

Create the GitOps repository structure with all manifests:

```bash
openCenter cluster setup prod-k8s
```

This command:
- Renders base GitOps structure
- Generates cluster-specific manifests
- Creates Terraform/OpenTofu configuration
- Sets up FluxCD GitOps automation

You'll see output like:

```
Created GitOps repo at: /home/user/gitops/prod-k8s
```

### Explore the GitOps Repository

Check what was created:

```bash
cd /home/user/gitops/prod-k8s
tree -L 3
```

You'll see:

```
.
├── applications/
│   └── overlays/
│       └── prod-k8s/          # Application manifests
├── infrastructure/
│   └── clusters/
│       └── prod-k8s/          # Infrastructure configs
│           ├── main.tf        # Terraform main config
│           ├── provider.tf    # OpenStack provider
│           ├── Makefile       # Build automation
│           └── flux/          # FluxCD configs
└── .git/                      # Git repository
```

### Initialize Git Repository

Commit the initial configuration:

```bash
cd /home/user/gitops/prod-k8s
git add .
git commit -m "Initial cluster configuration for prod-k8s"
```

If you have a remote Git repository:

```bash
git remote add origin git@github.com:yourorg/prod-k8s-gitops.git
git push -u origin main
```


## Step 10: Bootstrap Infrastructure

Deploy the cluster infrastructure to OpenStack:

```bash
openCenter cluster bootstrap prod-k8s
```

This command runs these steps automatically:
1. **make terraform**: Generates Terraform configuration from templates
2. **terraform init**: Initializes Terraform providers and modules
3. **terraform apply**: Provisions OpenStack resources (VMs, networks, security groups)

The bootstrap process takes 15-30 minutes. You'll see progress output:

```
Step "make-terraform": Run make terraform
$ make terraform
Running make terraform in /home/user/gitops/prod-k8s/infrastructure/clusters/prod-k8s

Step "terraform-init": Initialize Terraform
$ terraform init
Initializing Terraform in /home/user/gitops/prod-k8s/infrastructure/clusters/prod-k8s

Step "terraform-apply": Apply Terraform configuration
$ terraform apply -auto-approve
Applying Terraform configuration (this may take several minutes)...
Still running... (elapsed: 30s)
Still running... (elapsed: 1m0s)
...
Command completed successfully after 18m32s

Bootstrap complete.
Log written to /home/user/gitops/prod-k8s/infrastructure/clusters/prod-k8s/logs/bootstrap-2025-01-15-1736960400.log
```

### What Gets Created

OpenStack resources provisioned:
- **5 compute instances**: 3 masters, 2 workers
- **Security groups**: API access, node communication, pod networking
- **Floating IP**: For Kubernetes API server access
- **Server groups**: Anti-affinity for HA placement
- **Volumes**: Boot volumes for each node (if configured)

### Monitor Progress

Watch OpenStack resources being created:

```bash
# In another terminal
watch -n 5 'openstack server list'
```

You'll see instances transition from BUILD to ACTIVE state.


### Bootstrap Troubleshooting

**"openstack CLI not found"**
```bash
pip3 install python-openstackclient
```

**"terraform init failed"**
- Check internet connectivity for provider downloads
- Verify OpenStack credentials are correct
- Check `opentofu.backend` configuration

**"terraform apply failed: quota exceeded"**
- Check OpenStack quotas: `openstack quota show`
- Reduce node counts or use smaller flavors
- Contact OpenStack administrator for quota increase

**"authentication failed"**
- Verify `application_credential_id` and `application_credential_secret`
- Check `auth_url` is correct and accessible
- Ensure credentials haven't expired

**"network not found"**
- Verify `network_id` exists: `openstack network show <id>`
- Check you have access to the network
- Confirm network is in the correct project

### Resume Failed Bootstrap

If bootstrap fails partway through, fix the issue and resume:

```bash
# Resume from where it failed
openCenter cluster bootstrap prod-k8s

# Or restart from a specific step
openCenter cluster bootstrap prod-k8s --from-step terraform-apply

# Or restart completely
openCenter cluster bootstrap prod-k8s --restart
```

Bootstrap state is saved in:
```
/home/user/gitops/prod-k8s/infrastructure/clusters/prod-k8s/logs/bootstrap-state.json
```


## Step 11: Verify Cluster Deployment

After bootstrap completes, verify your cluster is running.

### Check OpenStack Resources

Verify instances are running:

```bash
openstack server list --name prod-k8s
```

You should see:

```
+--------------------------------------+------------------+--------+----------------------------------+
| ID                                   | Name             | Status | Networks                         |
+--------------------------------------+------------------+--------+----------------------------------+
| abc123...                            | prod-k8s-cp-1    | ACTIVE | private=10.0.0.10, 203.0.113.50 |
| def456...                            | prod-k8s-cp-2    | ACTIVE | private=10.0.0.11                |
| ghi789...                            | prod-k8s-cp-3    | ACTIVE | private=10.0.0.12                |
| jkl012...                            | prod-k8s-wn-1    | ACTIVE | private=10.0.0.20                |
| mno345...                            | prod-k8s-wn-2    | ACTIVE | private=10.0.0.21                |
+--------------------------------------+------------------+--------+----------------------------------+
```

All instances should show `ACTIVE` status.

### Get Kubeconfig

The kubeconfig file is generated during bootstrap:

```bash
export KUBECONFIG=/home/user/gitops/prod-k8s/infrastructure/clusters/prod-k8s/kubeconfig.yaml
```

Or copy it to your default location:

```bash
mkdir -p ~/.kube
cp /home/user/gitops/prod-k8s/infrastructure/clusters/prod-k8s/kubeconfig.yaml ~/.kube/config
```

### Check Cluster Nodes

Verify all nodes are ready:

```bash
kubectl get nodes
```

Expected output:

```
NAME            STATUS   ROLES           AGE   VERSION
prod-k8s-cp-1   Ready    control-plane   15m   v1.33.5
prod-k8s-cp-2   Ready    control-plane   14m   v1.33.5
prod-k8s-cp-3   Ready    control-plane   13m   v1.33.5
prod-k8s-wn-1   Ready    <none>          12m   v1.33.5
prod-k8s-wn-2   Ready    <none>          11m   v1.33.5
```

All nodes should show `Ready` status. If nodes show `NotReady`, wait a few minutes for initialization to complete.


### Check System Pods

Verify core system components are running:

```bash
kubectl get pods -n kube-system
```

You should see pods for:
- `calico-node-*`: Calico networking (one per node)
- `calico-kube-controllers-*`: Calico controller
- `coredns-*`: DNS service
- `kube-apiserver-*`: API servers (one per master)
- `kube-controller-manager-*`: Controller managers
- `kube-scheduler-*`: Schedulers
- `kube-proxy-*`: Network proxy (one per node)

All pods should show `Running` status.

### Check OpenStack Integration

Verify OpenStack Cloud Controller Manager:

```bash
kubectl get pods -n kube-system -l app=openstack-cloud-controller-manager
```

Verify OpenStack CSI driver:

```bash
kubectl get pods -n kube-system -l app=csi-cinder-controllerplugin
kubectl get pods -n kube-system -l app=csi-cinder-nodeplugin
```

### Test Storage

Create a test PVC to verify Cinder CSI:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: csi-cinder-sc-delete
EOF
```

Check PVC status:

```bash
kubectl get pvc test-pvc
```

Should show `Bound` status:

```
NAME       STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS
test-pvc   Bound    pvc-abc123-def4-5678-90ab-cdef12345678    1Gi        RWO            csi-cinder-sc-delete
```

Verify volume was created in OpenStack:

```bash
openstack volume list | grep pvc-
```

Clean up test PVC:

```bash
kubectl delete pvc test-pvc
```


### Test Networking

Deploy a test application to verify networking:

```bash
kubectl create deployment nginx --image=nginx:latest
kubectl expose deployment nginx --port=80 --type=LoadBalancer
```

Wait for external IP assignment:

```bash
kubectl get svc nginx -w
```

You should see an external IP assigned (this may take 1-2 minutes):

```
NAME    TYPE           CLUSTER-IP      EXTERNAL-IP     PORT(S)        AGE
nginx   LoadBalancer   10.43.100.50    203.0.113.100   80:30080/TCP   2m
```

Test connectivity:

```bash
curl http://203.0.113.100
```

You should see the nginx welcome page.

Clean up test deployment:

```bash
kubectl delete svc nginx
kubectl delete deployment nginx
```


## Step 12: Access Cluster Nodes

SSH into cluster nodes for troubleshooting or maintenance.

### Find Node IP Addresses

Get floating IP for master node:

```bash
openstack server show prod-k8s-cp-1 -f value -c addresses
```

### SSH to Master Node

```bash
ssh -i ~/.config/openCenter/clusters/opencenter/secrets/ssh/prod-k8s-production-sjc3 ubuntu@203.0.113.50
```

Replace:
- SSH key path with your actual key location
- IP address with your master node's floating IP

### SSH to Worker Nodes

Worker nodes typically don't have floating IPs. SSH through the master:

```bash
# From master node
ssh 10.0.0.20  # Worker node private IP
```

Or use SSH proxy jump:

```bash
ssh -i ~/.config/openCenter/clusters/opencenter/secrets/ssh/prod-k8s-production-sjc3 \
  -J ubuntu@203.0.113.50 \
  ubuntu@10.0.0.20
```

### Check Node Status

Once connected to a node:

```bash
# Check kubelet status
sudo systemctl status kubelet

# View kubelet logs
sudo journalctl -u kubelet -f

# Check container runtime
sudo crictl ps

# View node resources
top
df -h
```


## What You Learned

You now understand how to:

1. **Gather OpenStack prerequisites**: Authentication, networking, images, and flavors
2. **Initialize cluster configuration**: Set OpenStack-specific parameters
3. **Configure credentials**: Application credentials or password authentication
4. **Set up networking**: VLANs, floating IPs, load balancers, and CNI
5. **Configure storage**: Cinder volumes and storage classes
6. **Enable secrets encryption**: SOPS with Age keys
7. **Generate GitOps repository**: Complete infrastructure-as-code structure
8. **Bootstrap infrastructure**: Automated Terraform provisioning
9. **Verify deployment**: Check nodes, pods, storage, and networking
10. **Access cluster nodes**: SSH for troubleshooting and maintenance

### Key Concepts

**OpenStack Integration:**
- Cloud Controller Manager handles load balancers and node lifecycle
- CSI driver provides persistent storage via Cinder
- Neutron networking integrates with Calico CNI
- Security groups control network access

**High Availability:**
- 3 master nodes provide control plane redundancy
- Anti-affinity rules spread nodes across compute hosts
- Load balancer distributes API traffic
- etcd runs on all master nodes for data redundancy

**GitOps Workflow:**
- Single YAML configuration as source of truth
- Generated manifests stored in Git
- Infrastructure changes tracked in version control
- Declarative updates through configuration changes

**Security:**
- SOPS encrypts secrets at rest
- SSH keys for node access
- Network policies via Calico
- OS hardening enabled by default


## Next Steps

Now that you have a running cluster, you can:

### Deploy Applications

1. **Add applications to GitOps repository**:
   ```bash
   cd /home/user/gitops/prod-k8s/applications/overlays/prod-k8s
   # Add your application manifests here
   git add .
   git commit -m "Add application manifests"
   git push
   ```

2. **Enable FluxCD for automated deployment**:
   ```bash
   kubectl apply -f /home/user/gitops/prod-k8s/infrastructure/clusters/prod-k8s/flux/
   ```

### Enable Additional Services

Edit your cluster configuration to enable services:

```yaml
opencenter:
  services:
    cert-manager:
      enabled: true  # Automated TLS certificates
    kube-prometheus-stack:
      enabled: true  # Monitoring with Prometheus and Grafana
    loki:
      enabled: true  # Log aggregation
    velero:
      enabled: true  # Backup and disaster recovery
```

Then regenerate and apply:

```bash
openCenter cluster setup prod-k8s --force
cd /home/user/gitops/prod-k8s
git add .
git commit -m "Enable monitoring and logging services"
kubectl apply -k applications/overlays/prod-k8s/
```

### Configure Monitoring

Set up Prometheus and Grafana:

1. Enable kube-prometheus-stack in configuration
2. Add Grafana admin password to secrets
3. Access Grafana dashboard via LoadBalancer or Ingress

See [Monitoring Setup](../how-to/monitoring-setup.md) for details.

### Set Up Backups

Configure Velero for cluster backups:

1. Create S3 bucket for backups
2. Add S3 credentials to secrets
3. Enable Velero service
4. Configure backup schedules

See [Backup and Recovery](../how-to/backup-recovery.md) for details.


### Scale the Cluster

Add more worker nodes:

```yaml
opencenter:
  cluster:
    kubernetes:
      worker_count: 5  # Increase from 2 to 5
```

Apply changes:

```bash
openCenter cluster validate prod-k8s
openCenter cluster setup prod-k8s --force
cd /home/user/gitops/prod-k8s/infrastructure/clusters/prod-k8s
terraform apply
```

### Upgrade Kubernetes

Update Kubernetes version:

```yaml
opencenter:
  cluster:
    kubernetes:
      version: "1.34.0"  # New version
```

Follow the upgrade procedure in [Cluster Upgrades](../how-to/cluster-upgrades.md).

### Multi-Cluster Management

Deploy additional clusters:

```bash
openCenter cluster init prod-k8s-west --opencenter.meta.region=dfw3
openCenter cluster init staging-k8s --opencenter.meta.env=staging
```

See [Multi-Cluster Management](../how-to/multi-cluster.md) for managing multiple clusters.


## Common Issues

### Nodes Not Ready

**Symptom**: `kubectl get nodes` shows `NotReady` status

**Causes**:
- Kubelet not started
- Network plugin not initialized
- Node can't reach API server

**Solutions**:
```bash
# SSH to node and check kubelet
ssh ubuntu@<node-ip>
sudo systemctl status kubelet
sudo journalctl -u kubelet -n 100

# Check Calico pods
kubectl get pods -n kube-system -l k8s-app=calico-node

# Restart kubelet if needed
sudo systemctl restart kubelet
```

### Pods Stuck in Pending

**Symptom**: Pods show `Pending` status indefinitely

**Causes**:
- Insufficient resources
- Node selector mismatch
- PVC not bound

**Solutions**:
```bash
# Check pod events
kubectl describe pod <pod-name>

# Check node resources
kubectl top nodes

# Check PVC status
kubectl get pvc
```

### Storage Not Working

**Symptom**: PVCs stuck in `Pending` state

**Causes**:
- CSI driver not running
- OpenStack credentials invalid
- Volume type not available

**Solutions**:
```bash
# Check CSI driver pods
kubectl get pods -n kube-system -l app=csi-cinder-controllerplugin

# Check CSI driver logs
kubectl logs -n kube-system -l app=csi-cinder-controllerplugin

# Verify OpenStack credentials
openstack volume list

# Check available volume types
openstack volume type list
```


### LoadBalancer Service Not Getting External IP

**Symptom**: Services with `type: LoadBalancer` stuck without external IP

**Causes**:
- Cloud controller manager not running
- OpenStack credentials invalid
- Floating IP pool exhausted
- Octavia service unavailable (if using Octavia)

**Solutions**:
```bash
# Check cloud controller manager
kubectl get pods -n kube-system -l app=openstack-cloud-controller-manager
kubectl logs -n kube-system -l app=openstack-cloud-controller-manager

# Check floating IP availability
openstack floating ip list
openstack floating ip create PUBLICNET

# Verify load balancer provider setting
# In cluster config: loadbalancer_provider: "ovn" or "octavia"
```

### API Server Unreachable

**Symptom**: `kubectl` commands fail with connection errors

**Causes**:
- API server not running
- Floating IP not assigned
- Security group blocking access
- Kubeconfig incorrect

**Solutions**:
```bash
# Check API server pods on master nodes
ssh ubuntu@<master-ip>
sudo crictl ps | grep kube-apiserver

# Verify floating IP assignment
openstack server show prod-k8s-cp-1 -f value -c addresses

# Check security group rules
openstack security group list
openstack security group rule list <security-group-id>

# Test API connectivity
curl -k https://<api-floating-ip>:6443/healthz
```

### DNS Resolution Failing

**Symptom**: Pods can't resolve DNS names

**Causes**:
- CoreDNS not running
- Network policy blocking DNS
- Incorrect DNS configuration

**Solutions**:
```bash
# Check CoreDNS pods
kubectl get pods -n kube-system -l k8s-app=kube-dns

# Test DNS from a pod
kubectl run -it --rm debug --image=busybox --restart=Never -- nslookup kubernetes.default

# Check CoreDNS logs
kubectl logs -n kube-system -l k8s-app=kube-dns
```


## Production Considerations

Before using this cluster in production, address these items:

### Security Hardening

**Restrict API Access:**
```yaml
opencenter:
  cluster:
    networking:
      k8s_api_port_acl:
        - "203.0.113.0/24"  # Your office network
        - "198.51.100.0/24"  # VPN network
```

**Enable Pod Security Standards:**
```bash
kubectl label namespace default pod-security.kubernetes.io/enforce=restricted
```

**Configure Network Policies:**
```yaml
# Deny all ingress by default
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-ingress
spec:
  podSelector: {}
  policyTypes:
  - Ingress
```

### High Availability

**Verify Anti-Affinity:**
```bash
# Check master nodes are on different compute hosts
openstack server show prod-k8s-cp-1 -f value -c "OS-EXT-SRV-ATTR:host"
openstack server show prod-k8s-cp-2 -f value -c "OS-EXT-SRV-ATTR:host"
openstack server show prod-k8s-cp-3 -f value -c "OS-EXT-SRV-ATTR:host"
```

Each should show a different compute host.

**Test Failover:**
```bash
# Simulate master node failure
openstack server stop prod-k8s-cp-1

# Verify cluster still responds
kubectl get nodes

# Restart node
openstack server start prod-k8s-cp-1
```

### Backup Strategy

**etcd Backups:**

Enable automated etcd backups:
```yaml
opencenter:
  services:
    etcd-backup:
      enabled: true
      s3_host: "https://swift.api.sjc3.rackspacecloud.com"
      s3_region: "SJC3"
```

**Velero for Application Backups:**
```yaml
opencenter:
  services:
    velero:
      enabled: true
      backup_bucket: "prod-k8s-backups"
      region: "us-east-1"
```

### Monitoring and Alerting

**Enable Prometheus Stack:**
```yaml
opencenter:
  services:
    kube-prometheus-stack:
      enabled: true
      prometheus_volume_size: 100  # GB
      grafana_volume_size: 20      # GB
```

**Configure Alerts:**
- Node down alerts
- High CPU/memory usage
- Disk space warnings
- Certificate expiration
- etcd health


### Resource Planning

**Compute Resources:**
- Masters: 4 vCPU, 8GB RAM minimum (production: 8 vCPU, 16GB RAM)
- Workers: 4 vCPU, 16GB RAM minimum (scale based on workload)
- Plan for 20-30% overhead for system pods

**Storage:**
- etcd: Fast SSD-backed volumes (HA-Performance type)
- Application data: Size based on workload requirements
- Backups: 2-3x cluster data size

**Network:**
- Pod CIDR: Plan for growth (default /16 supports 65,536 pods)
- Service CIDR: Typically /16 is sufficient
- Ensure no overlap with existing networks

### Cost Optimization

**Right-Size Flavors:**
```bash
# Monitor actual resource usage
kubectl top nodes
kubectl top pods --all-namespaces
```

Adjust flavors based on actual usage patterns.

**Use Spot/Preemptible Instances:**

For non-critical workloads, consider spot instances (if available in your OpenStack).

**Storage Tiering:**
- Hot data: HA-Performance volumes
- Warm data: HA-Standard volumes
- Cold data: Object storage (Swift)

### Compliance and Auditing

**Enable Audit Logging:**
```yaml
opencenter:
  cluster:
    kubernetes:
      security:
        k8s_hardening: true
```

**Track Changes:**
- All configuration changes in Git
- Terraform state tracks infrastructure changes
- Kubernetes audit logs track API access

**Regular Security Scans:**
```bash
# Install and run Trivy
kubectl apply -f https://raw.githubusercontent.com/aquasecurity/trivy-operator/main/deploy/static/trivy-operator.yaml
```


## Cleanup

To destroy the cluster and remove all resources:

### Backup Important Data

Before destroying, backup anything you need:

```bash
# Backup etcd
kubectl exec -n kube-system etcd-prod-k8s-cp-1 -- etcdctl snapshot save /tmp/backup.db

# Export application manifests
kubectl get all --all-namespaces -o yaml > cluster-backup.yaml

# Backup persistent volumes (if using Velero)
velero backup create final-backup --wait
```

### Destroy Infrastructure

```bash
cd /home/user/gitops/prod-k8s/infrastructure/clusters/prod-k8s
terraform destroy -auto-approve
```

This removes:
- All compute instances
- Security groups
- Floating IPs
- Volumes (if configured for deletion)

### Verify Cleanup

Check OpenStack resources are removed:

```bash
openstack server list --name prod-k8s
openstack volume list | grep prod-k8s
openstack floating ip list
```

### Remove Local Files

```bash
# Remove GitOps repository
rm -rf /home/user/gitops/prod-k8s

# Remove cluster configuration (optional)
rm ~/.config/openCenter/clusters/opencenter/.prod-k8s-config.yaml

# Remove SOPS keys (optional, keep for recovery)
# rm ~/.config/openCenter/clusters/opencenter/secrets/age/keys/prod-k8s-key.txt
```

**Warning**: Keep SOPS keys if you have encrypted backups you might need to restore.


## Summary

You've successfully deployed a production Kubernetes cluster on OpenStack using openCenter. The deployment includes:

**Infrastructure:**
- 3 master nodes for control plane HA
- 2 worker nodes for application workloads
- Anti-affinity placement for resilience
- Floating IP for API access

**Networking:**
- Calico CNI for pod networking
- OpenStack Neutron integration
- Load balancer support (OVN or Octavia)
- Security groups for access control

**Storage:**
- OpenStack Cinder CSI driver
- Dynamic volume provisioning
- Multiple storage classes

**Security:**
- SOPS encryption for secrets
- SSH key-based node access
- OS hardening enabled
- Network policies ready

**Operations:**
- GitOps repository for infrastructure-as-code
- Terraform state management
- Automated bootstrap process
- Comprehensive validation

The cluster is ready for application deployment. Follow the Next Steps section to add monitoring, backups, and deploy your applications.

## Related Documentation

- [Getting Started Tutorial](getting-started.md) - Basic openCenter concepts
- [Configuration Reference](../reference/configuration.md) - Complete configuration options
- [OpenStack Provider](../providers/openstack/README.md) - OpenStack-specific details
- [Secrets Management](../how-to/secrets-management.md) - SOPS and encryption
- [Troubleshooting Guide](../how-to/troubleshooting.md) - Common issues and solutions
- [GitOps Workflow](../explanation/gitops-workflow.md) - Understanding GitOps with openCenter
- [Security Model](../explanation/security-model.md) - Security architecture

## Getting Help

**Issues with this tutorial:**
- Check [GitHub Issues](https://github.com/rackerlabs/openCenter-cli/issues)
- Review [Troubleshooting Guide](../how-to/troubleshooting.md)

**OpenStack-specific questions:**
- Consult your OpenStack administrator
- Check [OpenStack Documentation](https://docs.openstack.org/)

**Kubernetes questions:**
- See [Kubernetes Documentation](https://kubernetes.io/docs/)
- Check [Calico Documentation](https://docs.tigera.io/calico/latest/about/)

---

**Tutorial Duration**: 45 minutes  
**Last Updated**: January 2025  
**Maintained By**: openCenter Team

