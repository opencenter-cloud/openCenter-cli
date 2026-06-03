---
id: getting-started
title: "Getting Started with openCenter"
sidebar_label: Getting Started with openCenter
description: Create your first Kubernetes cluster using openCenter from installation through deployment.
doc_type: tutorial
audience: "new users, platform engineers"
tags: [getting-started, tutorial, installation, first-cluster]
---
# Getting Started with openCenter

**Purpose:** For new users, shows how to create your first Kubernetes cluster using openCenter, covering installation through deployment.

This tutorial walks you through creating your first Kubernetes cluster with openCenter. You’ll learn the core workflow: initialize, validate, setup, and bootstrap. By the end, you’ll have a working cluster and understand how to manage it.

## What You’ll Build

A production-ready Kubernetes cluster with:

* 3 control plane nodes for high availability
* 2 worker nodes for running workloads
* GitOps-based configuration management with FluxCD
* 20+ platform services (monitoring, logging, ingress, etc.)
* Encrypted secrets management with SOPS

Time to complete: 10-15 minutes (plus infrastructure provisioning time)

## Prerequisites

Before starting, ensure you have:

1. **_Mise installed_** - Tool version manager and task runner

   ```bash
   # macOS
   brew install mise

   # Linux
   curl https://mise.run | sh
   ```
2. **_Git installed_** - Version control

   ```bash
   git --version  # Should show git version 2.x or higher
   ```
3. **_Infrastructure access_** - One of:
   * OpenStack cloud credentials (recommended for production)
   * Kind installed (for local development)
   * VMware vSphere with pre-provisioned VMs
   * AWS credentials (experimental)
4. **_SSH key_** - For cluster access

   ```bash
   # Generate if you don’t have one
   ssh-keygen -t ed25519 -C "your-email@example.com"
   ```

## Step 1: Install openCenter

Clone the repository and build the CLI:

```bash
# Clone repository
git clone https://github.com/opencenter-cloud/openCenter-cli.git
cd openCenter-cli

# Install project tools
mise install

# Build the binary
mise run build

# Verify installation
./bin/opencenter version
```

You should see version information including git commit and build date.

## Step 2: Initialize Your First Cluster

Create a new cluster configuration with sensible defaults:

```bash
# Initialize cluster named "demo" in organization "my-org"
./bin/opencenter cluster init demo --org my-org

# The configuration is created at:
# ~/.config/opencenter/clusters/my-org/.demo-config.yaml
```

The `init` command creates a complete configuration file with:

* Default Kubernetes version (1.33.5)
* 3 control plane nodes, 2 worker nodes
* Calico CNI networking
* 20+ platform services enabled
* OpenStack as default provider

## Step 3: Configure Your Cluster

Edit the configuration file to match your environment:

```bash
# Open configuration in your editor
$EDITOR ~/.config/opencenter/clusters/my-org/.demo-config.yaml
```

<mark>#</mark> Minimum Required Configuration

For OpenStack, you must set:

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
        floating_network_id: "your-floating-network-id"

secrets:
  global:
    openstack:
      application_credential_id: "your-app-cred-id"
      application_credential_secret: "your-app-cred-secret"
```

For Kind (local development):

```yaml
opencenter:
  infrastructure:
    provider: kind
```

<mark>#</mark> Optional: Customize Services

Enable or disable platform services:

```yaml
opencenter:
  services:
    keycloak:
      enabled: true  # Identity and access management
    kube-prometheus-stack:
      enabled: true  # Monitoring and alerting
    loki:
      enabled: true  # Log aggregation
```

See [Platform Services Reference](../reference/platform-services.md) for complete list.

## Step 4: Select Your Cluster

Make this cluster the active one:

```bash
./bin/opencenter cluster use my-org/demo
```

This creates `~/.config/opencenter/active` pointing to your cluster. Now you can run commands without specifying the cluster name.

## Step 5: Validate Configuration

Check for errors before deployment:

```bash
./bin/opencenter cluster validate
```

The validator checks:

* **_Schema compliance_** - Configuration matches JSON schema
* **_Business rules_** - Logical consistency (e.g., VRRP IP required when Octavia disabled)
* **_Provider requirements_** - Provider-specific constraints
* **_Network topology_** - Subnet overlaps, CIDR validity

<mark>#</mark> Common Validation Errors

**_Missing VRRP IP:_**

```
ERROR: vrrp_ip must be set when use_octavia=false and vrrp_enabled=true
```

Fix by setting:

```yaml
opencenter:
  cluster:
    networking:
      vrrp_ip: "10.0.0.10"
```

**_Invalid CIDR:_**

```
ERROR: subnet_pods overlaps with subnet_services
```

Fix by using non-overlapping subnets:

```yaml
opencenter:
  cluster:
    kubernetes:
      subnet_pods: "10.42.0.0/16"
      subnet_services: "10.43.0.0/16"
```

## Step 6: Setup GitOps Repository

Generate the GitOps repository structure:

```bash
./bin/opencenter cluster generate
```

This creates:

```
&lt;git_dir>/
├── .gitignore
├── applications/
│   └── overlays/demo/
│       ├── flux-system/          # FluxCD bootstrap
│       ├── services/              # Platform services
│       └── managed-services/      # Your applications
└── infrastructure/
    └── clusters/demo/
        ├── main.tf                # Terraform/OpenTofu
        ├── inventory/             # Kubespray Ansible
        └── kubeconfig.yaml        # Cluster access (generated later)
```

The `--render` flag materializes templates with your configuration values.

## Step 7: Initialize Git Repository

Create a Git repository for GitOps:

```bash
# Navigate to git directory (from your config)
cd &lt;git_dir>

