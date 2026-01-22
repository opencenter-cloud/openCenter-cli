# cluster bootstrap


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Arguments](#arguments)
- [Flags](#flags)
- [Examples](#examples)
- [Provider-Specific Behavior](#provider-specific-behavior)
- [State Management](#state-management)
- [Logging](#logging)
- [Long-Running Operations](#long-running-operations)
- [Locking](#locking)
- [Exit Codes](#exit-codes)
- [Status Updates](#status-updates)
- [Environment Variables](#environment-variables)
- [See Also](#see-also)
**doc_type:** reference

Run provider-specific bootstrap actions to deploy cluster infrastructure.

## Synopsis

```bash
openCenter cluster bootstrap [name] [flags]
```

## Description

The `cluster bootstrap` command executes provider-specific deployment actions to provision cluster infrastructure. It runs Terraform/OpenTofu for cloud providers or creates Kind clusters for local development.

Bootstrap operations are stateful and resumable. The command tracks completed steps and can restart from a specific step if interrupted.

## Arguments

- `name` - Cluster name (optional if active cluster is set)

## Flags

- `--dry-run` - Show planned actions without executing
- `--kubeconfig string` - Path to kubeconfig (default: "./kubeconfig.yaml")
- `--log string` - Log file path (default: `<git_dir>/infrastructure/clusters/<name>/logs/bootstrap-YYYY-MM-DD-TIMESTAMP.log`)
- `--container-runtime string` - Container runtime for Kind clusters (docker or podman)
- `--restart` - Rerun all bootstrap steps and ignore saved state
- `--step string` - Run a single bootstrap step by ID
- `--from-step string` - Restart bootstrap from the specified step ID

## Examples

```bash
# Bootstrap active cluster
openCenter cluster bootstrap

# Bootstrap specific cluster
openCenter cluster bootstrap my-cluster

# Bootstrap with specific kubeconfig
openCenter cluster bootstrap my-cluster --kubeconfig=/path/to/kubeconfig

# Bootstrap Kind cluster with Podman
openCenter cluster bootstrap kind-cluster --container-runtime=podman

# Dry-run mode
openCenter cluster bootstrap my-cluster --dry-run

# Restart from a specific step
openCenter cluster bootstrap my-cluster --from-step=terraform-apply

# Run only a specific step
openCenter cluster bootstrap my-cluster --step=terraform-init

# Restart all steps (ignore saved state)
openCenter cluster bootstrap my-cluster --restart
```

## Provider-Specific Behavior

### OpenStack, AWS, GCP, Azure

Bootstrap steps:
1. `make-terraform` - Run make terraform in cluster directory
2. `terraform-init` - Initialize Terraform
3. `terraform-apply` - Apply Terraform configuration

Requirements:
- GitOps directory must be configured (`gitops.git_dir`)
- Cluster infrastructure directory must exist
- Cloud provider credentials must be configured

### Kind (Local Development)

Bootstrap steps:
1. `kind-create` - Create Kind cluster with custom configuration
2. `kind-export-kubeconfig` - Export kubeconfig for cluster access

Configuration:
- Disables default CNI for custom network plugin installation
- Pod subnet: `10.244.0.0/16`
- Service subnet: `10.96.0.0/12`
- Creates 1 control-plane node and 3 worker nodes

Container runtime resolution (in order):
1. `--container-runtime` flag
2. `CONTAINER_RUNTIME` environment variable
3. `KIND_EXPERIMENTAL_PROVIDER` environment variable
4. Default: `docker`

## State Management

Bootstrap state is saved to: `<git_dir>/infrastructure/clusters/<name>/logs/bootstrap-state.json`

State tracking enables:
- Resuming interrupted bootstrap operations
- Skipping already-completed steps
- Restarting from a specific step
- Running individual steps for debugging

State file format:
```json
{
  "version": 1,
  "steps": {
    "step-id": {
      "status": "success|failed|running|skipped",
      "updated_at": "2026-01-18T14:30:00Z",
      "error": "error message if failed"
    }
  }
}
```

## Logging

Bootstrap operations are logged to timestamped files:
- Default location: `<git_dir>/infrastructure/clusters/<name>/logs/bootstrap-YYYY-MM-DD-TIMESTAMP.log`
- Override with `--log` flag
- Includes command output, timestamps, and progress indicators
- Credentials are automatically masked in logs

Log format:
```
# openCenter bootstrap log
# time: 2026-01-18T14:30:00Z
# cluster: my-cluster
# dir: /path/to/gitops/infrastructure/clusters/my-cluster

$ terraform init
...
```

## Long-Running Operations

For long-running commands (e.g., `terraform apply`):
- Progress updates every 30 seconds
- Elapsed time tracking
- Completion time reporting
- Output streaming to console and log file

## Locking

Bootstrap operations acquire an exclusive lock on the cluster to prevent concurrent modifications. Lock duration: 1 hour.

If another operation is in progress:
```
Error: failed to acquire lock for cluster "my-cluster": lock already held
Another operation may be in progress. Wait for it to complete or use 'openCenter cluster info my-cluster' to check lock status
```

## Exit Codes

- `0` - Bootstrap completed successfully
- `1` - Bootstrap failed (check logs for details)
- `2` - Lock acquisition failed (another operation in progress)

## Status Updates

On successful completion, cluster status is updated to:
- Stage: `bootstrap`
- Status: `success`

## Environment Variables

- `CONTAINER_RUNTIME` - Container runtime for Kind clusters (docker or podman)
- `KIND_EXPERIMENTAL_PROVIDER` - Alternative to CONTAINER_RUNTIME for Kind
- `KUBECONFIG` - Default kubeconfig path (overridden by `--kubeconfig` flag)

## See Also

- [cluster setup](setup.md) - Setup GitOps directory structure
- [cluster preflight](preflight.md) - Run preflight checks
- [cluster destroy](destroy.md) - Destroy cluster infrastructure
- [cluster status](status.md) - Show cluster status
