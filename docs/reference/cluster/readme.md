---
title: Cluster Commands Reference
doc_type: reference
category: reference
weight: 30
---

# Cluster Commands Reference


## Table of Contents

- [Overview](#overview)
- [Command Index](#command-index)
- [Common Patterns](#common-patterns)
- [Global Flags](#global-flags)
- [Environment Variables](#environment-variables)
- [Exit Codes](#exit-codes)
- [Related Documentation](#related-documentation)
Complete reference for all `openCenter cluster` subcommands organized by lifecycle phase.

## Overview

Cluster commands manage Kubernetes cluster configurations throughout their lifecycle. Commands follow a standard workflow from initialization through deployment and ongoing management.

## Command Index

### Initialization Phase

Commands for creating and configuring new clusters.

#### [init](./init.md)

Initialize a new cluster configuration with default values.

**Usage**: `openCenter cluster init [name] [flags]`

**Key Features**:
- Schema-based default generation
- Organization-based directory structure
- Automatic SOPS and SSH key generation
- Configuration override via flags

**Common Flags**:
- `--org` - Organization name
- `--type` - Cluster type (openstack, aws, kind, baremetal)
- `--force` - Overwrite existing configuration
- `--no-keygen` - Skip key generation

#### [edit](./edit.md)

Edit cluster configuration in your preferred editor.

**Usage**: `openCenter cluster edit [name]`

**Behavior**:
- Opens configuration file in `$EDITOR` or `$VISUAL`
- Falls back to `vi` if neither is set
- Uses active cluster if no name provided

### Validation Phase

Commands for verifying configuration correctness.

#### [validate](./validate.md)

Validate cluster configuration against schema and business rules.

**Usage**: `openCenter cluster validate [name] [flags]`

**Validation Checks**:
- JSON schema validation
- Required field validation
- Cross-field dependencies
- Provider-specific requirements
- SOPS key availability

**Flags**:
- `--generate-debug-config` - Save complete configuration for debugging
- `--output-dir` - Directory for debug output

#### [preflight](./preflight.md)

Run preflight checks for tools and provider requirements.

**Usage**: `openCenter cluster preflight [name]`

**Checks**:
- Required tools (git, kubectl, talosctl)
- Provider-specific requirements
- Cloud provider connectivity
- Credential validation

### Setup Phase

Commands for generating GitOps repository structure.

#### [setup](./setup.md)

Set up GitOps repository structure for a cluster.

**Usage**: `openCenter cluster setup [name] [flags]`

**Operations**:
- Renders base GitOps structure
- Generates cluster-specific templates
- Provisions OpenTofu configuration
- Idempotent by default

**Flags**:
- `--force` - Overwrite existing repository

#### [render](./render.md)

Render templates into GitOps directory (always overwrites).

**Usage**: `openCenter cluster render [name]`

**Behavior**:
- Always renders templates (no skip logic)
- No Git operations
- Ideal for iterative development
- Overwrites existing files

### Deployment Phase

Commands for provisioning and bootstrapping clusters.

#### [bootstrap](./bootstrap.md)

Run provider-specific bootstrap actions for a cluster.

**Usage**: `openCenter cluster bootstrap [name] [flags]`

**Provider Support**:
- **OpenStack/AWS/GCP/Azure**: Terraform workflow (init, apply)
- **Kind**: Local cluster creation with container runtime

**Flags**:
- `--dry-run` - Show planned actions without executing
- `--container-runtime` - Runtime for Kind (docker, podman)
- `--restart` - Rerun all steps ignoring saved state
- `--step` - Run single step by ID
- `--from-step` - Restart from specific step
- `--log` - Custom log file path

**State Management**:
- Tracks completed steps in `bootstrap-state.json`
- Resumes from last successful step
- Timestamped logs in cluster directory

### Management Phase

Commands for ongoing cluster operations.

#### [list](./list.md)

List all configured clusters.

**Usage**: `openCenter cluster list [flags]`

**Aliases**: `ls`

**Output**:
- Plain text (one cluster per line)
- Active cluster marked with `*`
- JSON format with `--json` flag

#### [select](./select.md)

Select and activate a cluster for operations.

**Usage**: `openCenter cluster select [name] [flags]`

**Modes**:
- **Interactive**: Select from list
- **Direct**: Specify cluster name
- **Activate**: Set environment variables
- **Deactivate**: Clear active cluster

**Flags**:
- `--activate` - Set environment variables
- `--export-only` - Output export commands only
- `--clear` - Deactivate current cluster
- `--shell` - Override shell detection

**Environment Variables Set**:
- `OPENCENTER_ACTIVE_CLUSTER`
- `KUBECONFIG`
- `BIN` (cluster bin directory)

#### [current](./current.md)

Show the current active cluster.

**Usage**: `openCenter cluster current [flags]`

**Flags**:
- `--quiet` / `-q` - Output just the cluster name

#### [status](./status.md)

Show active cluster status and metadata.

**Usage**: `openCenter cluster status [flags]`

**Information Displayed**:
- Active cluster name
- Environment and region
- Organization
- Provider
- Cluster status
- Next steps based on status

**Flags**:
- `--paths` - Show file paths and their status
- `--quiet` / `-q` - Output just cluster name

#### [info](./info.md)

Show detailed configuration for a cluster.

**Usage**: `openCenter cluster info [name] [flags]`

**Output Formats**:
- Human-readable YAML (default)
- JSON with `--json` flag
- Export commands with `--export-only`

**Information Displayed**:
- Configuration file path
- GitOps directory
- Metadata
- Lock status

**Flags**:
- `--json` - JSON output
- `--validate` - Validate configuration
- `--export-only` - Output export commands
- `--shell` - Override shell detection

#### [update](./update.md)

Update fields in existing cluster configuration.

**Usage**: `openCenter cluster update [name] [flags]`

**Behavior**:
- Uses dot notation for field paths
- Supports dynamic flags from schema
- Updates active cluster if no name provided

**Examples**:
```bash
# Update IAC fields
openCenter cluster update --iac.main.master_count=5

# Update with validation
openCenter cluster update my-cluster --iac.main.worker_count=3 --strict
```

**Flags**:
- `--strict` - Validate after update

### Service Management

Commands for managing cluster services.

#### [service](./service.md)

Manage cluster services (enable, disable, configure).

**Usage**: `openCenter cluster service <subcommand>`

**Subcommands**:
- `list` - List available services
- `enable` - Enable a service
- `disable` - Disable a service
- `status` - Show service status

### Credentials Management

Commands for managing cluster credentials.

#### [credentials](./credentials.md)

Manage cluster credentials and secrets.

**Usage**: `openCenter cluster credentials <subcommand>`

**Subcommands**:
- `export` - Export credentials to environment
- `unset` - Clear exported credentials

### Advanced Operations

Commands for advanced cluster operations.

#### [drift](./drift.md)

Detect configuration drift between declared and actual state.

**Usage**: `openCenter cluster drift [name]`

**Checks**:
- GitOps repository changes
- Infrastructure state differences
- Configuration file modifications

#### [backup](./backup.md)

Backup cluster configuration and state.

**Usage**: `openCenter cluster backup [name]`

**Backup Contents**:
- Cluster configuration file
- SOPS keys
- SSH keys
- GitOps repository state

#### [destroy](./destroy.md)

Destroy a cluster and remove all associated resources.

**Usage**: `openCenter cluster destroy <name> [flags]`

**Operations**:
- Removes GitOps directory
- Deletes cluster configuration
- Removes organization directories
- Clears active cluster marker

**Flags**:
- `--force` - Skip confirmation prompt

**Warning**: This operation is permanent and cannot be undone.

#### [schema](./schema.md)

Export cluster JSON schema with validation rules.

**Usage**: `openCenter cluster schema [flags]`

**Flags**:
- `--out` - Output file path (default stdout)
- `--pretty` - Pretty print JSON (default true)
- `--version` - Show schema version

**Note**: Hidden command for internal/development use.

## Common Patterns

### Standard Workflow

```bash
# 1. Initialize cluster
openCenter cluster init my-cluster --org myorg

# 2. Validate configuration
openCenter cluster validate my-cluster

# 3. Run preflight checks
openCenter cluster preflight my-cluster

# 4. Set up GitOps repository
openCenter cluster setup my-cluster

# 5. Bootstrap cluster
openCenter cluster bootstrap my-cluster

# 6. Activate cluster environment
eval $(openCenter cluster select my-cluster --activate --export-only)
```

### Development Workflow

```bash
# Initialize with Kind
openCenter cluster init dev-cluster --type kind

# Iterate on configuration
openCenter cluster edit dev-cluster
openCenter cluster render dev-cluster  # Fast template rendering

# Bootstrap local cluster
openCenter cluster bootstrap dev-cluster --container-runtime=podman
```

### Multi-Organization Management

```bash
# List all clusters
openCenter cluster list

# Select cluster in specific organization
openCenter cluster select myorg/production-cluster --activate

# Check current cluster
openCenter cluster current

# Show detailed status
openCenter cluster status --paths
```

## Global Flags

All cluster commands support these global flags:

- `--config-dir` - Override configuration directory
- `--help` / `-h` - Show command help

## Environment Variables

Cluster commands respect these environment variables:

- `OPENCENTER_CONFIG_DIR` - Configuration directory location
- `OPENCENTER_DEBUG` - Enable debug mode
- `OPENCENTER_ACTIVE_CLUSTER` - Current active cluster
- `CONTAINER_RUNTIME` - Container runtime for Kind (docker, podman)

See [Environment Variables Reference](../environment-variables.md) for complete list.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Configuration validation failed |
| 3 | Lock acquisition failed |
| 4 | Provider-specific error |

## Related Documentation

- [Configuration Reference](../configuration.md)
- [Environment Variables](../environment-variables.md)
- [Error Codes](../error-codes.md)
- [Shell Integration](../shell-integration.md)
