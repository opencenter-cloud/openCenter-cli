---
id: kind-local-development
title: "Set Up Local Development with Kind"
sidebar_label: Set Up Local
description: Set up a local Kubernetes development environment using Kind and Docker.
doc_type: tutorial
audience: "developers, platform engineers"
tags: [kind, local, development, docker]
---
# Set Up Local Development with Kind

**Purpose:** For developers, shows how to set up a local Kubernetes development environment using Kind, covering prerequisites through testing.

By the end of this tutorial, you’ll have a local Kubernetes cluster running in Docker containers, perfect for development and testing without cloud costs.

**Time:** 15-20 minutes

## What You’ll Build

A local development cluster with:

* 1 control plane node
* 2 worker nodes
* Calico CNI networking
* MetalLB load balancer (simulated)
* Core platform services (cert-manager, Gateway API, monitoring)
* FluxCD GitOps (optional)

## Prerequisites

Before starting, ensure you have:

**Local Tools:**

* Docker Desktop installed and running
* openCenter CLI installed
* Git installed
* 8 GB RAM available (minimum)
* 20 GB disk space available

**Verify Docker:**

```bash
# Check Docker is running
docker ps

# Check Docker resources
docker info | grep -E 'CPUs|Total Memory'
```

**Expected output:**

```
CPUs: 4
Total Memory: 16 GiB
```

If Docker is running and has sufficient resources, you’re ready to proceed.

## Step 1: Initialize Kind Cluster Configuration

Create a new cluster configuration with Kind defaults:

```bash
opencenter cluster init dev-cluster \
  --org local \
  --type kind \
  --kind-disable-default-cni
```

**What happens:**

* Creates configuration file at `~/.config/opencenter/clusters/local/.dev-cluster-config.yaml`
* Applies Kind defaults (single control plane, 2 workers)
* Sets `opencenter.infrastructure.kind.disable_default_cni: true` so openCenter can install the cluster CNI during bootstrap
* Generates SSH keys (not used for Kind, but required for consistency)
* Generates SOPS Age keys for secrets encryption

**Output:**

```
✓ Created cluster configuration: dev-cluster
✓ Generated SSH keys: ~/.config/opencenter/clusters/local/secrets/ssh/dev-cluster-key
✓ Generated SOPS Age keys: ~/.config/opencenter/clusters/local/secrets/age/dev-cluster-key.txt

Configuration file: ~/.config/opencenter/clusters/local/.dev-cluster-config.yaml

Next steps:
1. Edit configuration file to customize cluster (optional)
2. Validate configuration: opencenter cluster validate dev-cluster
3. Generate GitOps repository: opencenter cluster generate dev-cluster
```

## Step 2: Customize Cluster Configuration (Optional)

For development, the defaults are usually fine, but you can customize:

```bash
opencenter cluster edit dev-cluster
```

**Common customizations:**

```yaml
opencenter:
  meta:
    name: dev-cluster
    environment: development
    organization: local

  infrastructure:
    provider: kind
    kind:
      cluster_name: dev-cluster
      control_plane_count: 1  # Single control plane for dev
      worker_count: 2         # 2 workers for testing
      disable_default_cni: true

  cluster:
    kubernetes:
      version: "1.33.5"

    networking:
      pod_subnet: "10.42.0.0/16"
      service_subnet: "10.43.0.0/16"
      cni_plugin: calico

  services:
    # Core services (lightweight for dev)
    cert-manager:
      enabled: true

    gateway-api:
      enabled: true

    gateway:
      enabled: true

    # Monitoring (optional, uses resources)
    kube-prometheus-stack:
      enabled: false  # Disable for faster startup

    loki:
      enabled: false  # Disable for faster startup

    # Development tools
    headlamp:
      enabled: true  # Kubernetes dashboard
      hostname: "dashboard.local.dev-cluster.localhost"
```

**Resource optimization tips:**

* Disable monitoring (kube-prometheus-stack, Loki) for faster startup
* Enable only services you need for development
* Use 1 control plane node (no HA needed for dev)
* Use 2 worker nodes (enough for testing)

## Step 3: Validate Configuration

Validate your configuration:

```bash
opencenter cluster validate dev-cluster
```

**What’s validated:**

1. Schema compliance (structure, types, formats)
2. Business rules (cross-field dependencies)
3. Kind constraints (node counts, networking)

**Expected output:**

```
✓ Schema validation passed
✓ Business rules validation passed
✓ Kind validation passed
  - Docker is running
  - Docker has sufficient resources (8 GB RAM, 20 GB disk)
  - Kind CLI is available

Configuration is valid and ready for deployment.
```

**📌 NOTE**\
Kind validation is fast (no cloud API calls).

## Step 4: Generate GitOps Repository

Generate the GitOps repository structure:

```bash
opencenter cluster generate dev-cluster
```

**What’s generated:**

