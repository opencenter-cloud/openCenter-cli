---
id: deploy-cluster
title: "Deploy a New Cluster"
sidebar_label: Deploy a New Cluster
description: Step-by-step guide to initialize, configure, validate, and deploy a new Kubernetes cluster with openCenter.
doc_type: how-to
audience: "operators, platform engineers"
tags: [deploy, cluster, openstack, gitops, bootstrap]
---
# Deploy a New Cluster

**Purpose:** For operators, shows how to deploy a new Kubernetes cluster from scratch using openCenter, covering initialization through bootstrap.

## Prerequisites

* openCenter CLI built and available (`mise run build`)
* A provisioned Git repository for GitOps (GitHub, Gitea, or similar)
* A GitHub personal access token (or equivalent) with write access to the repository
* Infrastructure credentials for your target provider (OpenStack in this example)
* The `opencenter-rmpk` external plugin installed (for provider-specific sync)

## 1. Initialize the cluster configuration

Create a new configuration file with provider-appropriate defaults. The `--org` flag sets the organization namespace and `--type` selects the infrastructure provider.

```bash
opencenter cluster init services-2026-02-0d --org opencenter-cloud --type openstack
```

This creates the configuration at:

```
~/.config/opencenter/clusters/opencenter-cloud/.services-2026-02-0d-config.yaml
```

The file includes sensible defaults for Kubernetes version, node counts, CNI, and platform services. Edit it to match your environment before proceeding.

**Evidence:** `cmd/cluster_init.go`, `internal/config/defaults/`

## 2. Set the active cluster

Select the cluster so subsequent commands operate on it without requiring the name each time:

```bash
opencenter cluster use opencenter-cloud/services-2026-02-0d
```

Verify the selection:

```bash
opencenter cluster use
```

The output shows cluster metadata, GitOps paths, and environment setup commands.

**Evidence:** `cmd/cluster_use.go`

## 3. Sync provider-specific configuration

For OpenStack clusters, use the `rmpk` plugin to synchronize provider-specific values (images, flavors, networks) into the cluster configuration:

```bash
opencenter rmpk cluster sync openstack \
  --cluster-config opencenter-cloud/.services-2026-02-0d-config.yaml \
  --cloud flex-dfw-dev \
  --yes
```

The `--cloud` flag references an OpenStack cloud entry from your `clouds.yaml`. The `--yes` flag skips confirmation prompts.

> `rmpk` is an [external CLI plugin](concepts/plugin-external-cli.md) discovered at runtime. It must be installed separately.

## 4. Set GitOps and secrets configuration

Configure the GitOps repository URL, authentication token, and any required secrets using dot-notation paths:

```bash
opencenter cluster set opencenter.gitops.repository.url=https://github.com/opencenter-cloud/token-test-repo.git
opencenter cluster set opencenter.gitops.auth.token.token=$(cat ~/.config/opencenter/github_token.env)
opencenter cluster set secrets.keycloak.admin_password=$(openssl rand -base64 16)
```

Each `cluster set` call updates the active cluster’s configuration file in place.

**What these values control:**

| Path | Purpose |
| --- | --- |
| `opencenter.gitops.repository.url` | Remote Git repository where generated manifests are pushed |
| `opencenter.gitops.auth.token.token` | Authentication token for FluxCD to pull from the repository |
| `secrets.keycloak.admin_password` | Admin password for the Keycloak identity service |

**Evidence:** `cmd/cluster_set.go`

## 5. Validate the configuration

Run validation to catch errors before committing to a deployment:

```bash
opencenter cluster validate
```

Validation checks:

* JSON schema compliance (structure, types, required fields)
* Cross-field dependencies (e.g., VRRP IP required when Octavia is disabled)
* GitOps repository URL format and auth configuration
* Network configuration (CIDR ranges, subnet overlaps)
* SOPS encryption key availability

A passing result looks like:

```
✓ Validation successful

Cluster: opencenter-cloud/services-2026-02-0d
Organization: opencenter-cloud
Provider: openstack
Validation mode: offline

Summary: passed
```