# Initialize Git
git init
git add .
git commit -m "Initial cluster configuration"

# Add remote (optional but recommended)
git remote add origin &lt;your-git-url>
git push -u origin main
```

## Step 8: Bootstrap Cluster

Deploy the cluster:

```bash
./bin/opencenter cluster deploy
```

This command:

1. **_Provisions infrastructure_** - Creates VMs, networks, load balancers (OpenStack/AWS)
2. **_Deploys Kubernetes_** - Runs Kubespray Ansible playbooks
3. **_Bootstraps FluxCD_** - Installs GitOps controller
4. **_Deploys services_** - FluxCD reconciles platform services from gitops-base

<mark>#</mark> What Happens During Bootstrap

**_Phase 1: Infrastructure (5-10 minutes)_**

* Creates OpenStack VMs or Kind containers
* Configures networking and security groups
* Provisions storage volumes

**_Phase 2: Kubernetes (10-15 minutes)_**

* Installs container runtime (ContainerD)
* Deploys control plane components
* Joins worker nodes
* Configures CNI networking (Calico)

**_Phase 3: GitOps (2-5 minutes)_**

* Installs FluxCD controllers
* Creates GitRepository sources
* Deploys Kustomization resources

**_Phase 4: Services (10-20 minutes)_**

* FluxCD reconciles platform services
* Deploys cert-manager, Keycloak, Prometheus, etc.
* Configures ingress and TLS certificates

Total time: 30-50 minutes depending on provider and service count.

## Step 9: Verify Deployment

Check cluster status:

```bash
# Get kubeconfig
export KUBECONFIG=&lt;git_dir>/infrastructure/clusters/demo/kubeconfig.yaml

# Check nodes
kubectl get nodes

# Check FluxCD
kubectl get kustomizations -n flux-system

# Check platform services
kubectl get helmreleases -A
```

Expected output:

```
NAME       STATUS   AGE
demo-cp-1  Ready    15m
demo-cp-2  Ready    15m
demo-cp-3  Ready    15m
demo-wn-1  Ready    12m
demo-wn-2  Ready    12m
```

## Step 10: Access Services

Platform services are available at:

* **_Headlamp (Dashboard):_** `https://dashboard.<org>.<cluster>.<region>.k8s.opencenter.cloud`
* **_Keycloak (Auth):_** `https://auth.<org>.<cluster>.<region>.k8s.opencenter.cloud`
* **_Grafana (Monitoring):_** `https://grafana.<org>.<cluster>.<region>.k8s.opencenter.cloud`

Default credentials are in your configuration file under `secrets` section.

## Check Your Work

Verify everything is working:

1. **_Nodes are Ready:_**

   ```bash
   kubectl get nodes
   # All nodes should show STATUS: Ready
   ```
2. **_FluxCD is reconciling:_**

   ```bash
   kubectl get kustomizations -n flux-system
   # All should show READY: True
   ```
3. **_Services are deployed:_**

   ```bash
   kubectl get helmreleases -A
   # All should show STATUS: deployed
   ```
4. **_Pods are running:_**

   ```bash
   kubectl get pods -A
   # Most pods should show STATUS: Running
   ```

## Next Steps

Now that you have a working cluster, explore:

* **_[Add Worker Pools](../operations/add-worker-pools.md)_** - Scale your cluster
* **_[Manage Secrets](../operations/manage-secrets.md)_** - Rotate encryption keys
* **_[Customize Services](../operations/customize-services.md)_** - Configure platform services
* **_[Deploy Applications](../index.md)_** - Add your workloads

## Troubleshooting

<mark>#</mark> Validation Fails

**_Problem:_** `opencenter cluster validate` shows errors

**_Solution:_** Read error messages carefully. Common issues:

* Missing required fields (credentials, network IDs)
* Invalid CIDR ranges or overlapping subnets
* Provider-specific constraints not met

See [Troubleshooting Guide](../operations/troubleshoot-deployment.md#validation-errors) for details.

<mark>#</mark> Bootstrap Hangs

**_Problem:_** `opencenter cluster deploy` appears stuck

**_Solution:_** Check logs:

```bash
# Terraform logs
tail -f &lt;git_dir>/infrastructure/clusters/demo/terraform.log

# Ansible logs
tail -f &lt;git_dir>/infrastructure/clusters/demo/ansible.log
```

Common causes:

* Network connectivity issues
* Insufficient cloud quotas
* SSH key not accessible

<mark>#</mark> Services Not Deploying

**_Problem:_** FluxCD shows errors, services not reconciling

**_Solution:_** Check FluxCD status:

```bash
kubectl get kustomizations -n flux-system
kubectl describe kustomization &lt;name> -n flux-system
```

Common causes:

* Git repository not accessible (SSH key issue)
* SOPS decryption failure (Age key not found)
* Service configuration errors

See [Troubleshooting Guide](../operations/troubleshoot-deployment.md) for complete solutions.

## Summary

You’ve successfully:

* Installed openCenter CLI
* Initialized a cluster configuration
* Validated configuration for errors
* Generated GitOps repository structure
* Deployed a production-ready Kubernetes cluster
* Verified cluster and services are running

The cluster is now managed through GitOps. All changes should be made by updating the configuration file and running `opencenter cluster generate` to regenerate manifests.

---

## Evidence

This tutorial is based on:

* Workflow validation: `tests/features/workflow.feature:1-73`
* Configuration defaults: `internal/config/defaults.go:48-451`
* Product workflow: `.kiro/steering/product.md:16-22`
* Build system: `.kiro/steering/tech.md:52-91`
* Project structure: `.kiro/steering/structure.md:118-128`
* Session 1 codebase review: A1-A11
* Session 2 facts inventory: B0