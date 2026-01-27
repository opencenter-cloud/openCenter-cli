# Working with v2 Configurations

## Overview

This guide explains how to create, validate, and manage v2 cluster configurations in opencenter-cli.

## Creating New v2 Configurations

### Using cluster init (Default)

The `cluster init` command creates new v2 configurations by default:

```bash
# Create a new v2 configuration (default)
opencenter cluster init my-cluster

# Explicitly specify v2 schema
opencenter cluster init my-cluster --schema-version=2.0

# Create v2 baremetal cluster
opencenter cluster init my-cluster --type baremetal --schema-version=2.0
```

**Output:**
```
Generated ed25519 SSH key pair at ~/.config/opencenter/clusters/opencenter/secrets/ssh/my-cluster-dev-local
Created cluster configuration in organization 'opencenter' at ~/.config/opencenter/clusters/opencenter/my-cluster/
GitOps repository root: ~/.config/opencenter/clusters/opencenter/gitops
SOPS key location: ~/.config/opencenter/clusters/opencenter/secrets/age/my-cluster-key.txt
```

### Schema Version Flag

```bash
# v2 (default, recommended)
opencenter cluster init my-cluster --schema-version=2.0

# v1 (legacy, for backward compatibility)
opencenter cluster init my-cluster --schema-version=1.0
```

## Loading Existing v2 Configurations

### Important: cluster init vs Other Commands

**`cluster init` is for creating NEW configurations only.** To work with existing v2 configs, use:

- `cluster validate` - Validate existing v2 configuration
- `cluster update` - Update existing v2 cluster
- `cluster render` - Render GitOps manifests from v2 config
- `cluster setup` - Setup GitOps repository from v2 config

### Validating v2 Configurations

```bash
# Validate v2 configuration file
opencenter cluster validate --config my-cluster-v2.yaml

# Validate active cluster (auto-detects v2)
opencenter cluster validate
```

**Expected output:**
```
✓ Schema validation passed (v2.0)
✓ Business rules validation passed
✓ Provider validation passed (baremetal)
✓ Deployment validation passed (Kubespray)
✓ Service dependencies validated
✓ Configuration is valid ✓
```

### Updating v2 Clusters

```bash
# Update cluster from v2 config file
opencenter cluster update --config my-cluster-v2.yaml

# Update active cluster
opencenter cluster update
```

## Migrating from v1 to v2

### Automatic Migration

```bash
# Migrate v1 config to v2
opencenter cluster migrate-config \
  --input my-cluster-v1.yaml \
  --output my-cluster-v2.yaml

# Validate migrated config
opencenter cluster validate --config my-cluster-v2.yaml
```

### Migration Report

The migration tool provides detailed feedback:

```
Migration Report for: my-cluster
================================

Schema Version: v1.0 → v2.0

Field Relocations:
------------------
✓ cluster.networking.vrrp_ip → infrastructure.networking.vrrp_ip
✓ opencenter.storage.* → infrastructure.storage.*
✓ opencenter.deployment.* → deployment.*

Applied Defaults (Hydration):
------------------------------
✓ infrastructure.cloud.openstack.image_id: "799dcf97..." (provider-region: sjc3)

Validation: PASSED ✓
```

## Common Workflows

### Create and Validate

```bash
# Create v2 config
opencenter cluster init prod-cluster --schema-version=2.0

# Validate before deployment
opencenter cluster validate

# Setup GitOps repository
opencenter cluster setup
```

### Edit and Validate

```bash
# Edit configuration file
vim ~/.config/opencenter/clusters/opencenter/.prod-cluster-config.yaml

# Validate changes
opencenter cluster validate

# Apply changes
opencenter cluster update
```

### Override Values During Init

```bash
# Create v2 config with custom values
opencenter cluster init my-cluster \
  --schema-version=2.0 \
  --org myorg \
  --opencenter.meta.env=prod \
  --opencenter.cluster.kubernetes.version=1.31.4 \
  --opencenter.infrastructure.provider=baremetal
```

## Troubleshooting

### Error: v2 configuration detected

**Problem:**
```
Error: failed to load configuration from file 'k8s-uat-2.yaml': 
v2 configuration detected: use v2.ConfigLoader for loading v2 configurations
```

**Solution:**

This error occurs when using `cluster init --config` with a v2 configuration file. The `cluster init` command is designed for creating NEW configurations, not loading existing ones.

**Use the correct command:**

```bash
# For validation
opencenter cluster validate --config k8s-uat-2.yaml

# For updates
opencenter cluster update --config k8s-uat-2.yaml

# For rendering manifests
opencenter cluster render --config k8s-uat-2.yaml

# For GitOps setup
opencenter cluster setup --config k8s-uat-2.yaml
```

**If you want to create a NEW config based on an existing template:**

```bash
# Copy the template
cp k8s-uat-2.yaml my-new-cluster.yaml

# Edit the cluster name
vim my-new-cluster.yaml  # Change opencenter.meta.name and opencenter.cluster.cluster_name

# Create new cluster from template (without --config flag)
opencenter cluster init my-new-cluster \
  --schema-version=2.0 \
  --opencenter.meta.organization=myorg
```

### Error: invalid schema version

**Problem:**
```
Error: invalid schema version '3.0': must be 1.0 or 2.0
```

**Solution:**

Only v1.0 and v2.0 are supported. Use v2.0 for new clusters:

```bash
opencenter cluster init my-cluster --schema-version=2.0
```

### Missing Required Fields

**Problem:**
```
E001: opencenter.infrastructure.networking.vrrp_ip: required field is missing
```

**Solution:**

Edit the configuration file and add the missing field:

```yaml
opencenter:
  infrastructure:
    networking:
      vrrp_ip: "192.168.1.5"
```

Then validate:

```bash
opencenter cluster validate
```

## Command Reference

### Commands that Support v2

| Command | v2 Support | Purpose |
|---------|-----------|---------|
| `cluster init` | ✓ (creates v2 by default) | Create new configuration |
| `cluster validate` | ✓ | Validate configuration |
| `cluster update` | ✓ | Update existing cluster |
| `cluster render` | ✓ | Render GitOps manifests |
| `cluster setup` | ✓ | Setup GitOps repository |
| `cluster migrate-config` | ✓ | Migrate v1 to v2 |
| `cluster bootstrap` | ✓ | Bootstrap cluster |
| `cluster destroy` | ✓ | Destroy cluster |

### Commands that Load Existing Configs

These commands can load and work with existing v2 configurations:

```bash
# Validation
opencenter cluster validate --config my-cluster-v2.yaml

# Updates
opencenter cluster update --config my-cluster-v2.yaml

# Rendering
opencenter cluster render --config my-cluster-v2.yaml

# Setup
opencenter cluster setup --config my-cluster-v2.yaml
```

## Best Practices

1. **Always use v2 for new clusters**: v2 is the current schema and will receive all new features
2. **Validate before deployment**: Run `cluster validate` before `cluster bootstrap`
3. **Use organization structure**: Organize clusters by organization for better management
4. **Version control your configs**: Store cluster configs in Git for change tracking
5. **Test migrations in dev first**: Migrate dev/test clusters before production

## Related Documentation

- [v2 Configuration Reference](../cluster-config/v2-reference.md)
- [Migration Guide](../cluster-config/migration-guide.md)
- [Baremetal Schema Comparison](../cluster-config/examples/baremetal-schema-comparison.md)
- [CLI Commands Reference](../reference/cli-commands.md)

---

**Last Updated**: January 2026  
**Schema Version**: v2.0 (default)
