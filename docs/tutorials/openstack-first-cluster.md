# Deploy Your First Production Cluster on OpenStack

**Purpose:** For OpenStack users, shows how to deploy a production-ready Kubernetes cluster on OpenStack, covering prerequisites through validation.

By the end of this tutorial, you'll have a fully functional, production-ready Kubernetes cluster running on OpenStack with platform services deployed via GitOps.

**Time:** 45-60 minutes

## What You'll Build

A production Kubernetes cluster with:
- 3 control plane nodes (high availability)
- 3 worker nodes (production capacity)
- Calico CNI networking
- OpenStack Cinder CSI storage
- Octavia load balancer
- 20+ platform services (cert-manager, Keycloak, monitoring, etc.)
- FluxCD GitOps continuous delivery

## Prerequisites

Before starting, ensure you have:

**OpenStack Access:**
- OpenStack cloud account (public or private)
- API credentials (username, password, project name)
- OpenStack CLI installed (`openstack` command)
- Network quota (1 network, 1 subnet, 1 router)
- Compute quota (6 instances minimum, 24 vCPUs, 96 GB RAM)
- Storage quota (240 GB volumes)

**Local Tools:**
- openCenter CLI installed
- Git installed
- SSH client
- Text editor

**Verify OpenStack Access:**

```bash
# Test OpenStack credentials
openstack server list

# Check quotas
openstack quota show
```

If these commands work, you're ready to proceed.

## Step 1: Initialize Cluster Configuration

Create a new cluster configuration with OpenStack defaults:

```bash
opencenter cluster init prod-cluster \
  --org my-company \
  --type openstack
```

**What happens:**
- Creates configuration file at `~/.config/opencenter/clusters/my-company/.prod-cluster-config.yaml`
- Applies OpenStack defaults (region, availability zone, image ID, etc.)
- Generates SSH keys for cluster access
- Generates SOPS Age keys for secrets encryption

**Output:**

```
✓ Created cluster configuration: prod-cluster
✓ Generated SSH keys: ~/.config/opencenter/clusters/my-company/secrets/ssh/prod-cluster-key
✓ Generated SOPS Age keys: ~/.config/opencenter/clusters/my-company/secrets/age/prod-cluster-key.txt

Configuration file: ~/.config/opencenter/clusters/my-company/.prod-cluster-config.yaml

Next steps:
1. Edit configuration file to customize cluster
2. Validate configuration: opencenter cluster validate prod-cluster
3. Generate GitOps repository: opencenter cluster setup prod-cluster
```

## Step 2: Configure OpenStack Credentials

Edit the configuration file to add your OpenStack credentials:

```bash
opencenter cluster edit prod-cluster
```

Update the OpenStack section:

```yaml
opencenter:
  infrastructure:
    provider: openstack
    openstack:
      # Your OpenStack region
      region: sjc3
      
      # Your OpenStack credentials
      auth_url: "https://identity.api.rackspacecloud.com/v3"
      username: "your-username"
      password: "your-password"  # Will be encrypted with SOPS
      project_name: "your-project"
      project_domain: "rackspace_cloud_domain"
      user_domain: "rackspace_cloud_domain"
      
      # Availability zone
      availability_zone: az1
      
      # Ubuntu 24.04 image ID (verify this exists in your region)
      image_id: "799dcf97-3656-4361-8187-13ab1b295e33"
      
      # Floating IP pool for external access
      floating_ip_pool: "PUBLICNET"
      
      # Network configuration
      network_name: "prod-cluster-network"
      subnet_cidr: "10.2.128.0/22"
      dns_nameservers:
        - "8.8.8.8"
        - "8.8.4.4"
```

**Finding your image ID:**

```bash
# List available Ubuntu images
openstack image list --name Ubuntu

# Use the Ubuntu 24.04 image ID
```

Save and close the editor.

## Step 3: Customize Cluster Configuration

Edit cluster-specific settings:

