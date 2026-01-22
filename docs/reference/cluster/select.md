# `openCenter cluster select` - Select the Active Cluster


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Arguments](#arguments)
- [Options](#options)
- [Examples](#examples)
- [Output](#output)
- [Environment Setup](#environment-setup)
- [Notes](#notes)
- [See Also](#see-also)
## Synopsis
```bash
openCenter cluster select [name] [OPTIONS]
```

## Description

Select the active cluster and display comprehensive information including metadata, GitOps paths, cluster-specific paths, and environment setup commands. If no cluster name is provided, an interactive selection menu is displayed.

For deployed clusters, the command generates environment setup commands to configure KUBECONFIG, ANSIBLE_INVENTORY, virtual environment, and PATH variables.

## Arguments

### `[name]`
- **Required/Optional**: Optional
- **Description**: Name of the cluster to select (format: `cluster` or `organization/cluster`). If not provided, displays an interactive selection menu
- **Example**: `my-cluster` or `production/my-cluster`

## Options

### `--export-only`
- **Description**: Only output export commands for shell evaluation (useful for `eval` command)
- **Type**: Boolean
- **Default**: `false`

### `-h, --help`
- **Description**: Display help information for this subcommand

## Examples

### Basic usage
```bash
openCenter cluster select my-cluster
```
Sets the active cluster and displays comprehensive information.

### Interactive selection
```bash
openCenter cluster select
```
Displays an interactive menu to select from available clusters.

### Select cluster in organization
```bash
openCenter cluster select production/prod-cluster
```
Selects a cluster within a specific organization.

### Export-only mode for shell evaluation
```bash
eval "$(openCenter cluster select my-cluster --export-only)"
```
Configures shell environment with cluster-specific variables.

### Configure environment in one command
```bash
eval "$(openCenter cluster select production/prod-cluster --export-only)"
```
Sets up environment for a deployed cluster in the production organization.

## Output

### Full Output (Default)

```
Active cluster set to production/prod-cluster

Cluster Information:
  Name:         prod-cluster
  Environment:  prod
  Region:       us-east-1
  Status:       deployed
  Organization: production

GitOps Repository:
  GitOps Directory:      /home/user/.config/openCenter/clusters/production/gitops
  Applications Directory: /home/user/.config/openCenter/clusters/production/gitops/applications/overlays/prod-cluster
  Infrastructure Directory: /home/user/.config/openCenter/clusters/production/gitops/infrastructure/clusters/prod-cluster
  Secrets Directory:     /home/user/.config/openCenter/clusters/production/secrets

Cluster Paths:
  Cluster Directory:     /home/user/.config/openCenter/clusters/production/prod-cluster
  SOPS Key Path:         /home/user/.config/openCenter/clusters/production/secrets/age/keys/prod-cluster.txt
  SOPS Config Path:      /home/user/.config/openCenter/clusters/production/.sops.yaml

Environment Setup Commands:
  export KUBECONFIG=/home/user/.config/openCenter/clusters/production/prod-cluster/kubeconfig.yaml
  export ANSIBLE_INVENTORY=/home/user/.config/openCenter/clusters/production/prod-cluster/inventory
  source /home/user/.config/openCenter/clusters/production/prod-cluster/.venv/bin/activate
  export PATH=/home/user/.config/openCenter/clusters/production/prod-cluster/.bin:$PATH

To configure your shell environment, run:
  eval "$(openCenter cluster select prod-cluster)"
```

### Export-Only Output (--export-only)

```bash
export KUBECONFIG=/home/user/.config/openCenter/clusters/production/prod-cluster/kubeconfig.yaml
export ANSIBLE_INVENTORY=/home/user/.config/openCenter/clusters/production/prod-cluster/inventory
source /home/user/.config/openCenter/clusters/production/prod-cluster/.venv/bin/activate
export PATH=/home/user/.config/openCenter/clusters/production/prod-cluster/.bin:$PATH
```

### Interactive Selection Menu

```
Select a cluster

> production/prod-cluster
  Organization: production

  production/staging-cluster
  Organization: production

  development/dev-cluster
  Organization: development

  my-cluster
```

Navigation:
- Use arrow keys (↑/↓) to navigate
- Press Enter to select
- Press 'q' or Ctrl+C to quit

## Environment Setup

For deployed clusters (status: `deployed`), the following environment variables are configured:

### KUBECONFIG
Path to the cluster's kubeconfig file for kubectl access:
```bash
export KUBECONFIG=/path/to/cluster/kubeconfig.yaml
```

### ANSIBLE_INVENTORY
Path to the Ansible inventory file for cluster management:
```bash
export ANSIBLE_INVENTORY=/path/to/cluster/inventory
```

### Virtual Environment
Activates the cluster-specific Python virtual environment:
```bash
source /path/to/cluster/.venv/bin/activate
```

### PATH
Adds cluster-specific binaries to PATH:
```bash
export PATH=/path/to/cluster/.bin:$PATH
```

## Notes

- The active cluster is stored in `~/.config/openCenter/.active`
- Interactive mode uses Bubble Tea for terminal UI
- Organization information is displayed in the interactive menu
- Environment setup commands are only generated for deployed clusters
- Use `--export-only` with `eval` for quick environment configuration
- The command validates that the cluster exists before setting it as active
- Cluster metadata is loaded from the configuration file
- GitOps paths are resolved based on organization structure

## See Also

- `openCenter cluster current` - Show current active cluster
- `openCenter cluster list` - List all clusters
- `openCenter cluster info` - Show detailed cluster information
- `openCenter cluster init` - Initialize a new cluster
