# cluster status


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Flags](#flags)
- [Examples](#examples)
- [Output Format](#output-format)
- [Information Displayed](#information-displayed)
- [Status-Based Next Steps](#status-based-next-steps)
- [Use Cases](#use-cases)
- [Error Handling](#error-handling)
- [See Also](#see-also)
**doc_type:** reference

Show the current active cluster and its status information.

## Synopsis

```bash
openCenter cluster status [flags]
```

## Description

The `cluster status` command displays information about the currently active cluster, including metadata, status, and optionally file paths and their availability.

If no cluster is active, it shows available clusters and suggests using `cluster select` to set one.

## Flags

- `--paths` - Show cluster file paths and their status
- `--quiet, -q` - Quiet output (just the cluster name)

## Examples

```bash
# Show active cluster status
openCenter cluster status

# Show active cluster with file paths
openCenter cluster status --paths

# Quiet output (just the cluster name)
openCenter cluster status --quiet

# Use in scripts
CLUSTER=$(openCenter cluster status --quiet)
```

## Output Format

### Default Output

```
Active Cluster: my-cluster
  Name:         my-cluster
  Environment:  prod
  Region:       us-east-1
  Status:       deployed
  Organization: myorg
  Provider:     openstack

Next Steps:
  - Run 'eval $(openCenter cluster activate)' to configure your environment
  - Use 'kubectl' to interact with the cluster
```

### With --paths Flag

```
Active Cluster: my-cluster
  Name:         my-cluster
  Environment:  prod
  Region:       us-east-1
  Status:       deployed
  Organization: myorg
  Provider:     openstack

Cluster Paths:
  Config Directory:  /home/user/.config/openCenter/clusters/myorg/infrastructure/clusters/my-cluster
  SOPS Key:          /home/user/.config/openCenter/clusters/myorg/secrets/age/my-cluster-key.txt
  GitOps Directory:  /home/user/gitops/myorg
  SOPS Key Status:   ✓ Present
  GitOps Status:     ✓ Initialized
  Kubeconfig:        ✓ Present

Next Steps:
  - Run 'eval $(openCenter cluster activate)' to configure your environment
  - Use 'kubectl' to interact with the cluster
```

### Quiet Output

```
my-cluster
```

### No Active Cluster

```
No active cluster set

Available clusters:
  - cluster1
  - cluster2
  - my-cluster

Use 'openCenter cluster select <name>' to set an active cluster
```

## Information Displayed

### Cluster Metadata
- **Name** - Cluster display name
- **Environment** - Environment (dev, staging, prod)
- **Region** - Cloud region
- **Status** - Current cluster status
- **Organization** - Organization name
- **Provider** - Infrastructure provider

### File Paths (with --paths)
- **Config Directory** - Cluster configuration directory
- **SOPS Key** - SOPS Age encryption key path
- **GitOps Directory** - GitOps repository directory
- **SOPS Key Status** - Whether key file exists
- **GitOps Status** - Whether GitOps directory is initialized
- **Kubeconfig** - Whether kubeconfig file exists

## Status-Based Next Steps

The command suggests next steps based on cluster status:

### initialized
```
Next Steps:
  - Run 'openCenter cluster validate my-cluster' to validate configuration
  - Run 'openCenter cluster setup my-cluster' to generate GitOps repository
```

### validated
```
Next Steps:
  - Run 'openCenter cluster setup my-cluster' to generate GitOps repository
```

### setup or ready
```
Next Steps:
  - Run 'openCenter cluster bootstrap my-cluster' to deploy the cluster
```

### deployed
```
Next Steps:
  - Run 'eval $(openCenter cluster activate)' to configure your environment
  - Use 'kubectl' to interact with the cluster
```

## Use Cases

### Quick Status Check

```bash
# Check active cluster
openCenter cluster status
```

### Verify File Paths

```bash
# Check if all required files exist
openCenter cluster status --paths
```

### Scripting Integration

```bash
# Get active cluster name
CLUSTER=$(openCenter cluster status --quiet)

# Check if cluster is active
if [ -z "$CLUSTER" ]; then
  echo "No active cluster"
  exit 1
fi

# Use cluster name in other commands
openCenter cluster validate "$CLUSTER"
```

### CI/CD Integration

```bash
#!/bin/bash
set -e

# Verify active cluster
CLUSTER=$(openCenter cluster status --quiet)
if [ -z "$CLUSTER" ]; then
  echo "Error: No active cluster set"
  exit 1
fi

# Check cluster status
STATUS=$(openCenter cluster info "$CLUSTER" --json | jq -r '.metadata.status')
if [ "$STATUS" != "deployed" ]; then
  echo "Error: Cluster is not deployed (status: $STATUS)"
  exit 1
fi

# Proceed with operations
echo "Operating on cluster: $CLUSTER"
```

### Troubleshooting

```bash
# Check file availability
openCenter cluster status --paths

# Verify SOPS key exists
if openCenter cluster status --paths | grep -q "SOPS Key Status:   ✗ Missing"; then
  echo "SOPS key is missing, regenerating..."
  openCenter cluster init my-cluster --regenerate-keys
fi
```

## Error Handling

**No active cluster:**
```
No active cluster set

Available clusters:
  - cluster1
  - cluster2

Use 'openCenter cluster select <name>' to set an active cluster
```

**Configuration not found:**
```
Active cluster: my-cluster
Status: Configuration not found or invalid
```

**Failed to load configuration:**
```
Error: failed to get active cluster: no active cluster marker found
```

## See Also

- [cluster select](../cli-commands.md#cluster-select) - Set active cluster
- [cluster current](../cli-commands.md#cluster-current) - Display active cluster name
- [cluster info](info.md) - Display detailed cluster information
- [cluster list](list.md) - List all configured clusters
