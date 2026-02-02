# v2.0.0 Configuration Field Structure

**doc_type: reference**

This document describes the v2.0.0 configuration field structure and the changes from v1.x.

## Table of Contents

- [Overview](#overview)
- [Field Structure Changes](#field-structure-changes)
- [Migration Examples](#migration-examples)
- [Validation](#validation)
- [FAQ](#faq)

## Overview

opencenter-cli v2.0.0 introduces a new configuration field structure that better organizes cluster settings. The new structure moves fields from `opencenter.cluster.*` to `opencenter.infrastructure.*` for better logical grouping.

**Key Changes**:
- Networking configuration moved from `cluster.networking` to `infrastructure.networking`
- Compute configuration moved from `cluster.kubernetes` to `infrastructure.compute`
- Storage configuration moved from `opencenter.storage` to `infrastructure.storage`

## Field Structure Changes

### Networking Fields

**v1.x Location** (deprecated):
```yaml
opencenter:
  cluster:
    networking:
      vrrp_ip: "10.0.0.1"
      subnet_pods: "10.244.0.0/16"
      subnet_services: "10.96.0.0/12"
```

**v2.0.0 Location** (required):
```yaml
opencenter:
  infrastructure:
    networking:
      vrrp_ip: "10.0.0.1"
      subnet_pods: "10.244.0.0/16"
      subnet_services: "10.96.0.0/12"
```

### Compute Fields

**v1.x Location** (deprecated):
```yaml
opencenter:
  cluster:
    kubernetes:
      flavor_control_plane: "m1.large"
      flavor_worker: "m1.medium"
      flavor_etcd: "m1.small"
      master_count: 3
      worker_count: 5
```

**v2.0.0 Location** (required):
```yaml
opencenter:
  infrastructure:
    compute:
      flavor_control_plane: "m1.large"
      flavor_worker: "m1.medium"
      flavor_etcd: "m1.small"
      master_count: 3
      worker_count: 5
```

### Storage Fields

**v1.x Location** (deprecated):
```yaml
opencenter:
  storage:
    type: "ceph"
    default_storage_class: "rbd"
    size: "100Gi"
```

**v2.0.0 Location** (required):
```yaml
opencenter:
  infrastructure:
    storage:
      type: "ceph"
      default_storage_class: "rbd"
      size: "100Gi"
```

## Migration Examples

### Example 1: Complete Configuration Migration

**v1.x Configuration**:
```yaml
schema_version: "1.0"
opencenter:
  meta:
    name: "my-cluster"
    region: "ord1"
  cluster:
    networking:
      vrrp_ip: "10.0.0.1"
      subnet_pods: "10.244.0.0/16"
    kubernetes:
      flavor_control_plane: "m1.large"
      flavor_worker: "m1.medium"
      master_count: 3
      worker_count: 5
  storage:
    type: "ceph"
```

**v2.0.0 Configuration**:
```yaml
schema_version: "2.0"
opencenter:
  meta:
    name: "my-cluster"
    region: "ord1"
  infrastructure:
    networking:
      vrrp_ip: "10.0.0.1"
      subnet_pods: "10.244.0.0/16"
    compute:
      flavor_control_plane: "m1.large"
      flavor_worker: "m1.medium"
      master_count: 3
      worker_count: 5
    storage:
      type: "ceph"
```

### Example 2: Minimal Configuration

**v1.x**:
```yaml
opencenter:
  cluster:
    networking:
      vrrp_ip: "10.0.0.1"
```

**v2.0.0**:
```yaml
opencenter:
  infrastructure:
    networking:
      vrrp_ip: "10.0.0.1"
```

## Validation

opencenter-cli v2.0.0 includes validation to detect and reject v1 field locations.

### Validation Errors

When a v1 field location is detected, you'll see an error like:

```
Error: Configuration uses v1 field location: opencenter.cluster.networking.vrrp_ip
  v2.0.0 requires v2 field structure
  Migration: opencenter.cluster.networking.vrrp_ip → opencenter.infrastructure.networking.vrrp_ip
  To upgrade: Install opencenter v1.x and run 'opencenter cluster migrate-config'
```

### Automatic Migration

To migrate your configuration:

1. **Install opencenter v1.x** (if not already installed):
   ```bash
   # Using homebrew
   brew install opencenter@1
   
   # Or download from releases
   # https://github.com/rackerlabs/opencenter-cli/releases
   ```

2. **Run migration command**:
   ```bash
   opencenter cluster migrate-config <cluster-name>
   ```

3. **Verify migration**:
   ```bash
   opencenter cluster validate <cluster-name>
   ```

4. **Upgrade to v2.0.0**:
   ```bash
   brew upgrade opencenter
   ```

## FAQ

### Q: Why did the field structure change?

**A**: The new structure provides better logical grouping of infrastructure-related settings. In v1.x, infrastructure settings were scattered across `cluster.networking`, `cluster.kubernetes`, and `opencenter.storage`. In v2.0.0, all infrastructure settings are under `infrastructure.*`.

### Q: Can I use v1 field locations in v2.0.0?

**A**: No. v2.0.0 only supports v2 field structure. You must migrate your configuration using opencenter v1.x before upgrading to v2.0.0.

### Q: Will my v1 configuration break?

**A**: Yes, if you try to use it with v2.0.0. You must migrate to v2 field structure first using the migration command in v1.x.

### Q: How do I know if my configuration is v1 or v2?

**A**: Check the `schema_version` field:
- v1: `schema_version: "1.0"` or missing
- v2: `schema_version: "2.0"`

Also check field locations:
- v1: Fields under `opencenter.cluster.networking`, `opencenter.cluster.kubernetes`, `opencenter.storage`
- v2: Fields under `opencenter.infrastructure.networking`, `opencenter.infrastructure.compute`, `opencenter.infrastructure.storage`

### Q: What if I have multiple clusters?

**A**: You need to migrate each cluster individually:
```bash
# List all clusters
opencenter cluster list

# Migrate each cluster
opencenter cluster migrate-config cluster1
opencenter cluster migrate-config cluster2
opencenter cluster migrate-config cluster3
```

Or use the batch migration script:
```bash
# Download migration script
curl -O https://raw.githubusercontent.com/rackerlabs/opencenter-cli/main/scripts/migrate-to-v2.sh

# Run batch migration
bash migrate-to-v2.sh
```

### Q: Can I manually edit my configuration?

**A**: Yes, but using the migration command is recommended as it ensures all fields are correctly relocated and validates the result. If you choose to manually edit:

1. Change `schema_version` to `"2.0"`
2. Move all fields from v1 locations to v2 locations (see examples above)
3. Validate the configuration: `opencenter cluster validate <cluster-name>`

### Q: What happens if I miss a field during manual migration?

**A**: The validation will catch it and show an error with the exact field location that needs to be updated.

### Q: Are there any other breaking changes in v2.0.0?

**A**: Yes. See the [BREAKING_CHANGES.md](../../BREAKING_CHANGES.md) document for a complete list of breaking changes in v2.0.0.

## References

- [v2.0.0 Breaking Changes](../../BREAKING_CHANGES.md)
- [Migration Guide](migration-guide.md)
- [Architecture Documentation](architecture.md)
- [v2.0.0 Release Notes](../../CHANGELOG.md)
