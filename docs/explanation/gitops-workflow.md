# GitOps Workflow in openCenter


## Table of Contents

- [What GitOps Means Here](#what-gitops-means-here)
- [Why GitOps](#why-gitops)
- [Repository Structure](#repository-structure)
- [How openCenter Generates Repositories](#how-opencenter-generates-repositories)
- [FluxCD Integration](#fluxcd-integration)
- [ArgoCD Integration](#argocd-integration)
- [Configuration Change Workflow](#configuration-change-workflow)
- [Trade-offs](#trade-offs)
- [Common Misconceptions](#common-misconceptions)
**doc_type: explanation**

This document explains what GitOps means in openCenter's context, why it's the default approach, and how configuration changes flow through Git to your cluster.

## What GitOps Means Here

GitOps treats Git as the single source of truth for infrastructure and application configuration. Every change to your cluster—whether it's a new service, a configuration update, or an infrastructure modification—flows through a Git commit. A continuous delivery tool (FluxCD or ArgoCD) watches the repository and automatically applies changes to the cluster.

openCenter generates a complete GitOps repository from your cluster configuration. The repository contains everything needed to provision infrastructure, deploy Kubernetes, and install applications. You commit this repository to Git, point FluxCD or ArgoCD at it, and the cluster converges to match what's in the repository.

## Why GitOps

### Version Control for Infrastructure

Every change is a commit with an author, timestamp, and message. You can see who changed what and when. If something breaks, you revert the commit. The Git history becomes an audit trail of every cluster modification.

### Declarative Desired State

You describe what you want, not how to get there. The GitOps tool compares the repository state to the cluster state and makes the necessary changes. If someone manually modifies the cluster, the tool detects the drift and corrects it.

### Automated Reconciliation

FluxCD or ArgoCD runs a reconciliation loop. Every few minutes, it checks if the cluster matches the repository. If it doesn't, it applies the difference. This means your cluster self-heals from manual changes and automatically picks up new commits.

### Separation of Concerns

Developers commit application changes. Platform engineers commit infrastructure changes. The GitOps tool applies both without requiring cluster credentials for every team member. Access control happens at the Git level, not the cluster level.

## Repository Structure

openCenter generates a repository with two top-level directories:

```
gitops-repo/
├── applications/
│   ├── base/
│   └── overlays/
│       └── <cluster-name>/
│           ├── services/
│           │   ├── fluxcd/
│           │   └── sources/
│           └── managed-services/
│               ├── fluxcd/
│               └── sources/
├── infrastructure/
│   └── clusters/
│       └── <cluster-name>/
│           ├── main.tf
│           ├── variables.tf
│           └── inventory/
└── .flux-system/
    └── kustomization.yaml
```

### Applications Directory

The `applications/` directory contains Kubernetes manifests for services running on the cluster. It uses Kustomize to organize configurations:

- **base/**: Shared base configurations that apply to all clusters
- **overlays/<cluster-name>/**: Cluster-specific customizations

Each cluster overlay contains:

- **services/**: Core cluster services (cert-manager, ingress controllers, monitoring)
- **managed-services/**: Optional add-ons (backup solutions, security tools)
- **fluxcd/**: FluxCD Kustomization resources that tell FluxCD what to deploy
- **sources/**: GitRepository and HelmRepository sources for applications

The `fluxcd/` subdirectories contain Kustomization manifests that reference the service definitions. FluxCD reads these to determine what to install and how to configure it.

### Infrastructure Directory

The `infrastructure/clusters/<cluster-name>/` directory contains provider-specific provisioning code:

- **main.tf**: Terraform/OpenTofu configuration for infrastructure (networks, VMs, load balancers)
- **variables.tf**: Input variables for the Terraform configuration
- **inventory/**: Ansible inventory files for Kubespray-based deployments
- **Makefile**: Task automation for provisioning and teardown

This directory is not managed by FluxCD. You run `mise run tofu-apply` or `mise run ansible-deploy` to provision infrastructure. Once the cluster exists, FluxCD takes over for application deployment.

### Flux System Directory

The `.flux-system/` directory contains FluxCD's own configuration. This includes the Kustomization that bootstraps FluxCD itself. When you run `flux bootstrap`, it creates this directory and commits it to the repository.

## How openCenter Generates Repositories

openCenter embeds templates in the binary using Go's `embed` package. When you run `openCenter cluster setup`, it:

1. Creates a workspace directory for generation
2. Copies base files from `gitops-base-dir/` (README, .gitignore, directory structure)
3. Renders cluster-specific applications from `templates/cluster-apps-base/`
4. Renders infrastructure configuration from `templates/infrastructure-cluster-template/`
5. Processes `.tpl` and `.tmpl` files through Go's template engine with your cluster config
6. Writes the final repository to the configured `git_dir` path

Templates have access to your entire cluster configuration. A template can reference `{{ .OpenCenter.Meta.Name }}` to get the cluster name or `{{ .OpenCenter.Infrastructure.Provider }}` to conditionally generate provider-specific resources.

The generation process runs in stages with checkpointing. If a stage fails, openCenter rolls back to the previous checkpoint. This prevents partial repository generation that could leave you with an inconsistent state.

### Template Rendering

Files ending in `.tpl` are always rendered and have the extension stripped. Files ending in `.tmpl` are rendered only if you pass `--render` to the setup command. This lets you generate stub files that you can manually customize before committing.

The template engine uses Sprig functions for string manipulation, date formatting, and other utilities. You can use `{{ .ClusterName | kebabcase }}` or `{{ now | date "2006-01-02" }}` in templates.

### Service Filtering

openCenter skips disabled services during generation. If you set `services.cert-manager.enabled: false` in your configuration, the generator won't create cert-manager manifests or FluxCD Kustomizations. This keeps the repository clean and avoids deploying unwanted services.

## FluxCD Integration

FluxCD is a continuous delivery tool that watches Git repositories and applies changes to Kubernetes clusters. It runs inside the cluster as a set of controllers.

### How It Works

1. You commit a change to the GitOps repository
2. FluxCD's source-controller polls the repository every few minutes
3. When it detects a new commit, it fetches the updated manifests
4. The kustomize-controller builds the final manifests using Kustomize
5. The kustomize-controller applies the manifests to the cluster
6. If the apply fails, FluxCD retries with exponential backoff

FluxCD stores its state in Kubernetes custom resources. You can run `flux get kustomizations` to see what's deployed and whether it's up to date.

### Reconciliation Loop

FluxCD doesn't just apply changes once. It continuously reconciles the cluster state with the repository state. If someone runs `kubectl delete` on a resource managed by FluxCD, FluxCD recreates it on the next reconciliation cycle (typically 5-10 minutes).

This reconciliation loop is what makes GitOps self-healing. Manual changes don't stick. The cluster always converges back to what's in Git.

### Drift Detection

FluxCD detects drift by comparing the manifests in Git to the resources in the cluster. If they differ, FluxCD reports the drift and can optionally correct it. You can configure drift detection to be strict (correct any difference) or lenient (ignore certain fields like status or metadata).

## ArgoCD Integration

ArgoCD is an alternative to FluxCD with a similar reconciliation model but a different architecture. It provides a web UI for visualizing application state and manually triggering syncs.

openCenter generates FluxCD-compatible manifests by default, but the repository structure works with ArgoCD as well. You need to create ArgoCD Application resources that point to the `applications/overlays/<cluster-name>/` directory.

ArgoCD's reconciliation loop works the same way: poll Git, compare to cluster, apply differences. The main difference is that ArgoCD has a centralized API server and UI, while FluxCD is fully decentralized with CLI-only management.

## Configuration Change Workflow

Here's how a typical configuration change flows through the system:

1. **Edit Configuration**: You modify `.opencenter-config.yaml` to change a service setting
2. **Regenerate Repository**: Run `openCenter cluster render` to update the GitOps repository
3. **Review Changes**: Run `git diff` to see what changed in the generated manifests
4. **Commit Changes**: Run `git commit -am "Update ingress controller replicas"` and `git push`
5. **Automatic Deployment**: FluxCD detects the new commit and applies the changes to the cluster
6. **Verify Deployment**: Run `flux get kustomizations` or check the ArgoCD UI to confirm the sync

You never run `kubectl apply` directly. All changes go through Git. This ensures the repository stays in sync with the cluster and gives you a complete audit trail.

### Rollback

If a change breaks something, you revert the Git commit and push. FluxCD sees the revert and rolls back the cluster to the previous state. This is faster and safer than manually undoing changes with `kubectl`.

### Emergency Changes

Sometimes you need to make an emergency change without going through Git. You can run `kubectl` directly, but FluxCD will undo your change on the next reconciliation. To make the change stick, you need to either:

- Suspend reconciliation with `flux suspend kustomization <name>`
- Commit the change to Git before the next reconciliation cycle
- Configure FluxCD to ignore certain fields or resources

## Trade-offs

### GitOps vs Imperative

**GitOps** gives you version control, audit trails, and automated reconciliation. The cost is complexity: you need a Git repository, a continuous delivery tool, and a way to generate manifests from configuration.

**Imperative** (running `kubectl apply` directly) is simpler for small clusters or development environments. You don't need Git or FluxCD. The cost is no audit trail, no automated rollback, and no drift detection.

openCenter defaults to GitOps because it targets production clusters where audit trails and automated reconciliation matter. For local development with Kind, you can skip the GitOps repository and apply manifests directly.

### Complexity vs Benefits

GitOps adds moving parts: Git hosting, FluxCD controllers, webhook receivers, and SSH keys. Each part can fail. You need to monitor FluxCD's reconciliation status and handle failures.

The benefit is operational safety. You can't accidentally break production by running the wrong `kubectl` command. Every change is reviewed in a pull request. Rollbacks are a `git revert` away.

For small teams or simple clusters, this might be overkill. For regulated environments or large teams, it's essential.

### Repository Size

The GitOps repository grows over time. Every commit adds to the history. Large Helm charts or binary files (like container images) bloat the repository.

openCenter generates text-based manifests, not binaries. The repository stays small. If you add Helm charts, use HelmRepository sources instead of committing the chart tarballs.

## Common Misconceptions

### "GitOps means everything is in Git"

Not quite. Secrets are encrypted with SOPS before committing. Infrastructure state (Terraform state files) lives in a backend, not in Git. Container images are in a registry, not in Git.

Git holds the *configuration* for these things: the SOPS-encrypted secrets, the Terraform code that generates state, the image tags that reference registry images.

### "FluxCD applies changes instantly"

FluxCD polls the repository on an interval (default: 1 minute for sources, 5 minutes for kustomizations). There's a delay between pushing a commit and seeing it applied. You can trigger a manual sync with `flux reconcile` to speed this up.

### "GitOps prevents manual changes"

GitOps *detects* and *corrects* manual changes, but it doesn't prevent them. You can still run `kubectl` commands. FluxCD will undo them on the next reconciliation, but there's a window where the manual change is active.

If you need to prevent manual changes entirely, use Kubernetes admission controllers or RBAC policies.

### "I need to learn Kustomize to use openCenter"

openCenter generates Kustomize manifests for you. You don't need to write Kustomize overlays by hand. If you want to customize beyond what openCenter supports, you can edit the generated files, but the basic workflow doesn't require Kustomize knowledge.

### "GitOps is only for applications"

openCenter uses GitOps for both applications and infrastructure configuration. The infrastructure provisioning (Terraform/Ansible) runs outside the cluster, but the cluster configuration (services, networking, storage) is managed through GitOps.

---

**Related Documentation**

- [Architecture Overview](architecture.md) - How openCenter components fit together
- [Security Model](security-model.md) - How secrets are encrypted and managed
- [Template Engine](template-engine.md) - How templates generate manifests
- [Configuration System](configuration-system.md) - How the YAML configuration works

**See Also**

- [Tutorial: First Cluster](../tutorials/first-cluster.md) - Hands-on walkthrough of the GitOps workflow
- [How-To: Update Cluster Configuration](../how-to/update-cluster-config.md) - Step-by-step guide to making changes
- [Reference: CLI Commands](../reference/cli-commands.md) - Complete command reference

---

**Last Updated**: January 19, 2026  
**openCenter Version**: 1.0.0
