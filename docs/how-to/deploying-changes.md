# Deploying Configuration Changes


## Table of Contents

- [When to use this guide](#when-to-use-this-guide)
- [Prerequisites](#prerequisites)
- [Workflow overview](#workflow-overview)
- [Steps](#steps)
- [GitOps workflow integration](#gitops-workflow-integration)
- [Configuration change patterns](#configuration-change-patterns)
- [Troubleshooting](#troubleshooting)
- [Rollback procedure](#rollback-procedure)
- [Best practices](#best-practices)
- [Related commands](#related-commands)
- [Next steps](#next-steps)
**doc_type:** how-to

This guide shows you how to deploy configuration changes to an existing cluster using the openCenter workflow.

## When to use this guide

Use this workflow when you need to:
- Update cluster configuration settings
- Change service configurations
- Modify infrastructure parameters
- Apply new secrets or credentials

## Prerequisites

- Existing cluster initialized with `openCenter cluster init`
- Configuration file at `~/.config/openCenter/clusters/<organization>/.cluster-config.yaml`
- SOPS Age key configured for secrets encryption
- Git repository initialized in the GitOps directory

## Workflow overview

The deployment workflow follows four stages:

1. **Edit** - Modify cluster configuration
2. **Validate** - Check configuration correctness
3. **Setup** - Regenerate GitOps repository
4. **Apply** - Deploy changes to cluster

## Steps

### 1. Edit cluster configuration

Open your cluster configuration file:

```bash
# For organization-based structure
vim ~/.config/openCenter/clusters/<organization>/.<cluster>-config.yaml

# Or use the active cluster
openCenter cluster edit
```

Make your changes. Common modifications:

```yaml
opencenter:
  cluster:
    kubernetes:
      version: 1.33.5
      worker_count: 4
  
  services:
    kube-prometheus-stack:
      enabled: true
```

### 2. Validate configuration

Run validation to catch errors before deployment:

```bash
openCenter cluster validate <cluster-name>
```

The validator checks:
- Schema compliance
- Required field presence
- Cross-field dependencies
- Provider-specific requirements
- Secret availability

Fix any validation errors before proceeding.

### 3. Regenerate GitOps repository

Update the GitOps repository with your changes:

```bash
openCenter cluster setup <cluster-name>
```

This command:
- Renders updated templates
- Generates new manifests
- Updates FluxCD configurations
- Provisions OpenTofu files

Use `--force` to overwrite existing files:

```bash
openCenter cluster setup <cluster-name> --force
```

### 4. Commit and push changes

Review the generated changes:

```bash
cd ~/.config/openCenter/clusters/<organization>/gitops
git status
git diff
```

Commit and push to trigger deployment:

```bash
git add .
git commit -m "Update cluster configuration: <description>"
git push origin main
```

### 5. Monitor deployment

FluxCD automatically reconciles changes. Check status:

```bash
# Watch FluxCD reconciliation
kubectl get kustomizations -n flux-system -w

# Check specific service deployment
kubectl get helmreleases -n <namespace>

# View FluxCD logs
kubectl logs -n flux-system -l app=kustomize-controller -f
```

## GitOps workflow integration

openCenter generates a GitOps repository structure that FluxCD monitors:

```
gitops/
├── applications/
│   ├── base/           # Base service manifests
│   └── overlays/       # Cluster-specific overrides
└── infrastructure/
    └── clusters/       # Cluster infrastructure
```

Changes flow through this path:

1. Configuration change → `cluster setup` → GitOps repository update
2. Git push → FluxCD detects change
3. FluxCD reconciles → Kubernetes applies changes
4. Health checks verify deployment

## Configuration change patterns

### Adding a service

Enable the service in configuration:

```yaml
opencenter:
  services:
    cert-manager:
      enabled: true
      email: admin@example.com
```

Provide required secrets:

```yaml
secrets:
  cert_manager:
    aws_access_key: ${CERT_MANAGER_AWS_KEY}
    aws_secret_access_key: ${CERT_MANAGER_AWS_SECRET}
```

Run the workflow:

```bash
openCenter cluster validate my-cluster
openCenter cluster setup my-cluster --force
cd ~/.config/openCenter/clusters/myorg/gitops
git add . && git commit -m "Enable cert-manager" && git push
```

### Scaling workers

Update worker count:

```yaml
opencenter:
  cluster:
    kubernetes:
      worker_count: 5
```

For infrastructure changes, run bootstrap after setup:

```bash
openCenter cluster setup my-cluster --force
openCenter cluster bootstrap my-cluster
```

### Updating secrets

Edit encrypted secrets directly:

```bash
# Edit with SOPS
sops ~/.config/openCenter/clusters/myorg/gitops/applications/overlays/my-cluster/secrets/credentials.yaml

# Or update configuration and re-run setup
openCenter cluster setup my-cluster --force
```

## Troubleshooting

### Validation fails with missing secrets

Error: `secrets.service_name.field is required when service is enabled`

Solution: Add the required secret to your configuration or environment:

```bash
export SERVICE_SECRET="your-secret-value"
openCenter cluster validate my-cluster
```

### Setup fails with "already initialized"

Error: `GitOps repository already initialized`

Solution: Use `--force` to overwrite:

```bash
openCenter cluster setup my-cluster --force
```

### FluxCD not reconciling changes

Check FluxCD status:

```bash
kubectl get gitrepositories -n flux-system
kubectl describe kustomization <service-name> -n flux-system
```

Force reconciliation:

```bash
flux reconcile kustomization flux-system --with-source
```

### Changes not appearing in cluster

Verify the Git repository is accessible:

```bash
kubectl get gitrepositories -n flux-system
kubectl logs -n flux-system -l app=source-controller
```

Check for SOPS decryption issues:

```bash
kubectl get secrets sops-age -n flux-system
kubectl logs -n flux-system -l app=kustomize-controller | grep -i sops
```

## Rollback procedure

To revert changes:

1. Identify the last working commit:

```bash
cd ~/.config/openCenter/clusters/myorg/gitops
git log --oneline
```

2. Revert to that commit:

```bash
git revert <commit-hash>
git push origin main
```

3. FluxCD automatically applies the rollback.

For immediate rollback, use kubectl:

```bash
kubectl rollout undo deployment/<name> -n <namespace>
```

## Best practices

- **Validate before setup**: Catch errors early with `cluster validate`
- **Review diffs**: Always check `git diff` before pushing
- **Commit atomically**: One logical change per commit
- **Use descriptive messages**: Explain what changed and why
- **Test in staging**: Validate changes in a test cluster first
- **Monitor deployments**: Watch FluxCD reconciliation after pushing
- **Keep secrets encrypted**: Never commit plaintext secrets

## Related commands

- `openCenter cluster validate` - Validate configuration
- `openCenter cluster setup` - Regenerate GitOps repository
- `openCenter cluster bootstrap` - Apply infrastructure changes
- `openCenter sops secrets-encrypt` - Encrypt secrets files
- `flux reconcile` - Force FluxCD reconciliation

## Next steps

- [Monitoring Setup](monitoring.md) - Configure observability
- [Secrets Management](secrets-management.md) - Manage SOPS encryption
- [Troubleshooting](troubleshooting.md) - Debug common issues
