# cluster destroy


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Arguments](#arguments)
- [Flags](#flags)
- [Examples](#examples)
- [Confirmation Prompt](#confirmation-prompt)
- [Deletion Process](#deletion-process)
- [Directory Structure Handling](#directory-structure-handling)
- [Locking](#locking)
- [Output](#output)
- [Active Cluster Handling](#active-cluster-handling)
- [Error Handling](#error-handling)
- [Recovery](#recovery)
- [See Also](#see-also)
**doc_type:** reference

Destroy cluster infrastructure and clean up resources.

## Synopsis

```bash
openCenter cluster destroy <name> [flags]
```

## Description

The `cluster destroy` command permanently removes cluster infrastructure, configuration files, and related resources. It deletes the cluster directory structure, GitOps repository, configuration file, and clears the active cluster marker if the destroyed cluster was active.

**Warning:** This operation is irreversible. All cluster data, configuration, and infrastructure will be permanently deleted.

## Arguments

- `name` - Cluster name (required)

## Flags

- `--force` - Skip confirmation prompt

## Examples

```bash
# Destroy cluster (with confirmation)
openCenter cluster destroy my-cluster

# Force destroy without confirmation
openCenter cluster destroy my-cluster --force
```

## Confirmation Prompt

Unless `--force` is used, the command displays a warning:

```
WARNING: This will permanently destroy cluster "my-cluster" in organization "myorg".
```

In non-interactive mode (tests), confirmation is skipped.

## Deletion Process

The destroy operation removes resources in this order:

1. **Update cluster status** - Sets status to "destroyed" (skipped for flat configs)
2. **Remove GitOps directory** - Deletes `gitops.git_dir` if configured
3. **Remove cluster directories** - Deletes cluster-specific directories based on structure type
4. **Remove configuration file** - Deletes the cluster configuration YAML file
5. **Clear active marker** - Removes active cluster marker if this cluster was active

## Directory Structure Handling

### Organization-Based Structure

For clusters in organization-based structure:

Removes:
- Cluster directory: `clusters/<org>/infrastructure/clusters/<cluster>/`
- Applications directory: `clusters/<org>/applications/overlays/<cluster>/`
- Configuration file: `clusters/<org>/.<cluster>-config.yaml`

### Legacy Structure

For clusters in legacy structure:

Removes:
- Cluster directory: `clusters/<cluster>/`
- Configuration file: `clusters/<cluster>/.<cluster>-config.yaml`

### Flat Configuration

For flat configuration files (not in clusters directory):

Removes:
- Configuration file only (no cluster directory)

## Locking

Destroy operations acquire an exclusive lock on the cluster to prevent concurrent modifications. Lock duration: 1 hour.

If another operation is in progress:
```
Error: failed to acquire lock for cluster "my-cluster": lock already held
Another operation may be in progress. Wait for it to complete or use 'openCenter cluster info my-cluster' to check lock status
```

## Output

Successful destroy operation displays:

```
Removed GitOps directory: /path/to/gitops
Removed cluster directory: /path/to/clusters/org/infrastructure/clusters/my-cluster
Removed applications directory: /path/to/clusters/org/applications/overlays/my-cluster
Removed config file: /path/to/clusters/org/.my-cluster-config.yaml
Cleared active cluster marker
Cluster "my-cluster" destroyed successfully.
```

## Active Cluster Handling

If the destroyed cluster was the active cluster:
- Active cluster marker is cleared
- User must select a new active cluster with `cluster select`

## Error Handling

Common errors:

**Cluster not found:**
```
Error: failed to load cluster configuration: cluster "my-cluster" not found
```

**Lock acquisition failed:**
```
Error: failed to acquire lock for cluster "my-cluster": lock already held
```

**Directory removal failed:**
```
Error: failed to remove cluster directory: permission denied
```

## Recovery

If destroy fails partway through:
- Partial cleanup may have occurred
- Check output to see which resources were removed
- Manually remove remaining resources if necessary
- Lock will be automatically released after timeout

## See Also

- [cluster init](init.md) - Initialize new cluster configuration
- [cluster backup](backup.md) - Create cluster backups before destruction
- [cluster list](list.md) - List all configured clusters
- [cluster info](info.md) - Display cluster information
