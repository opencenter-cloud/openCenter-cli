---
title: Local Development with Kind
doc_type: tutorial
weight: 20
---

# Local Development with Kind


## Table of Contents

- [What You'll Learn](#what-youll-learn)
- [Prerequisites](#prerequisites)
- [Step 1: Install Kind](#step-1-install-kind)
- [Step 2: Create a Kind Cluster](#step-2-create-a-kind-cluster)
- [Step 3: Initialize openCenter Configuration](#step-3-initialize-opencenter-configuration)
- [Step 4: Customize for Local Development](#step-4-customize-for-local-development)
- [Step 5: Set Up GitOps Repository](#step-5-set-up-gitops-repository)
- [Step 6: Validate Configuration](#step-6-validate-configuration)
- [Step 7: Bootstrap the Cluster](#step-7-bootstrap-the-cluster)
- [Step 8: Verify Cluster Status](#step-8-verify-cluster-status)
- [Step 9: Test GitOps Workflow](#step-9-test-gitops-workflow)
- [Step 10: Access Cluster Services](#step-10-access-cluster-services)
- [Working with Multiple Local Clusters](#working-with-multiple-local-clusters)
- [Debugging Tips](#debugging-tips)
- [Cleaning Up](#cleaning-up)
- [Next Steps](#next-steps)
- [Troubleshooting](#troubleshooting)
- [Additional Resources](#additional-resources)
This tutorial walks you through setting up a local Kubernetes cluster using Kind (Kubernetes in Docker) for testing openCenter configurations without cloud infrastructure costs.

## What You'll Learn

By the end of this tutorial, you'll be able to:

- Install and configure Kind for local development
- Create a multi-node Kubernetes cluster on your workstation
- Initialize an openCenter cluster configuration for Kind
- Test GitOps workflows locally before deploying to production
- Debug cluster configurations in a safe environment

## Prerequisites

Before starting, ensure you have:

- Docker or Podman installed and running
- At least 8GB of available RAM
- 20GB of free disk space
- openCenter CLI installed (`mise run build`)

## Step 1: Install Kind

openCenter includes Kind as a managed tool through mise. Install it with:

```bash
mise install kind
```

Verify the installation:

```bash
kind version
```

You should see output showing the Kind version (e.g., `kind v0.20.0`).

## Step 2: Create a Kind Cluster

openCenter provides a pre-configured mise task for creating Kind clusters with the right settings:

```bash
mise run kind-cluster-no-cni
```

This command creates a cluster named `opencenter-dev` with:

- One control plane node
- Three worker nodes
- No default CNI (you'll install Calico through openCenter)
- Custom pod and service subnets matching openCenter defaults

The cluster nodes will show `NotReady` status until you install a CNI plugin. This is expected.

### Using Podman Instead of Docker

If you prefer Podman over Docker, the mise configuration already sets the required environment variable:

```bash
# Already configured in .mise.toml
KIND_EXPERIMENTAL_PROVIDER=podman
```

The `kind-cluster-no-cni` task automatically uses Podman when available.

## Step 3: Initialize openCenter Configuration

Create a new cluster configuration for your Kind cluster:

```bash
./bin/openCenter cluster init kind-demo \
  --org local \
  --opencenter.infrastructure.provider=kind \
  --opencenter.cluster.kubernetes.version=1.33.5
```

This creates a configuration in `~/.config/openCenter/clusters/local/` with:

- Organization: `local` (for local development clusters)
- Provider: `kind` (uses Kind-specific bootstrap logic)
- Kubernetes version: 1.33.5 (matches Kind's default)

### Understanding the Directory Structure

Your local development cluster follows the organization-based structure:

```
~/.config/openCenter/clusters/local/
├── .kind-demo-config.yaml          # Cluster configuration
├── infrastructure/
│   └── clusters/
│       └── kind-demo/              # Cluster-specific files
├── applications/
│   └── overlays/
│       └── kind-demo/              # Application manifests
└── secrets/
    ├── age/
    │   └── keys/
    │       └── kind-demo-key.txt   # SOPS encryption key
    └── ssh/
        └── kind-demo-dev-local     # SSH keys
```

## Step 4: Customize for Local Development

Edit the configuration to optimize for local development:

```bash
./bin/openCenter cluster update kind-demo \
  --opencenter.cluster.kubernetes.master_count=1 \
  --opencenter.cluster.kubernetes.worker_count=2 \
  --opencenter.cluster.kubernetes.flavor_master=local \
  --opencenter.cluster.kubernetes.flavor_worker=local
```

These settings reduce resource usage for local testing:

- Single control plane node (instead of 3 for HA)
- Two worker nodes (instead of production defaults)
- Local flavors (no cloud provider sizing)

## Step 5: Set Up GitOps Repository

Initialize the GitOps repository structure:

```bash
./bin/openCenter cluster setup kind-demo
```

This command:

1. Creates the GitOps directory structure
2. Generates Flux manifests for continuous deployment
3. Copies application templates
4. Configures SOPS for secret encryption
5. Initializes a local Git repository

The GitOps repository is created at:

```
~/.config/openCenter/clusters/local/
```

This directory becomes your GitOps repository root, containing both infrastructure and application manifests.

## Step 6: Validate Configuration

Before bootstrapping, validate your configuration:

```bash
./bin/openCenter cluster validate kind-demo
```

The validator checks:

- Schema compliance
- Required fields
- Provider-specific settings
- Network configuration
- Service dependencies

Fix any validation errors before proceeding.

## Step 7: Bootstrap the Cluster

Deploy your configuration to the Kind cluster:

```bash
./bin/openCenter cluster bootstrap kind-demo
```

For Kind clusters, this command:

1. Applies the cluster configuration to Kind
2. Installs Calico CNI
3. Deploys Flux for GitOps
4. Applies infrastructure manifests
5. Configures cluster services

The bootstrap process takes 5-10 minutes. Watch the progress:

```bash
kubectl get pods -A --watch
```

## Step 8: Verify Cluster Status

Check that all components are running:

```bash
./bin/openCenter cluster status kind-demo
```

You should see:

- Cluster status: `deployed`
- All nodes: `Ready`
- Core services: `Running`

Verify Flux is syncing:

```bash
kubectl get gitrepositories -n flux-system
kubectl get kustomizations -n flux-system
```

## Step 9: Test GitOps Workflow

Make a change to test the GitOps workflow:

1. Edit an application manifest:

```bash
cd ~/.config/openCenter/clusters/local/applications/overlays/kind-demo
vim my-app.yaml
```

2. Commit the change:

```bash
git add my-app.yaml
git commit -m "Update my-app configuration"
```

3. Watch Flux reconcile:

```bash
flux reconcile kustomization apps --with-source
```

4. Verify the change:

```bash
kubectl get deployment my-app -o yaml
```

## Step 10: Access Cluster Services

Set up your environment to access the cluster:

```bash
eval $(./bin/openCenter cluster select kind-demo --activate --export-only)
```

This configures:

- `KUBECONFIG`: Points to Kind cluster kubeconfig
- `PATH`: Includes cluster-specific binaries
- Cloud credentials: Not needed for Kind

Test access:

```bash
kubectl get nodes
kubectl get pods -A
```

## Working with Multiple Local Clusters

You can run multiple Kind clusters simultaneously for testing different configurations:

```bash
# Create a second cluster
kind create cluster --name opencenter-staging

# Initialize configuration
./bin/openCenter cluster init kind-staging --org local

# Switch between clusters
./bin/openCenter cluster select kind-demo
./bin/openCenter cluster select kind-staging
```

List all local clusters:

```bash
./bin/openCenter cluster list
kind get clusters
```

## Debugging Tips

### Cluster Won't Start

If nodes remain `NotReady`:

```bash
# Check CNI installation
kubectl get pods -n kube-system | grep calico

# View CNI logs
kubectl logs -n kube-system -l k8s-app=calico-node
```

### GitOps Not Syncing

If Flux isn't reconciling:

```bash
# Check Flux status
flux check

# View reconciliation logs
kubectl logs -n flux-system deploy/source-controller
kubectl logs -n flux-system deploy/kustomize-controller
```

### Resource Constraints

If your workstation struggles with the cluster:

```bash
# Reduce worker nodes
./bin/openCenter cluster update kind-demo \
  --opencenter.cluster.kubernetes.worker_count=1

# Disable resource-intensive services
./bin/openCenter cluster update kind-demo \
  --opencenter.services.kube-prometheus-stack.enabled=false
```

## Cleaning Up

When you're done testing, clean up resources:

```bash
# Delete the Kind cluster
kind delete cluster --name opencenter-dev

# Remove openCenter configuration (optional)
./bin/openCenter cluster destroy kind-demo
```

The destroy command removes:

- Cluster configuration files
- GitOps repository
- Generated secrets
- Local state

## Next Steps

Now that you have a local development environment:

- **Test Configuration Changes**: Validate changes locally before deploying to production
- **Develop Custom Applications**: Use the local cluster to develop and test applications
- **Learn GitOps Patterns**: Experiment with Flux and Kustomize workflows
- **Multi-Cluster Management**: Set up multiple local clusters to simulate production environments

For production deployments, see:

- [OpenStack Deployment Guide](../how-to/deploy-openstack.md)
- [Managing Multiple Clusters](multi-cluster.md)
- [GitOps Best Practices](../how-to/gitops-workflow.md)

## Troubleshooting

### Kind Cluster Creation Fails

**Problem**: `kind create cluster` fails with network errors.

**Solution**: Check Docker/Podman networking:

```bash
# For Docker
docker network ls
docker network inspect kind

# For Podman
podman network ls
podman network inspect kind
```

### SOPS Encryption Errors

**Problem**: Cannot encrypt secrets with SOPS.

**Solution**: Verify Age key exists:

```bash
ls -la ~/.config/openCenter/clusters/local/secrets/age/keys/
cat ~/.config/openCenter/clusters/local/secrets/age/keys/kind-demo-key.txt
```

If missing, regenerate:

```bash
./bin/openCenter cluster init kind-demo --regenerate-keys --force
```

### Flux Bootstrap Fails

**Problem**: Flux installation fails during bootstrap.

**Solution**: Check cluster connectivity:

```bash
kubectl cluster-info
kubectl get nodes

# Manually install Flux
flux install --export > flux-system.yaml
kubectl apply -f flux-system.yaml
```

## Additional Resources

- [Kind Documentation](https://kind.sigs.k8s.io/)
- [Flux Documentation](https://fluxcd.io/docs/)
- [Calico Documentation](https://docs.tigera.io/calico/latest/about/)
- [openCenter Configuration Reference](../reference/config.md)