```
~/dev-cluster-gitops/
├── .gitignore
├── .sops.yaml
├── README.md
│
├── applications/
│   └── overlays/dev-cluster/
│       ├── services/              # Platform services
│       └── managed-services/      # Your applications
│
└── infrastructure/
    └── clusters/dev-cluster/
        ├── kind-config.yaml       # Kind cluster configuration
        └── kubeconfig.yaml        # Generated after deployment
```

**Output:**

```
✓ Generated GitOps repository: ~/dev-cluster-gitops
✓ Created Kind configuration

Next steps:
1. Bootstrap cluster: opencenter cluster deploy dev-cluster
```

**📌 NOTE**\
For local development, Git repository is optional (but recommended for GitOps workflow).

## Step 5: Bootstrap Cluster

Deploy the cluster (this takes 5-10 minutes):

```bash
opencenter cluster deploy dev-cluster
```

**What happens:**

```
Phase 1: Kind Cluster Creation (2-3 minutes)
  ✓ Creating Kind cluster with 3 nodes
  ✓ Loading container images
  ✓ Configuring networking
  ✓ Generating kubeconfig

Phase 2: CNI Installation (1-2 minutes)
  ✓ Installing Calico CNI
  ✓ Waiting for Calico pods to be ready

Phase 3: Platform Services (2-5 minutes)
  ✓ Installing cert-manager
  ✓ Installing Gateway API
  ✓ Installing MetalLB
  ✓ Installing Headlamp (if enabled)

Cluster is ready!
```

**Monitor progress:**

```bash
# In another terminal, watch Docker containers
watch -n 2 'docker ps | grep dev-cluster'

# After cluster is created, watch pods
export KUBECONFIG=~/dev-cluster-gitops/infrastructure/clusters/dev-cluster/kubeconfig.yaml
watch -n 2 'kubectl get pods -A'
```

## Step 6: Verify Cluster

Verify the cluster is working:

```bash
# Set kubeconfig
export KUBECONFIG=~/dev-cluster-gitops/infrastructure/clusters/dev-cluster/kubeconfig.yaml

# Check nodes
kubectl get nodes

# Expected output:
# NAME                        STATUS   ROLES           AGE   VERSION
# dev-cluster-control-plane   Ready    control-plane   5m    v1.33.5
# dev-cluster-worker          Ready    <none>          4m    v1.33.5
# dev-cluster-worker2         Ready    <none>          4m    v1.33.5

# Check platform services
kubectl get pods -A

# Expected output: All pods Running

# Check cert-manager
kubectl get certificates -A

# Check Gateway API
kubectl get gatewayclasses
```

**All checks passed?** Your local cluster is ready for development!

## Step 7: Access Services

Access platform services locally:

**Headlamp (Dashboard):**

```bash
# Port-forward Headlamp
kubectl port-forward -n headlamp svc/headlamp 8080:80

# Open browser
open http://localhost:8080
```

**Kubernetes API:**

```bash
# API is accessible via kubeconfig
kubectl cluster-info

# Expected output:
# Kubernetes control plane is running at https://127.0.0.1:<random-port>
```

**Load Balancer Services:**

Kind uses MetalLB to simulate load balancers. Services get IPs from Docker network:

```bash
# Check MetalLB IP pool
kubectl get ipaddresspool -n metallb-system

# Create a test service
kubectl create deployment nginx --image=nginx
kubectl expose deployment nginx --port=80 --type=LoadBalancer

# Get load balancer IP
kubectl get svc nginx

# Access service
curl http://<EXTERNAL-IP>
```

## Step 8: Deploy a Test Application

Deploy a simple application to test the cluster:

```bash
# Create namespace
kubectl create namespace test-app

# Deploy application
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello-world
  namespace: test-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: hello-world
  template:
    metadata:
      labels:
        app: hello-world
    spec:
      containers:
      - name: hello-world
        image: gcr.io/google-samples/hello-app:1.0
        ports:
        - containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: hello-world
  namespace: test-app
spec:
  type: LoadBalancer
  selector:
    app: hello-world
  ports:
  - port: 80
    targetPort: 8080
EOF

# Wait for pods to be ready
kubectl wait --for=condition=ready pod -l app=hello-world -n test-app --timeout=60s

# Get service IP
kubectl get svc hello-world -n test-app

# Test application
curl http://<EXTERNAL-IP>
```

**Expected output:**

```
Hello, world!
Version: 1.0.0
Hostname: hello-world-<pod-id>
```

## Step 9: Test GitOps Workflow (Optional)

If you want to test GitOps workflow locally:

```bash
# Initialize Git repository
cd ~/dev-cluster-gitops
git init
git add .
git commit -m "Initial dev-cluster configuration"

# Install FluxCD
flux install

# Create GitRepository source (local path)
flux create source git dev-cluster \
  --url=file://$(pwd) \
  --branch=main \
  --interval=1m

# Create Kustomization
flux create kustomization dev-cluster \
  --source=dev-cluster \
  --path=./applications/overlays/dev-cluster \
  --prune=true \
  --interval=1m

# Watch reconciliation
flux get kustomizations --watch
```