For provider connectivity checks (image IDs, flavor availability, quota limits), use online mode:

```bash
opencenter cluster validate --validation online
```

Fix any reported errors before continuing. See [Validate Configuration](operations/validate-configuration.md) for error resolution guidance.

**Evidence:** `cmd/cluster_validate.go`, `internal/config/validator.go`

## 6. Generate the GitOps repository

Render templates and create the GitOps repository structure:

```bash
opencenter cluster generate
```

This produces the full directory layout under the configured `local_dir`:

```
<git_dir>/
├── applications/
│   └── overlays/services-2026-02-0d/
│       ├── flux-system/           # FluxCD bootstrap manifests
│       ├── services/              # Platform service Kustomizations and overrides
│       └── managed-services/      # Application manifests
└── infrastructure/
    └── clusters/services-2026-02-0d/
        ├── main.tf                # OpenTofu/Terraform configuration
        ├── inventory/             # Kubespray Ansible inventory
        └── credentials/           # Provider credentials (SOPS-encrypted)
```

Use `--force` to overwrite an existing repository. Use `--render-only` to render templates without running the full repository setup.

**Evidence:** `cmd/cluster_generate.go`, `internal/gitops/`

## 7. Preview the deploy plan (optional)

Inspect what the deploy will do without making changes:

```bash
opencenter cluster deploy --dry-run
```

The dry run prints the ordered list of deploy steps, their dependencies, and the commands each step executes. Review this to confirm the plan matches your expectations.

## 8. Deploy the cluster

Run the full deployment:

```bash
opencenter cluster deploy
```

The deploy process is **resumable**. If a step fails, fix the underlying issue and re-run `opencenter cluster deploy` -- it picks up from the last saved state.

### Deploy phases

| Phase | Duration | What happens |
| --- | --- | --- |
| Infrastructure | 5--10 min | Provisions VMs, networks, security groups, load balancers via OpenTofu |
| Kubernetes | 10--15 min | Installs container runtime, deploys control plane and workers via Kubespray |
| GitOps | 2--5 min | Bootstraps FluxCD, creates GitRepository and Kustomization resources |
| Services | 10--20 min | FluxCD reconciles platform services (cert-manager, Keycloak, Prometheus, etc.) |

### Useful flags

| Flag | Purpose |
| --- | --- |
| `--restart` | Ignore saved state and rerun all steps from the beginning |
| `--step <id>` | Run a single deploy step by ID |
| `--from-step <id>` | Resume from a specific step instead of the last saved state |
| `--debug` | Print step details before each step runs |
| `--confirm-commit` | Prompt before auto-committing uncommitted GitOps changes |
| `--break-lock` | Force-remove an existing operation lock |

**Evidence:** `cmd/cluster_deploy.go`, `internal/cluster/bootstrap.go`

## Verification

After deploy completes, confirm the cluster is healthy:

```bash
# Check node status
export KUBECONFIG=<git_dir>/infrastructure/clusters/services-2026-02-0d/kubeconfig.yaml
kubectl get nodes

# Check FluxCD reconciliation
kubectl get kustomizations -n flux-system

# Check platform services
kubectl get helmreleases -A
```

All nodes should report `Ready`, all Kustomizations should show `Ready: True`, and HelmReleases should show `deployed`.

## Troubleshooting

**Validation fails:** Read the error messages -- they include the field path and expected value. Common issues: missing credentials, overlapping CIDRs, missing VRRP IP. See [Validate Configuration](operations/validate-configuration.md).

**Deploy step fails:** Check the bootstrap log printed at failure. Fix the issue and re-run `opencenter cluster deploy` to resume. Use `--debug` for step-level detail.

**GitOps dirty tree error:** The deploy requires a clean Git working tree. Commit or stash changes in the GitOps directory, or use `--confirm-commit` to auto-commit before deploy.

**Remote origin mismatch:** If the GitOps directory’s `origin` remote doesn’t match `opencenter.gitops.repository.url`, update it:

```bash
git -C <git_dir> remote set-url origin <correct-url>
```

See [Troubleshoot Deployment](operations/troubleshoot-deployment.md) for additional scenarios.