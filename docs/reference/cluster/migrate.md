# `openCenter cluster migrate` - Migrate Cluster Configurations


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Arguments](#arguments)
- [Options](#options)
- [Migration Process](#migration-process)
- [Examples](#examples)
- [Output](#output)
- [Directory Structure Changes](#directory-structure-changes)
- [Backup and Rollback](#backup-and-rollback)
- [Validation](#validation)
- [Exit Codes](#exit-codes)
- [Notes](#notes)
- [Troubleshooting](#troubleshooting)
- [See Also](#see-also)
## Synopsis
```bash
openCenter cluster migrate [cluster-name] [OPTIONS]
```

## Description

Migrate cluster configurations from the legacy flat directory structure to the new organization-based directory structure. This command can migrate individual clusters or all legacy clusters at once.

The migration process creates backups, moves files to the new structure, updates configuration with organization metadata, and validates the migration was successful.

## Arguments

### `[cluster-name]`
- **Required/Optional**: Optional
- **Description**: Name of the cluster to migrate. If not provided, migrates all legacy clusters (requires `--force`)
- **Example**: `my-cluster`

## Options

### `--organization <name>`
- **Description**: Target organization for migration
- **Type**: String
- **Default**: `opencenter`

### `--backup`
- **Description**: Create backup before migration
- **Type**: Boolean
- **Default**: `true`

### `--rollback <backup-path>`
- **Description**: Rollback cluster from specified backup path
- **Type**: String
- **Example**: `--rollback /path/to/backup.tar.gz`

### `--dry-run`
- **Description**: Show what would be migrated without performing the migration
- **Type**: Boolean
- **Default**: `false`

### `--force`
- **Description**: Force migration of all legacy clusters without confirmation
- **Type**: Boolean
- **Default**: `false`

### `-h, --help`
- **Description**: Display help information for this subcommand

## Migration Process

The migration performs the following steps:

1. **Detect Legacy Clusters** - Identifies clusters using old directory structure
2. **Create Backup** - Creates timestamped backup of cluster configuration (if `--backup` is true)
3. **Create Organization Structure** - Creates organization-based directory hierarchy
4. **Move Configuration** - Moves cluster configuration to organization directory
5. **Move Secrets** - Moves SOPS and SSH keys to organization secrets directory
6. **Move GitOps** - Moves GitOps repository to organization structure
7. **Update Configuration** - Updates configuration with organization metadata
8. **Update SOPS Config** - Updates SOPS configuration for organization structure
9. **Validate Migration** - Verifies migration was successful

## Examples

### Dry run to see what would be migrated
```bash
openCenter cluster migrate --dry-run
```
Output:
```
DRY RUN: Would migrate the following clusters to organization 'opencenter':
  - cluster1
  - cluster2
  - cluster3
```

### Migrate specific cluster
```bash
openCenter cluster migrate my-cluster
```

### Migrate to specific organization
```bash
openCenter cluster migrate my-cluster --organization production
```

### Migrate all legacy clusters
```bash
openCenter cluster migrate --force
```

### Migrate without backup
```bash
openCenter cluster migrate my-cluster --backup=false
```

### Rollback migration
```bash
openCenter cluster migrate --rollback /path/to/backup.tar.gz my-cluster
```

## Output

### Successful Migration

```
Migrating cluster 'my-cluster' to organization 'production'...
  Creating backup...
  Backup created at: /home/user/.config/openCenter/backups/my-cluster-20251117-103000.tar.gz
  Migrating files and directories...
  Validating migration...
  Successfully migrated cluster 'my-cluster'

Migration Summary:
  Successfully migrated: 1 clusters
    - my-cluster

Backups created:
  /home/user/.config/openCenter/backups/my-cluster-20251117-103000.tar.gz

To rollback a cluster, use: openCenter cluster migrate --rollback <backup-path> <cluster-name>
```

### Migration with Failures

```
Migrating cluster 'cluster1' to organization 'opencenter'...
  Creating backup...
  Backup created at: /home/user/.config/openCenter/backups/cluster1-20251117-103000.tar.gz
  Migrating files and directories...
  Migration failed for cluster 'cluster1': directory already exists
  Attempting rollback...
  Rollback successful

Migration Summary:
  Successfully migrated: 0 clusters
  Failed to migrate: 1 clusters
    - cluster1

Backups created:
  /home/user/.config/openCenter/backups/cluster1-20251117-103000.tar.gz
```

## Directory Structure Changes

### Before Migration (Legacy)

```
~/.config/openCenter/
├── clusters/
│   ├── my-cluster/
│   │   ├── config.yaml
│   │   ├── secrets/
│   │   └── gitops/
│   └── .active
```

### After Migration (Organization-Based)

```
~/.config/openCenter/
├── clusters/
│   ├── production/
│   │   ├── .my-cluster-config.yaml
│   │   ├── .sops.yaml
│   │   ├── secrets/
│   │   │   ├── age/keys/
│   │   │   └── ssh/
│   │   └── gitops/
│   │       ├── applications/overlays/my-cluster/
│   │       └── infrastructure/clusters/my-cluster/
│   └── .active
```

## Backup and Rollback

### Backup Location

Backups are stored in:
```
~/.config/openCenter/backups/<cluster>-<timestamp>.tar.gz
```

### Backup Contents

- Cluster configuration file
- Secrets directory (SOPS and SSH keys)
- GitOps repository
- Metadata file with migration information

### Rollback Process

```bash
# List available backups
ls ~/.config/openCenter/backups/

# Rollback specific cluster
openCenter cluster migrate --rollback ~/.config/openCenter/backups/my-cluster-20251117-103000.tar.gz my-cluster
```

## Validation

The migration validates:

- Configuration file exists in new location
- Secrets are accessible in new location
- GitOps repository structure is correct
- SOPS configuration is updated
- Cluster can be loaded with new structure

## Exit Codes

- `0` - Migration successful
- `1` - Migration failed or validation errors

## Notes

- Backups are created by default before migration
- Use `--dry-run` to preview migration without making changes
- Migration is atomic - failures trigger automatic rollback
- The `--force` flag is required to migrate all clusters at once
- Legacy clusters are detected automatically
- Organization structure is created if it doesn't exist
- SOPS configuration is updated for organization-based paths
- GitOps repository is moved to organization directory
- Active cluster selection is preserved after migration
- Backups are timestamped for easy identification

## Troubleshooting

### No legacy clusters found
**Output**: `No legacy clusters found to migrate.`

**Solution**: All clusters are already using organization-based structure.

### Cluster already exists in organization
**Error**: `Migration failed: cluster 'my-cluster' already exists in organization 'production'`

**Solution**: Choose a different organization or remove the existing cluster:
```bash
openCenter cluster migrate my-cluster --organization other-org
```

### Backup creation failed
**Error**: `Failed to create backup for cluster 'my-cluster'`

**Solution**: Check disk space and permissions:
```bash
df -h ~/.config/openCenter/
ls -la ~/.config/openCenter/backups/
```

### Rollback failed
**Error**: `Rollback also failed: backup file not found`

**Solution**: Verify backup path:
```bash
ls -la ~/.config/openCenter/backups/
```

## See Also

- `openCenter cluster list` - List all clusters
- `openCenter cluster init` - Initialize new cluster with organization
- `openCenter cluster validate` - Validate migrated configuration