```yaml
opencenter:
  meta:
    name: prod-cluster
    environment: production
    region: sjc3
    organization: my-company
  
  cluster:
    # Kubernetes version
    kubernetes:
      version: "1.33.5"
    
    # Node counts (high availability)
    master_count: 3
    worker_count: 3
    
    # Instance flavors
    master_flavor: "gp.0.4.8"   # 4 vCPU, 8 GB RAM
    worker_flavor: "gp.0.4.16"  # 4 vCPU, 16 GB RAM
    bastion_flavor: "gp.0.2.2"  # 2 vCPU, 2 GB RAM
    
    # Networking
    networking:
      pod_subnet: "10.42.0.0/16"
      service_subnet: "10.43.0.0/16"
      cni_plugin: calico
      use_octavia: true  # Use OpenStack load balancer
    
    # Storage
    storage:
      default_storage_class: "csi-cinder-sc-delete"
      worker_volume_size: 40  # GB per worker
      worker_volume_type: "HA-Standard"
```

**Flavor selection tips:**
- Masters: 4 vCPU, 8 GB RAM minimum (control plane overhead)
- Workers: 4 vCPU, 16 GB RAM minimum (application workloads)
- Bastion: 2 vCPU, 2 GB RAM (SSH jump host only)

## Step 4: Configure Platform Services

Review and customize platform services:

```yaml
opencenter:
  services:
    # Core services (enabled by default)
    cert-manager:
      enabled: true
    
    keycloak:
      enabled: true
      hostname: "auth.my-company.prod-cluster.sjc3.k8s.opencenter.cloud"
      admin_password: "change-me-in-production"  # Will be encrypted
    
    kube-prometheus-stack:
      enabled: true
      grafana_admin_password: "change-me-in-production"  # Will be encrypted
    
    loki:
      enabled: true
      retention_days: 30
    
    velero:
      enabled: true
      s3_bucket: "prod-cluster-backups"
      s3_region: "sjc3"
    
    # Optional services
    harbor:
      enabled: true  # Container registry
      hostname: "harbor.my-company.prod-cluster.sjc3.k8s.opencenter.cloud"
      admin_password: "change-me-in-production"  # Will be encrypted
    
    headlamp:
      enabled: true  # Kubernetes dashboard
      hostname: "dashboard.my-company.prod-cluster.sjc3.k8s.opencenter.cloud"
```

**Service selection tips:**
- Enable cert-manager (required for TLS certificates)
- Enable Keycloak (authentication and RBAC)
- Enable monitoring (kube-prometheus-stack, Loki)
- Enable Velero (disaster recovery)
- Optional: Harbor (private registry), Headlamp (dashboard)

## Step 5: Validate Configuration

Validate your configuration before deployment:

```bash
opencenter cluster validate prod-cluster
```

**What's validated:**
1. Schema compliance (structure, types, formats)
2. Business rules (cross-field dependencies)
3. OpenStack constraints (image IDs, flavors, networks)
4. Connectivity (optional, requires credentials)

**Expected output:**

```
✓ Schema validation passed
✓ Business rules validation passed
✓ OpenStack validation passed
  - Image ID exists: 799dcf97-3656-4361-8187-13ab1b295e33
  - Flavors available: gp.0.4.8, gp.0.4.16, gp.0.2.2
  - Network quota sufficient: 1/10 networks used
  - Compute quota sufficient: 0/50 instances used

Configuration is valid and ready for deployment.
```

