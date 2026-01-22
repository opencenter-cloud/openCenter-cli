---
title: Managing Multiple Clusters
doc_type: tutorial
weight: 30
---

# Managing Multiple Clusters


## Table of Contents

- [What You'll Learn](#what-youll-learn)
- [Prerequisites](#prerequisites)
- [Understanding Organization Structure](#understanding-organization-structure)
- [Step 1: Create Your First Organization](#step-1-create-your-first-organization)
- [Step 2: Add More Clusters to the Organization](#step-2-add-more-clusters-to-the-organization)
- [Step 3: Create Staging Organization](#step-3-create-staging-organization)
- [Step 4: Create Production Organization](#step-4-create-production-organization)
- [Step 5: List All Clusters](#step-5-list-all-clusters)
- [Step 6: Switch Between Clusters](#step-6-switch-between-clusters)
- [Step 7: Activate Cluster Environment](#step-7-activate-cluster-environment)
- [Step 8: View Current Cluster](#step-8-view-current-cluster)
- [Step 9: Share Configuration Across Clusters](#step-9-share-configuration-across-clusters)
- [Step 10: Manage Secrets Across Clusters](#step-10-manage-secrets-across-clusters)
- [Working with Multiple Environments](#working-with-multiple-environments)
- [Advanced Multi-Cluster Patterns](#advanced-multi-cluster-patterns)
- [Shell Integration](#shell-integration)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)
- [Next Steps](#next-steps)
- [Additional Resources](#additional-resources)
This tutorial teaches you how to manage multiple Kubernetes clusters with openCenter, including organizing clusters by environment, switching between clusters, and maintaining consistent configurations across your infrastructure.

## What You'll Learn

By the end of this tutorial, you'll be able to:

- Organize clusters using the organization structure
- Create and manage clusters across multiple environments
- Switch between clusters efficiently
- Share configurations and secrets across clusters
- Maintain consistency across development, staging, and production

## Prerequisites

Before starting, ensure you have:

- openCenter CLI installed and configured
- Access to cloud infrastructure (OpenStack, AWS, or local Kind)
- Basic understanding of Kubernetes concepts
- Familiarity with GitOps workflows

## Understanding Organization Structure

openCenter uses an organization-based directory structure to manage multiple clusters:

```
~/.config/openCenter/clusters/
├── production/                    # Production organization
│   ├── .prod-east-config.yaml    # Cluster config at org level
│   ├── .prod-west-config.yaml
│   ├── infrastructure/
│   │   └── clusters/
│   │       ├── prod-east/        # Cluster-specific files
│   │       └── prod-west/
│   ├── applications/
│   │   └── overlays/
│   │       ├── prod-east/        # App configs per cluster
│   │       └── prod-west/
│   └── secrets/
│       ├── age/
│       │   └── keys/             # Shared encryption keys
│       └── ssh/                  # Shared SSH keys
├── staging/                       # Staging organization
│   ├── .staging-config.yaml
│   ├── infrastructure/
│   ├── applications/
│   └── secrets/
└── development/                   # Development organization
    ├── .dev-local-config.yaml
    ├── infrastructure/
    ├── applications/
    └── secrets/
```

This structure provides:

- **Isolation**: Each organization has separate configurations and secrets
- **Sharing**: Clusters within an organization share encryption keys and SSH keys
- **Organization**: Clear separation between environments
- **Scalability**: Easy to add new clusters to existing organizations

## Step 1: Create Your First Organization

Start by creating a development organization:

```bash
./bin/openCenter cluster init dev-local \
  --org development \
  --opencenter.meta.env=dev \
  --opencenter.meta.region=local
```

This creates:

- Organization: `development`
- Cluster: `dev-local`
- Environment: `dev`
- Region: `local`

The configuration is stored at:

```
~/.config/openCenter/clusters/development/.dev-local-config.yaml
```

## Step 2: Add More Clusters to the Organization

Add additional development clusters:

```bash
# Development cluster in cloud region
./bin/openCenter cluster init dev-cloud \
  --org development \
  --opencenter.meta.env=dev \
  --opencenter.meta.region=us-east-1 \
  --opencenter.infrastructure.provider=openstack

# Development cluster for testing
./bin/openCenter cluster init dev-test \
  --org development \
  --opencenter.meta.env=dev \
  --opencenter.meta.region=local \
  --opencenter.infrastructure.provider=kind
```

All three clusters share:

- SOPS encryption keys (for secret management)
- SSH keys (for infrastructure access)
- Organization-level configurations

## Step 3: Create Staging Organization

Set up a staging environment:

```bash
./bin/openCenter cluster init staging \
  --org staging \
  --opencenter.meta.env=staging \
  --opencenter.meta.region=us-east-1 \
  --opencenter.infrastructure.provider=openstack \
  --opencenter.cluster.kubernetes.master_count=3 \
  --opencenter.cluster.kubernetes.worker_count=3
```

Staging typically mirrors production configuration but with reduced scale.

## Step 4: Create Production Organization

Set up production clusters:

```bash
# Production cluster in US East
./bin/openCenter cluster init prod-east \
  --org production \
  --opencenter.meta.env=prod \
  --opencenter.meta.region=us-east-1 \
  --opencenter.infrastructure.provider=openstack \
  --opencenter.cluster.kubernetes.master_count=3 \
  --opencenter.cluster.kubernetes.worker_count=5

# Production cluster in US West (for HA)
./bin/openCenter cluster init prod-west \
  --org production \
  --opencenter.meta.env=prod \
  --opencenter.meta.region=us-west-2 \
  --opencenter.infrastructure.provider=openstack \
  --opencenter.cluster.kubernetes.master_count=3 \
  --opencenter.cluster.kubernetes.worker_count=5
```

Production clusters share encryption keys within the `production` organization, enabling:

- Consistent secret management
- Simplified disaster recovery
- Cross-region replication

## Step 5: List All Clusters

View all configured clusters:

```bash
./bin/openCenter cluster list
```

Output shows clusters in `organization/cluster` format:

```
development/dev-local
development/dev-cloud
development/dev-test
staging/staging
production/prod-east
production/prod-west
```

The asterisk (`*`) indicates the currently active cluster.

## Step 6: Switch Between Clusters

Select a cluster to work with:

```bash
./bin/openCenter cluster select development/dev-local
```

This displays comprehensive cluster information:

```
Cluster Information:
  Name:         dev-local
  Environment:  dev
  Region:       local
  Status:       deployed
  Organization: development

GitOps Repository:
  GitOps Directory:      ~/.config/openCenter/clusters/development
  Applications Directory: ~/.config/openCenter/clusters/development/applications/overlays/dev-local
  Infrastructure Directory: ~/.config/openCenter/clusters/development/infrastructure/clusters/dev-local
  Secrets Directory:     ~/.config/openCenter/clusters/development/secrets

Cluster Paths:
  Cluster Directory:     ~/.config/openCenter/clusters/development/infrastructure/clusters/dev-local
  SOPS Key Path:         ~/.config/openCenter/clusters/development/secrets/age/keys/dev-local-key.txt
  SOPS Config Path:      ~/.config/openCenter/clusters/development/.sops.yaml
```

### Interactive Selection

If you don't specify a cluster name, openCenter shows an interactive menu:

```bash
./bin/openCenter cluster select
```

Use arrow keys to navigate and Enter to select.

## Step 7: Activate Cluster Environment

Set up your shell environment for the selected cluster:

```bash
eval $(./bin/openCenter cluster select development/dev-local --activate --export-only)
```

This configures:

- `KUBECONFIG`: Points to cluster kubeconfig
- `OPENCENTER_ACTIVE_CLUSTER`: Current cluster identifier
- `BIN`: Cluster-specific binaries directory
- `PATH`: Includes cluster binaries
- Cloud credentials: AWS or OpenStack credentials from cluster config

Verify the environment:

```bash
echo $OPENCENTER_ACTIVE_CLUSTER
kubectl cluster-info
```

## Step 8: View Current Cluster

Check which cluster is currently active:

```bash
./bin/openCenter cluster current
```

For use in shell prompts:

```bash
# Full cluster name
./bin/openCenter cluster current --quiet

# Short name (just cluster, no organization)
./bin/openCenter cluster active-fast --short

# Formatted for prompt
./bin/openCenter cluster active-fast --prompt
```

## Step 9: Share Configuration Across Clusters

Create a base configuration template for reuse:

```bash
# Export staging configuration as template
./bin/openCenter cluster render staging > staging-template.yaml

# Create new cluster from template
./bin/openCenter cluster init staging-eu \
  --org staging \
  --config staging-template.yaml \
  --opencenter.meta.region=eu-west-1
```

This approach ensures consistency across clusters in the same environment.

## Step 10: Manage Secrets Across Clusters

Clusters within an organization share SOPS encryption keys, making it easy to share secrets:

```bash
# Encrypt a secret in development organization
cd ~/.config/openCenter/clusters/development
echo "database_password: supersecret" > db-secret.yaml
sops -e -i db-secret.yaml

# The encrypted secret can be used by any cluster in the organization
cp db-secret.yaml applications/overlays/dev-local/
cp db-secret.yaml applications/overlays/dev-cloud/
cp db-secret.yaml applications/overlays/dev-test/
```

All clusters in the `development` organization can decrypt this secret using the shared Age key.

## Working with Multiple Environments

### Development Workflow

Use development clusters for rapid iteration:

```bash
# Select development cluster
eval $(./bin/openCenter cluster select development/dev-local --activate --export-only)

# Make changes to applications
cd ~/.config/openCenter/clusters/development/applications/overlays/dev-local
vim my-app.yaml

# Test changes
kubectl apply -f my-app.yaml

# Commit when ready
git add my-app.yaml
git commit -m "Update my-app configuration"
```

### Staging Workflow

Promote changes to staging for validation:

```bash
# Select staging cluster
eval $(./bin/openCenter cluster select staging/staging --activate --export-only)

# Copy validated changes from development
cp ~/.config/openCenter/clusters/development/applications/overlays/dev-local/my-app.yaml \
   ~/.config/openCenter/clusters/staging/applications/overlays/staging/

# Apply to staging
kubectl apply -f ~/.config/openCenter/clusters/staging/applications/overlays/staging/my-app.yaml

# Run integration tests
./run-integration-tests.sh
```

### Production Workflow

Deploy to production after staging validation:

```bash
# Select production cluster
eval $(./bin/openCenter cluster select production/prod-east --activate --export-only)

# Copy validated changes from staging
cp ~/.config/openCenter/clusters/staging/applications/overlays/staging/my-app.yaml \
   ~/.config/openCenter/clusters/production/applications/overlays/prod-east/

# Apply to production (with caution)
kubectl apply -f ~/.config/openCenter/clusters/production/applications/overlays/prod-east/my-app.yaml

# Monitor rollout
kubectl rollout status deployment/my-app
```

## Advanced Multi-Cluster Patterns

### Cross-Region Replication

Maintain identical configurations across regions:

```bash
# Create base configuration
./bin/openCenter cluster render production/prod-east > prod-base.yaml

# Deploy to west region with region-specific overrides
./bin/openCenter cluster init prod-west \
  --org production \
  --config prod-base.yaml \
  --opencenter.meta.region=us-west-2 \
  --opencenter.infrastructure.cloud.openstack.region=us-west-2
```

### Environment Promotion

Automate promotion between environments:

```bash
#!/bin/bash
# promote-to-staging.sh

SOURCE_ORG="development"
SOURCE_CLUSTER="dev-cloud"
TARGET_ORG="staging"
TARGET_CLUSTER="staging"

# Copy application manifests
cp -r ~/.config/openCenter/clusters/$SOURCE_ORG/applications/overlays/$SOURCE_CLUSTER/* \
      ~/.config/openCenter/clusters/$TARGET_ORG/applications/overlays/$TARGET_CLUSTER/

# Commit changes
cd ~/.config/openCenter/clusters/$TARGET_ORG
git add applications/
git commit -m "Promote from $SOURCE_ORG/$SOURCE_CLUSTER to $TARGET_ORG/$TARGET_CLUSTER"
git push

echo "Promoted to $TARGET_ORG/$TARGET_CLUSTER"
```

### Cluster Comparison

Compare configurations across clusters:

```bash
# Compare development and staging
diff <(./bin/openCenter cluster render development/dev-cloud) \
     <(./bin/openCenter cluster render staging/staging)

# Compare production regions
diff <(./bin/openCenter cluster render production/prod-east) \
     <(./bin/openCenter cluster render production/prod-west)
```

## Shell Integration

Add cluster information to your shell prompt:

### Bash

Add to `~/.bashrc`:

```bash
# openCenter cluster prompt
opencenter_prompt() {
  local cluster=$(./bin/openCenter cluster active-fast --prompt 2>/dev/null)
  if [ -n "$cluster" ]; then
    echo "$cluster "
  fi
}

PS1='$(opencenter_prompt)\u@\h:\w\$ '
```

### Zsh

Add to `~/.zshrc`:

```zsh
# openCenter cluster prompt
opencenter_prompt() {
  local cluster=$(./bin/openCenter cluster active-fast --prompt 2>/dev/null)
  if [[ -n "$cluster" ]]; then
    echo "$cluster "
  fi
}

PROMPT='$(opencenter_prompt)%n@%m:%~%# '
```

### Fish

Add to `~/.config/fish/config.fish`:

```fish
# openCenter cluster prompt
function opencenter_prompt
  set -l cluster (./bin/openCenter cluster active-fast --prompt 2>/dev/null)
  if test -n "$cluster"
    echo "$cluster "
  end
end

function fish_prompt
  echo (opencenter_prompt)(whoami)@(hostname):(pwd)'$ '
end
```

## Troubleshooting

### Cannot Find Cluster

**Problem**: `cluster configuration directory 'my-cluster' not found`

**Solution**: Use the full `organization/cluster` format:

```bash
# Wrong
./bin/openCenter cluster select my-cluster

# Correct
./bin/openCenter cluster select production/my-cluster
```

### Wrong Cluster Active

**Problem**: Commands apply to wrong cluster.

**Solution**: Check and set active cluster:

```bash
# Check current cluster
./bin/openCenter cluster current

# Switch to correct cluster
eval $(./bin/openCenter cluster select production/prod-east --activate --export-only)

# Verify
echo $OPENCENTER_ACTIVE_CLUSTER
kubectl config current-context
```

### Shared Secrets Not Decrypting

**Problem**: Cannot decrypt secrets in new cluster.

**Solution**: Verify SOPS key is shared:

```bash
# Check key location
ls -la ~/.config/openCenter/clusters/production/secrets/age/keys/

# Verify SOPS configuration
cat ~/.config/openCenter/clusters/production/.sops.yaml

# Test decryption
sops -d ~/.config/openCenter/clusters/production/applications/overlays/prod-east/secret.yaml
```

## Best Practices

### Organization Naming

- Use descriptive organization names: `production`, `staging`, `development`
- Avoid abbreviations that might be ambiguous
- Keep names lowercase for consistency

### Cluster Naming

- Include environment and region: `prod-east`, `staging-eu`, `dev-local`
- Use consistent naming patterns across organizations
- Avoid special characters except hyphens

### Environment Separation

- Never share encryption keys between production and non-production
- Use separate organizations for different security boundaries
- Implement strict access controls on production organizations

### Configuration Management

- Store base configurations in version control
- Use templates for consistency across similar clusters
- Document environment-specific overrides

## Next Steps

Now that you can manage multiple clusters:

- **Implement CI/CD**: Automate deployments across environments
- **Set Up Monitoring**: Track cluster health across all environments
- **Configure Backups**: Implement disaster recovery for production clusters
- **Optimize Costs**: Right-size clusters based on environment needs

For more advanced topics, see:

- [GitOps Workflow Guide](../how-to/gitops-workflow.md)
- [Disaster Recovery](../how-to/disaster-recovery.md)
- [Security Best Practices](../how-to/security.md)
- [Configuration Reference](../reference/config.md)

## Additional Resources

- [Organization Structure Documentation](../reference/directory-structure.md)
- [Cluster Selection Command Reference](../reference/cli/cluster-select.md)
- [SOPS Encryption Guide](../how-to/secrets-management.md)
- [Multi-Region Deployment Patterns](../how-to/multi-region.md)
