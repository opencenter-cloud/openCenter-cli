# cluster render


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Arguments](#arguments)
- [Examples](#examples)
- [Rendering Process](#rendering-process)
- [Output Structure](#output-structure)
- [Organization Support](#organization-support)
- [Output](#output)
- [Use Cases](#use-cases)
- [Comparison with cluster setup](#comparison-with-cluster-setup)
- [Error Handling](#error-handling)
- [See Also](#see-also)
**doc_type:** reference

Render cluster templates into the GitOps directory structure.

## Synopsis

```bash
openCenter cluster render [name]
```

## Description

The `cluster render` command renders cluster templates into the GitOps directory structure. It always renders templates without any initialization checks, making it ideal for iterative development and testing configuration changes.

Unlike `cluster setup`, this command:
- Always renders templates (no skip logic)
- Does not perform Git operations
- Does not check if directory already exists
- Overwrites existing files
- Perfect for development and testing

## Arguments

- `name` - Cluster name (optional if active cluster is set)

## Examples

```bash
# Render templates for active cluster
openCenter cluster render

# Render for specific cluster
openCenter cluster render my-cluster
```

## Rendering Process

The command performs these operations:

1. **Copy Base GitOps Structure**
   - Copies base templates from embedded resources
   - Creates directory structure
   - Installs base manifests

2. **Render Cluster-Specific Applications**
   - Renders application overlays for the cluster
   - Applies cluster-specific customizations
   - Generates Kustomization files

3. **Render Infrastructure Templates**
   - Renders infrastructure cluster manifests
   - Generates provider-specific configurations
   - Creates namespace and RBAC resources

4. **Provision OpenTofu**
   - Renders `main.tf` with cluster configuration
   - Generates `provider.tf` for cloud provider
   - Creates Terraform variable files

## Output Structure

Templates are rendered to:
```
<git_dir>/
├── infrastructure/
│   └── clusters/<cluster>/
│       ├── main.tf                    # Terraform main configuration
│       ├── provider.tf                # Provider configuration
│       ├── variables.tf               # Variable definitions
│       ├── kustomization.yaml         # Kustomization manifest
│       └── ...                        # Other infrastructure files
└── applications/
    └── overlays/<cluster>/
        ├── kustomization.yaml         # Application kustomization
        └── ...                        # Application manifests
```

## Organization Support

The command handles organization-based directory structures automatically:

```
<git_dir>/<organization>/
├── infrastructure/
│   └── clusters/<cluster>/
└── applications/
    └── overlays/<cluster>/
```

## Output

```
Rendering templates for cluster: my-cluster
Organization: myorg
Rendering templates to: /path/to/gitops/myorg/infrastructure/clusters/my-cluster
Render complete.
```

## Use Cases

### Iterative Development

Test configuration changes without full setup:
```bash
# Edit configuration
openCenter cluster edit my-cluster

# Render templates to see changes
openCenter cluster render my-cluster

# Review rendered files
ls -la ~/gitops/myorg/infrastructure/clusters/my-cluster/
```

### Template Debugging

Debug template rendering issues:
```bash
# Render templates
openCenter cluster render my-cluster

# Check for errors in rendered files
cat ~/gitops/myorg/infrastructure/clusters/my-cluster/main.tf
```

### Configuration Testing

Test different configurations quickly:
```bash
# Update configuration
openCenter cluster update my-cluster --opencenter.cluster.kubernetes.version=1.31.4

# Re-render templates
openCenter cluster render my-cluster

# Verify changes
grep "kubernetes_version" ~/gitops/myorg/infrastructure/clusters/my-cluster/main.tf
```

### CI/CD Integration

Generate templates in CI/CD pipelines:
```bash
#!/bin/bash
set -e

# Render templates
openCenter cluster render my-cluster

# Validate rendered Terraform
cd ~/gitops/myorg/infrastructure/clusters/my-cluster
terraform validate

# Validate Kubernetes manifests
kubectl apply --dry-run=client -k ~/gitops/myorg/applications/overlays/my-cluster/
```

## Comparison with cluster setup

| Feature | render | setup |
|---------|--------|-------|
| Renders templates | ✓ | ✓ |
| Git initialization | ✗ | ✓ |
| Checks existing directory | ✗ | ✓ |
| Skip logic | ✗ | ✓ |
| Overwrites files | ✓ | Conditional |
| Use case | Development/testing | Initial setup |

## Error Handling

**Cluster not found:**
```
Error: failed to load cluster configuration: cluster "my-cluster" not found
```

**GitOps directory not configured:**
```
Error: git_dir is not configured. Run 'openCenter cluster setup' first or set git_dir in the configuration
```

**Template rendering failed:**
```
Error: failed to render cluster templates: template execution error
```

**OpenTofu provisioning failed:**
```
Error: failed to provision opentofu: invalid configuration
```

## See Also

- [cluster setup](setup.md) - Setup GitOps directory structure with Git initialization
- [cluster validate](../cli-commands.md#cluster-validate) - Validate cluster configuration
- [cluster bootstrap](bootstrap.md) - Bootstrap cluster infrastructure