**If validation fails:**
- Read error messages carefully (they explain what's wrong)
- Fix issues in configuration file
- Re-run validation
- See [Troubleshooting](#troubleshooting) section below

## Step 6: Generate GitOps Repository

Generate the complete GitOps repository structure:

```bash
opencenter cluster setup prod-cluster --render
```

**What's generated:**

```
~/prod-cluster-gitops/
├── .gitignore
├── .sops.yaml                     # SOPS encryption rules
├── README.md
│
├── applications/
│   └── overlays/prod-cluster/
│       ├── flux-system/           # FluxCD bootstrap
│       ├── services/              # Platform services
│       └── managed-services/      # Customer applications
│
└── infrastructure/
    └── clusters/prod-cluster/
        ├── main.tf                # Terraform/OpenTofu
        ├── provider.tf
        ├── variables.tf
        ├── inventory/             # Kubespray Ansible
        └── credentials/           # Encrypted credentials
```

**Output:**

```
✓ Generated GitOps repository: ~/prod-cluster-gitops
✓ Encrypted secrets with SOPS
✓ Created Terraform configuration
✓ Created Kubespray inventory
✓ Created FluxCD manifests

Next steps:
1. Review generated files
2. Initialize Git repository: cd ~/prod-cluster-gitops && git init
3. Commit files: git add . && git commit -m "Initial cluster configuration"
4. Push to Git: git remote add origin <your-repo-url> && git push -u origin main
5. Bootstrap cluster: opencenter cluster bootstrap prod-cluster
```

## Step 7: Initialize Git Repository

Initialize and push to Git (GitOps requires Git):

```bash
# Navigate to GitOps repository
cd ~/prod-cluster-gitops

# Initialize Git
git init

# Add all files
git add .

# Commit
git commit -m "Initial prod-cluster configuration"

# Add remote (replace with your Git repository URL)
git remote add origin git@github.com:my-company/prod-cluster-gitops.git

# Push to remote
git push -u origin main
```

**Why Git is required:**
- FluxCD pulls configuration from Git
- Git provides audit trail (who changed what, when)
- Git enables rollback (revert commits)
- Git enables collaboration (pull requests)

## Step 8: Bootstrap Cluster

Deploy the cluster (this takes 30-45 minutes):

```bash
opencenter cluster bootstrap prod-cluster
```

**What happens:**

```
Phase 1: Infrastructure Provisioning (10-15 minutes)
  ✓ Creating network and subnet
  ✓ Creating router and external gateway
  ✓ Creating security groups
  ✓ Provisioning bastion host
  ✓ Provisioning 3 control plane nodes
  ✓ Provisioning 3 worker nodes
  ✓ Attaching volumes to workers
  ✓ Assigning floating IPs

Phase 2: Kubernetes Deployment (15-20 minutes)
  ✓ Installing dependencies (Python, Docker, etc.)
  ✓ Configuring control plane nodes
  ✓ Deploying etcd cluster
  ✓ Deploying Kubernetes API server
  ✓ Deploying Kubernetes controllers
  ✓ Joining worker nodes
  ✓ Installing Calico CNI
  ✓ Installing OpenStack CSI driver

Phase 3: GitOps Bootstrap (5-10 minutes)
  ✓ Installing FluxCD controllers
  ✓ Creating GitRepository sources
  ✓ Deploying platform services
  ✓ Waiting for services to be ready

Cluster is ready!
```

**Monitor progress:**

```bash
# In another terminal, watch cluster creation
watch -n 5 'openstack server list | grep prod-cluster'

# After Kubernetes is deployed, watch pods
export KUBECONFIG=~/prod-cluster-gitops/infrastructure/clusters/prod-cluster/kubeconfig.yaml
watch -n 5 'kubectl get pods -A'
```

## Step 9: Verify Cluster

Verify the cluster is working:

```bash
# Set kubeconfig
export KUBECONFIG=~/prod-cluster-gitops/infrastructure/clusters/prod-cluster/kubeconfig.yaml

# Check nodes
kubectl get nodes

# Expected output:
# NAME                        STATUS   ROLES           AGE   VERSION
# prod-cluster-master-1       Ready    control-plane   20m   v1.33.5
# prod-cluster-master-2       Ready    control-plane   20m   v1.33.5
# prod-cluster-master-3       Ready    control-plane   20m   v1.33.5
# prod-cluster-worker-1       Ready    <none>          18m   v1.33.5
# prod-cluster-worker-2       Ready    <none>          18m   v1.33.5
# prod-cluster-worker-3       Ready    <none>          18m   v1.33.5

# Check platform services
kubectl get helmreleases -A

# Expected output: 20+ HelmReleases in Ready state

# Check FluxCD reconciliation
flux get kustomizations

# Expected output: All Kustomizations in Ready state
```

**All checks passed?** Your cluster is ready for production workloads!

## Step 10: Access Services

Access platform services via their hostnames:

**Keycloak (Authentication):**
```
URL: https://auth.my-company.prod-cluster.sjc3.k8s.opencenter.cloud
Username: admin
Password: (from configuration file)
```

**Grafana (Monitoring):**
```
URL: https://grafana.my-company.prod-cluster.sjc3.k8s.opencenter.cloud
Username: admin
Password: (from configuration file)
```

**Headlamp (Dashboard):**
```
URL: https://dashboard.my-company.prod-cluster.sjc3.k8s.opencenter.cloud
```

**Harbor (Container Registry):**
```
URL: https://harbor.my-company.prod-cluster.sjc3.k8s.opencenter.cloud
Username: admin
Password: (from configuration file)
```

**DNS Configuration:**

For production, configure DNS records:

```
auth.my-company.prod-cluster.sjc3.k8s.opencenter.cloud     → <load-balancer-ip>
grafana.my-company.prod-cluster.sjc3.k8s.opencenter.cloud  → <load-balancer-ip>
dashboard.my-company.prod-cluster.sjc3.k8s.opencenter.cloud → <load-balancer-ip>
harbor.my-company.prod-cluster.sjc3.k8s.opencenter.cloud   → <load-balancer-ip>
```

Get load balancer IP:

```bash
kubectl get svc -n gateway gateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

## Check Your Work

Verify everything is working:

- [ ] All 6 nodes are Ready
- [ ] All platform services are deployed (HelmReleases Ready)
- [ ] FluxCD is reconciling (Kustomizations Ready)
- [ ] Can access Keycloak UI
- [ ] Can access Grafana UI
- [ ] Can access Headlamp UI
- [ ] DNS records configured (production only)

## Troubleshooting

### Validation Fails: Image ID Not Found

**Error:**
```
Error: Image ID not found in OpenStack region sjc3
Image ID: 799dcf97-3656-4361-8187-13ab1b295e33
```

**Solution:**

```bash
# List available Ubuntu images
openstack image list --name Ubuntu

# Update configuration with correct image ID
opencenter cluster edit prod-cluster
```

### Validation Fails: Insufficient Quota

**Error:**
```
Error: Insufficient compute quota
Required: 6 instances, 24 vCPUs, 96 GB RAM
Available: 2 instances, 8 vCPUs, 16 GB RAM
```

**Solution:**
- Request quota increase from OpenStack administrator
- Or reduce cluster size (fewer workers, smaller flavors)

### Bootstrap Fails: Terraform Error

**Error:**
```
Error: Error creating OpenStack server: Quota exceeded
```

**Solution:**

```bash
# Check current usage
openstack quota show

# Clean up any existing resources
cd ~/prod-cluster-gitops/infrastructure/clusters/prod-cluster
terraform destroy

# Retry bootstrap
opencenter cluster bootstrap prod-cluster
```

### Services Not Deploying: FluxCD Error

**Error:**
```
Kustomization cert-manager-base: reconciliation failed
```

**Solution:**

```bash
# Check FluxCD logs
kubectl logs -n flux-system deployment/kustomize-controller

# Check GitRepository status
kubectl describe gitrepository opencenter-cert-manager -n flux-system

# Force reconciliation
flux reconcile kustomization cert-manager-base
```

## Next Steps

Now that you have a production cluster, explore these topics:

**Deploy Applications:**
- [Customize Services](../how-to/customize-services.md) - Configure platform services
- [Manage Secrets](../how-to/manage-secrets.md) - Encrypt and rotate secrets

**Cluster Management:**
- [Add Worker Pools](../how-to/add-worker-pools.md) - Scale cluster capacity
- [Backup and Restore](../how-to/backup-and-restore.md) - Configure disaster recovery
- [Upgrade Kubernetes](../how-to/upgrade-kubernetes.md) - Upgrade cluster version

**Multi-Cluster:**
- [Multi-Cluster Setup](multi-cluster-setup.md) - Manage multiple clusters

**Understanding:**
- [GitOps Workflow](../explanation/gitops-workflow.md) - How GitOps works
- [Security Model](../explanation/security-model.md) - Security architecture

## What You Learned

In this tutorial, you:

- Initialized an OpenStack cluster configuration
- Configured OpenStack credentials and cluster settings
- Validated configuration before deployment
- Generated a complete GitOps repository
- Deployed a production Kubernetes cluster
- Verified cluster health and service deployment
- Accessed platform services

You now have a production-ready Kubernetes cluster on OpenStack with GitOps continuous delivery!

---

## Evidence

This tutorial is based on:

- OpenStack defaults: `internal/config/defaults.go:68-157`
- Workflow validation: `tests/features/workflow.feature:1-73`
- Provider documentation: `docs/providers/README.md:7`
- Bootstrap process: `cmd/cluster_bootstrap.go`
- GitOps structure: `internal/gitops/`, Ecosystem.md
- Service configuration: `internal/config/defaults.go:293-388`