**Why test GitOps locally:**

* Validate manifests before pushing to production
* Test FluxCD configuration
* Debug reconciliation issues
* Learn GitOps workflow

## Check Your Work

Verify everything is working:

* [ ] All 3 nodes are Ready
* [ ] All platform services are Running
* [ ] Can access Headlamp dashboard
* [ ] Test application deployed successfully
* [ ] Can curl test application
* [ ] GitOps workflow tested (optional)

## Troubleshooting

### Docker Not Running

**Error:**

```
Error: Cannot connect to Docker daemon
```

**Solution:**

```bash
# Start Docker Desktop
# On macOS: Open Docker Desktop application
# On Linux: sudo systemctl start docker

# Verify Docker is running
docker ps
```

### Insufficient Docker Resources

**Error:**

```
Error: Insufficient Docker resources
Required: 8 GB RAM
Available: 4 GB RAM
```

**Solution:**

1. Open Docker Desktop
2. Go to Settings → Resources
3. Increase Memory to 8 GB (or more)
4. Click "Apply & Restart"
5. Retry cluster creation

### Kind Cluster Creation Fails

**Error:**

```
Error: failed to create cluster: failed to pull image
```

**Solution:**

```bash
# Check Docker can pull images
docker pull kindest/node:v1.33.5

# If pull fails, check internet connection
# If behind proxy, configure Docker proxy settings

# Retry cluster creation
opencenter cluster deploy dev-cluster
```

### Pods Not Starting

**Error:**

```
Pods stuck in Pending state
```

**Solution:**

```bash
# Check pod events
kubectl describe pod <pod-name> -n <namespace>

# Common causes:
# 1. Insufficient resources (increase Docker memory)
# 2. Image pull errors (check internet connection)
# 3. Node not ready (wait for Calico CNI)

# Check node status
kubectl get nodes

# Check Calico pods
kubectl get pods -n calico-system
```

### MetalLB Not Assigning IPs

**Error:**

```
Service stuck in Pending state (no EXTERNAL-IP)
```

**Solution:**

```bash
# Check MetalLB pods
kubectl get pods -n metallb-system

# Check MetalLB configuration
kubectl get ipaddresspool -n metallb-system

# If no IP pool, create one
cat <<EOF | kubectl apply -f -
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: default
  namespace: metallb-system
spec:
  addresses:
  - 172.18.255.200-172.18.255.250
EOF
```

## Development Workflow

### Iterative Development

```bash
# 1. Make changes to application code
vim my-app/main.go

# 2. Build container image
docker build -t my-app:dev .

# 3. Load image into Kind cluster
kind load docker-image my-app:dev --name dev-cluster

# 4. Deploy to cluster
kubectl apply -f my-app/deployment.yaml

# 5. Test changes
kubectl port-forward -n my-app svc/my-app 8080:80
curl http://localhost:8080

# 6. Iterate (repeat steps 1-5)
```

### Cluster Lifecycle

```bash
# Stop cluster (preserves state)
docker stop $(docker ps -q --filter name=dev-cluster)

# Start cluster
docker start $(docker ps -aq --filter name=dev-cluster)

# Delete cluster (clean slate)
kind delete cluster --name dev-cluster

# Recreate cluster
opencenter cluster deploy dev-cluster
```

### Resource Management

```bash
# Check Docker resource usage
docker stats

# Check Kubernetes resource usage
kubectl top nodes
kubectl top pods -A

# Clean up unused images
docker image prune -a

# Clean up unused volumes
docker volume prune
```

## Next Steps

Now that you have a local development cluster, explore these topics:

**Development:**

* [Customize Services](operations/customize-services.md) - Configure platform services
* [Integrate CI/CD](operations/integrate-ci-cd.md) - Automate testing

**Testing:**

* [Validate Configuration](operations/validate-configuration.md) - Test configurations
* [Troubleshoot Deployment](operations/troubleshoot-deployment.md) - Debug issues

**Production:**

* [OpenStack First Cluster](getting-started/openstack-first-cluster.md) - Deploy to production
* [VMware Deployment](getting-started/vmware-deployment.md) - Deploy to VMware

**Understanding:**

* [Provider Comparison](concepts/provider-comparison.md) - Compare providers
* [GitOps Workflow](concepts/gitops-workflow.md) - How GitOps works

## What You Learned

In this tutorial, you:

* Initialized a Kind cluster configuration
* Customized cluster for local development
* Deployed a local Kubernetes cluster in Docker
* Verified cluster health and service deployment
* Deployed and tested a sample application
* Learned development workflow with Kind

You now have a local development environment for testing Kubernetes applications without cloud costs!

---

## Evidence

This tutorial is based on:

* Kind provider: `docs/providers/README.md:12`
* Kind defaults: `internal/config/defaults.go:27-31`
* Workflow validation: `tests/features/workflow.feature:1-73`
* Bootstrap process: `cmd/cluster_bootstrap.go`
* Service configuration: `internal/config/defaults.go:293-388`