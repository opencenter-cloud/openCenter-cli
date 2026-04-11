---
id: create-openstack-cluster
title: "Create an OpenStack Cluster"
sidebar_label: Create OpenStack Cluster
description: Deploy a production Kubernetes cluster on OpenStack with openCenter and FluxCD GitOps.
doc_type: how-to
audience: "platform engineers, operators"
tags: [openstack, cluster, deployment, gitops, production]
---

# Create an OpenStack Cluster

**Purpose:** For platform engineers and operators, shows how to deploy a production Kubernetes cluster on OpenStack with GitOps delivery via FluxCD.

## Prerequisites

- openCenter CLI installed (`opencenter version`)
- `git`, `kubectl`, `flux` installed
- OpenStack CLI installed (`openstack --version`)
- OpenStack API credentials (auth URL, project ID, username/password or application credentials)
- Sufficient OpenStack quota: 6 instances, 24 vCPUs, 96 GB RAM, 240 GB block storage
- A Git repository for the generated GitOps tree (GitHub, GitLab, Gitea, etc.)

Verify OpenStack access before proceeding:

```bash
openstack server list
openstack quota show
```

## Steps

### 1. Initialize the Cluster Configuration

```bash
opencenter cluster init prod-cluster \
  --org my-company \
  --type openstack
```

This creates a v2 configuration at `~/.config/opencenter/clusters/my-company/prod-cluster/` with OpenStack defaults (SJC3 region, Ubuntu 24.04 image, Kubespray deployment method). It also generates SOPS Age keys and an SSH key pair.

Confirm the generated paths:

```bash
opencenter cluster info prod-cluster
```

### 2. Edit OpenStack Credentials and Provider Settings

```bash
opencenter cluster edit prod-cluster
```

Locate the `opencenter.infrastructure.cloud.openstack` section and replace the placeholder values with your environment:

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        auth_url: "https://identity.api.your-cloud.com/v3"
        region: sjc3
        project_id: "your-project-id"
        project_name: "your-project-name"
        user_domain_name: "Default"
        project_domain_name: "Default"
        image_id: "799dcf97-3656-4361-8187-13ab1b295e33"
        network_id: "your-network-id"
        subnet_id: "your-subnet-id"
        floating_ip_pool: "PUBLICNET"
        router_external_network_id: "your-external-network-id"
        availability_zone: az1
```

Find the correct image ID for your region:

```bash
openstack image list --name Ubuntu
```

Find your network and subnet IDs:

```bash
openstack network list
openstack subnet list
```

### 3. Adjust Compute, Storage, and Networking

In the same configuration file, review and adjust these sections as needed.

Compute (under `opencenter.infrastructure.compute`):

```yaml
compute:
  master_count: 3
  worker_count: 3
  flavor_master: "gp.0.4.8"    # 4 vCPU, 8 GB RAM
  flavor_worker: "gp.0.4.16"   # 4 vCPU, 16 GB RAM
  flavor_bastion: "gp.0.2.2"   # 2 vCPU, 2 GB RAM
```

Storage (under `opencenter.infrastructure.storage`):

```yaml
storage:
  default_storage_class: "csi-cinder-sc-delete"
  worker_volume_size: 40
  worker_volume_type: "HA-Standard"
```

Networking (under `opencenter.infrastructure.networking`):

```yaml
networking:
  subnet_nodes: "10.2.128.0/22"
  dns_nameservers:
    - "8.8.8.8"
    - "8.8.4.4"
  ntp_servers:
    - "time.sjc3.rackspace.com"
    - "time2.sjc3.rackspace.com"
  loadbalancer_provider: ovn
  dns_zone_name: "prod-cluster.sjc3.k8s.opencenter.cloud"
```

Set the GitOps repository URL (under `opencenter.gitops`):

```yaml
gitops:
  git_url: "git@github.com:my-company/prod-cluster-gitops.git"
  git_branch: main
