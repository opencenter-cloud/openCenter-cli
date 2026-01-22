# cluster info


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Arguments](#arguments)
- [Flags](#flags)
- [Examples](#examples)
- [Output Format](#output-format)
- [Information Displayed](#information-displayed)
- [Active Cluster Indicator](#active-cluster-indicator)
- [Validation Mode](#validation-mode)
- [Export-Only Mode](#export-only-mode)
- [Shell-Specific Export](#shell-specific-export)
- [Lock Status](#lock-status)
- [Use Cases](#use-cases)
- [See Also](#see-also)
**doc_type:** reference

Display detailed information about a cluster.

## Synopsis

```bash
openCenter cluster info [name] [flags]
```

## Description

The `cluster info` command displays comprehensive information about a cluster, including metadata, configuration paths, GitOps settings, and lock status.

## Arguments

- `name` - Cluster name (optional if active cluster is set)

## Flags

- `--validate` - Validate cluster configuration invariants
- `--json` - Output JSON instead of YAML
- `--export-only` - Only output export commands for shell evaluation
- `--shell string` - Override shell detection (bash, zsh, fish, powershell)

## Examples

```bash
# Show info for active cluster
openCenter cluster info

# Show info for specific cluster
openCenter cluster info my-cluster

# Validate configuration
openCenter cluster info my-cluster --validate

# Output as JSON
openCenter cluster info my-cluster --json

# Export only environment variables
openCenter cluster info my-cluster --export-only

# Export for specific shell
openCenter cluster info my-cluster --export-only --shell=fish
```

## Output Format

### Default (YAML)

```
Active cluster: my-cluster
Config Path: /home/user/.config/openCenter/clusters/myorg/.my-cluster-config.yaml

git_dir: /home/user/gitops/myorg
git_url: git@github.com:myorg/gitops.git

Metadata:
  name: my-cluster
  cluster_name: my-cluster
  organization: myorg
  provider: openstack
  env: prod
  region: us-east-1
  status: deployed

Lock Status:
  status: available
```

### JSON Format

```json
{
  "config_path": "/home/user/.config/openCenter/clusters/myorg/.my-cluster-config.yaml",
  "cluster_name": "my-cluster",
  "organization": "myorg",
  "provider": "openstack",
  "metadata": {
    "name": "my-cluster",
    "env": "prod",
    "region": "us-east-1",
    "status": "deployed",
    "organization": "myorg"
  },
  "git_dir": "/home/user/gitops/myorg",
  "git_url": "git@github.com:myorg/gitops.git"
}
```

### Export-Only Format

```bash
export OPENCENTER_CLUSTER="my-cluster"
export OPENCENTER_ORGANIZATION="myorg"
export OPENCENTER_PROVIDER="openstack"
export OPENCENTER_ENV="prod"
export OPENCENTER_REGION="us-east-1"
export KUBECONFIG="/home/user/gitops/myorg/kubeconfig.yaml"
```

## Information Displayed

### Cluster Identification
- Cluster name
- Organization
- Configuration file path

### GitOps Configuration
- `git_dir` - GitOps repository directory
- `git_url` - GitOps repository URL

### Metadata
- `name` - Cluster display name
- `cluster_name` - Cluster identifier
- `organization` - Organization name
- `provider` - Infrastructure provider (openstack, aws, kind, etc.)
- `env` - Environment (dev, staging, prod)
- `region` - Cloud region
- `status` - Cluster status (initialized, validated, deployed, etc.)

### Lock Status
- `status` - Lock availability (available or locked)
- `message` - Lock status message if locked

## Active Cluster Indicator

The command shows "Active cluster:" prefix when:
- The cluster is the currently active cluster, OR
- The current working directory is the cluster's GitOps directory

## Validation Mode

With `--validate` flag, the command validates configuration and reports errors:

```bash
openCenter cluster info my-cluster --validate
```

**Success:**
```
Validation successful.
```

**Failure:**
```
Error: opencenter.cluster.kubernetes.version: required field missing
Error: opencenter.infrastructure.provider: must be one of: openstack, aws, kind
validation failed
```

## Export-Only Mode

The `--export-only` flag outputs only environment variable export commands, suitable for shell evaluation:

```bash
eval $(openCenter cluster info my-cluster --export-only)
```

This sets environment variables for:
- Cluster identification
- Organization
- Provider
- Environment
- Region
- Kubeconfig path

## Shell-Specific Export

Use `--shell` to generate shell-specific export syntax:

**Bash/Zsh:**
```bash
export VAR="value"
```

**Fish:**
```fish
set -x VAR "value"
```

**PowerShell:**
```powershell
$env:VAR = "value"
```

## Lock Status

The command checks if the cluster is locked by another operation:

**Available:**
```
Lock Status:
  status: available
```

**Locked:**
```
Lock Status:
  status: locked
  message: Another operation is in progress on this cluster
```

## Use Cases

### Quick Status Check
```bash
openCenter cluster info
```

### Configuration Validation
```bash
openCenter cluster info my-cluster --validate
```

### Environment Setup
```bash
eval $(openCenter cluster info my-cluster --export-only)
```

### Scripting Integration
```bash
CLUSTER_INFO=$(openCenter cluster info my-cluster --json)
PROVIDER=$(echo "$CLUSTER_INFO" | jq -r '.provider')
```

## See Also

- [cluster status](status.md) - Show active cluster status
- [cluster validate](../cli-commands.md#cluster-validate) - Validate cluster configuration
- [cluster select](../cli-commands.md#cluster-select) - Set active cluster
- [cluster list](list.md) - List all configured clusters