```

List available flavors in your region:

```bash
openstack flavor list
```

### 4. Run Preflight Checks

```bash
opencenter cluster preflight prod-cluster
```

Confirms that `git`, `kubectl`, `talosctl`, and the `openstack` CLI are available, and that the auth URL is configured. Fix any `MISSING` items before continuing.

### 5. Validate the Configuration

```bash
opencenter cluster validate prod-cluster
```

Runs schema validation, required-field checks, and cross-field dependency validation. Add `--check-connectivity` to also verify OpenStack API reachability:

```bash
opencenter cluster validate prod-cluster --check-connectivity
```

Fix any reported errors with `opencenter cluster edit prod-cluster` and re-validate.

### 6. Generate the GitOps Repository

```bash
opencenter cluster setup prod-cluster
```

Produces the full GitOps tree under the configured `git_dir`, including:

- `infrastructure/clusters/prod-cluster/` — OpenTofu configuration and Kubespray inventory
- `applications/overlays/prod-cluster/` — FluxCD Kustomizations and service manifests
- `secrets/` — SOPS-encrypted credential files

Use `--dry-run` to preview without writing files. Use `--force` to overwrite an existing tree.

### 7. Commit and Push the GitOps Repository

```bash
GITOPS_DIR=$(opencenter cluster info prod-cluster 2>/dev/null | grep "git_dir:" | awk '{print $2}')
cd "$GITOPS_DIR"

git init
git add .
git commit -m "feat: initial prod-cluster configuration"
git remote add origin git@github.com:my-company/prod-cluster-gitops.git
git push -u origin main
```

FluxCD reconciles from this repository, so it must be pushed before bootstrap.

### 8. Bootstrap the Cluster

```bash
opencenter cluster bootstrap prod-cluster
```

Bootstrap provisions infrastructure with OpenTofu (network, security groups, instances, volumes, floating IPs), deploys Kubernetes via Kubespray, and installs FluxCD to begin GitOps reconciliation. The process takes 30–45 minutes.

The command is resumable. If a step fails, fix the issue and re-run. Use `--restart` to re-run all steps from scratch, or `--from-step <id>` to resume from a specific step.

Monitor infrastructure creation in a separate terminal:

```bash
watch -n 10 'openstack server list | grep prod-cluster'
```

## Verification

```bash
# Export kubeconfig for the cluster
eval "$(opencenter cluster select prod-cluster --export-only)"

# Nodes
kubectl get nodes
# Expect: 3 control-plane + 3 worker nodes in Ready state

# FluxCD sources and kustomizations
flux get sources git -n flux-system
flux get kustomizations -n flux-system

# Platform services
kubectl get helmreleases -A
```

All nodes should be `Ready`, Flux sources `READY=True`, and HelmReleases reconciled.

Get the load balancer IP for DNS configuration:

```bash
kubectl get svc -n gateway gateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

Point your DNS records (wildcard or per-service) at this IP to access platform services (Keycloak, Grafana, Headlamp, etc.).

## Cleanup

```bash
opencenter cluster destroy prod-cluster --force
```

This tears down infrastructure (instances, volumes, networks) and removes the local configuration. The GitOps repository in Git is not deleted; remove it manually if no longer needed.

## Troubleshooting

### Preflight: `openstack CLI not found`

Install the OpenStack client tools:

```bash
pip install python-openstackclient
```

Then re-run `opencenter cluster preflight prod-cluster`.

### Validation: image ID not found

The default image ID is region-specific. List images in your region and update the config:

```bash
openstack image list --name Ubuntu
opencenter cluster edit prod-cluster
# Update opencenter.infrastructure.cloud.openstack.image_id
```

### Validation: insufficient quota

Reduce `master_count` or `worker_count`, pick smaller flavors, or request a quota increase from your OpenStack administrator.

### Bootstrap: OpenTofu quota error

```bash
# Check current usage
openstack quota show

# If resources from a failed run remain, destroy and retry
opencenter cluster destroy prod-cluster --force
opencenter cluster init prod-cluster --org my-company --type openstack --force
# Re-edit, re-validate, re-setup, re-bootstrap
```

### FluxCD: Kustomization reconciliation failure

```bash
kubectl logs -n flux-system deployment/kustomize-controller --tail=50
flux get kustomizations -n flux-system
flux reconcile kustomization <name> --with-source
```

### Bootstrap interrupted / stale lock

```bash
rm -f ~/.local/state/opencenter/locks/prod-cluster.lock
opencenter cluster bootstrap prod-cluster --restart
```

## Evidence

- OpenStack cloud defaults: `internal/config/v2/defaults.go:applyProviderCloudDefaults()`
- Region-specific defaults (SJC3, DFW3, IAD3, ORD1): `internal/config/defaults/openstack.go`
- OpenStack config struct: `internal/config/v2/infrastructure.go:OpenStackCloudConfig`
- Preflight checks: `internal/cloud/openstack/preflight.go:PreflightOpenStack()`
- Bootstrap command: `cmd/cluster_bootstrap.go`
- Setup command: `cmd/cluster_setup.go`
- Validate command: `cmd/cluster_validate.go`
- Provider state and drift: `internal/cloud/openstack/provider.go`
