# cluster init


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Arguments](#arguments)
- [Flags](#flags)
- [Examples](#examples)
- [Directory Structure](#directory-structure)
- [Key Generation](#key-generation)
- [Git Repository Initialization](#git-repository-initialization)
- [Configuration Override](#configuration-override)
- [Server Pool Configuration](#server-pool-configuration)
- [Output](#output)
- [Error Handling](#error-handling)
- [See Also](#see-also)
**doc_type:** reference

Initialize a new cluster configuration with default values.

## Synopsis

```bash
openCenter cluster init [name] [flags]
```

## Description

The `cluster init` command creates a new cluster configuration file with sensible defaults based on the JSON schema. Configuration values can be overridden using command-line flags with dot notation.

The configuration is created in an organization-based directory structure by default:
```
~/.config/openCenter/clusters/<organization>/<cluster>/
```

SOPS Age encryption keys and SSH key pairs are automatically generated unless `--no-keygen` is specified.

## Arguments

- `name` - Cluster name (optional if using `--config` flag or active cluster is set)

## Flags

### Configuration Source
- `--config string` - Load configuration from existing file (cluster name extracted from file)
- `--config-dir string` - Use legacy flat structure (creates `<dir>/<name>.yaml`)

### Organization
- `--org string` - Organization name (default: "opencenter")
- `--opencenter.meta.organization=<value>` - Alternative way to set organization

### Cluster Type
- `--type string` - Cluster type/provider (openstack, aws, kind, baremetal)

### Key Generation
- `--no-keygen` - Skip automatic SOPS and SSH key generation
- `--no-sops-keygen` - Skip automatic SOPS key generation only
- `--regenerate-keys` - Regenerate keys even if they already exist

### Validation
- `--strict` - Enable strict validation (fail if required values are missing)
- `--force` - Overwrite existing configuration

### Advanced
- `--server-pool stringArray` - Additional server pool configuration
- `--full-schema` - Generate configuration with all available fields including examples
- `--opencenter.<path>=<value>` - Override any configuration value using dot notation

## Examples

### Basic Initialization

```bash
# Initialize with defaults (uses "opencenter" as organization)
openCenter cluster init my-cluster

# Initialize with organization using --org flag
openCenter cluster init my-cluster --org myorg

# Initialize with organization using dot notation
openCenter cluster init my-cluster --opencenter.meta.organization=myorg
```

### From Configuration File

```bash
# Initialize from existing config file (cluster name extracted from config)
openCenter cluster init --config my-cluster-config.yaml

# Initialize from config file with explicit name (overrides config file name)
openCenter cluster init my-cluster --config template-config.yaml
```

### Cluster Types

```bash
# Initialize bare metal cluster
openCenter cluster init my-cluster --org myorg --type baremetal

# Initialize OpenStack cluster (default)
openCenter cluster init my-cluster --type openstack

# Initialize AWS cluster
openCenter cluster init my-cluster --type aws

# Initialize Kind cluster (local development)
openCenter cluster init my-cluster --type kind
```

### Custom Configuration

```bash
# Initialize with custom values
openCenter cluster init my-cluster \
  --org production \
  --opencenter.meta.env=prod \
  --opencenter.cluster.kubernetes.version=1.31.4 \
  --opencenter.infrastructure.provider=aws

# Initialize with full schema (all fields with examples)
openCenter cluster init my-cluster --full-schema
```

### Key Management

```bash
# Initialize without key generation (SOPS and SSH)
openCenter cluster init my-cluster --no-keygen

# Regenerate keys even if they already exist
openCenter cluster init my-cluster --regenerate-keys

# Skip only SOPS key generation
openCenter cluster init my-cluster --no-sops-keygen
```

### Force Overwrite

```bash
# Force overwrite existing configuration
openCenter cluster init my-cluster --force

# Force overwrite active cluster configuration
openCenter cluster init --force
```

### Strict Validation

```bash
# Initialize with strict validation
openCenter cluster init my-cluster --strict
```

### Additional Server Pools

```bash
# Add additional worker pools
openCenter cluster init my-cluster \
  --server-pool="name=pool1,worker_count=3,flavor_worker=m1.large,node_worker=worker-pool1" \
  --server-pool="name=pool2,worker_count=5,flavor_worker=m1.xlarge,node_worker=worker-pool2"
```

## Directory Structure

### Organization-Based (Default)

```
~/.config/openCenter/clusters/<organization>/
├── .<cluster>-config.yaml          # Cluster configuration
├── infrastructure/
│   └── clusters/<cluster>/         # Cluster-specific infrastructure
├── applications/
│   └── overlays/<cluster>/         # Cluster-specific applications
└── secrets/
    ├── age/<cluster>-key.txt       # SOPS Age encryption key
    └── ssh/<cluster>-<env>-<region> # SSH key pair
```

### Legacy Flat (with --config-dir)

```
<config-dir>/
└── <cluster>.yaml                  # Cluster configuration
```

## Key Generation

### SOPS Age Key

Generated at: `clusters/<org>/secrets/age/<cluster>-key.txt`

Format:
```
# created: 2026-01-18T14:30:00Z
# public key: age1...
AGE-SECRET-KEY-1...
```

Public key is automatically added to `.sops.yaml` configuration.

### SSH Key Pair

Generated at: `clusters/<org>/secrets/ssh/<cluster>-<env>-<region>`

- Private key: `<cluster>-<env>-<region>`
- Public key: `<cluster>-<env>-<region>.pub`
- Default cipher: `ed25519`
- Comment format: `<organization>-<cluster>-<region>`

Permissions:
- Private key: `0600`
- Public key: `0644`

## Git Repository Initialization

A git repository is automatically initialized in the GitOps directory with:

- Initial branch: `main`
- `.gitignore` file with common exclusions
- Pre-commit hook for SOPS encryption validation

### Pre-Commit Hook

The pre-commit hook validates that cluster configuration files containing sensitive data are properly encrypted with SOPS before being committed.

Features:
- Detects sensitive data patterns (passwords, secrets, tokens, keys)
- Verifies SOPS encryption
- Blocks commits with unencrypted secrets
- Provides helpful error messages

## Configuration Override

Use dot notation to override any configuration value:

```bash
# Set environment
--opencenter.meta.env=prod

# Set Kubernetes version
--opencenter.cluster.kubernetes.version=1.31.4

# Set infrastructure provider
--opencenter.infrastructure.provider=aws

# Set GitOps directory
--opencenter.gitops.git_dir=/path/to/gitops

# Set SSH key cipher
--secrets.ssh_key.cypher=rsa
```

## Server Pool Configuration

Server pool format: `key=value,key=value,...`

Required fields:
- `name` - Pool name
- `worker_count` - Number of workers
- `flavor_worker` - Instance flavor
- `node_worker` - Node identifier

Optional fields:
- `server_group_affinity` - Server group affinity policy
- `image_id` - Image ID
- `image_name` - Image name
- `worker_node_bfv_volume_size` - Boot from volume size
- `worker_node_bfv_destination_type` - BFV destination type
- `worker_node_bfv_source_type` - BFV source type
- `worker_node_bfv_volume_type` - BFV volume type
- `worker_node_bfv_delete_on_termination` - Delete BFV on termination
- `pf9_onboard` - PF9 onboarding flag
- `subnet_id` - Subnet ID

Example:
```bash
--server-pool="name=gpu-pool,worker_count=2,flavor_worker=g1.large,node_worker=gpu-worker,image_name=ubuntu-22.04-gpu"
```

## Output

Successful initialization displays:

```
Generated ed25519 SSH key pair at /path/to/ssh/my-cluster-prod-us-east-1
Initialized git repository at /path/to/gitops/myorg
Installed SOPS pre-commit hook at /path/to/gitops/myorg/.git/hooks/pre-commit
Created cluster configuration in organization 'myorg' at '/path/to/clusters/myorg/infrastructure/clusters/my-cluster'
GitOps repository root: /path/to/gitops/myorg
SOPS key location: /path/to/clusters/myorg/secrets/age/my-cluster-key.txt
```

## Error Handling

**Cluster already exists:**
```
Error: cluster configuration directory 'my-cluster' already exists in organization 'myorg', use --force to overwrite
```

**Invalid cluster name:**
```
Error: invalid cluster name 'my-cluster!@#': cluster name must contain only alphanumeric characters, hyphens, underscores, and forward slashes
```

**Invalid organization name:**
```
Error: invalid organization name 'my-org!@#': organization name must contain only alphanumeric characters, hyphens, and underscores
```

**Missing required fields (strict mode):**
```
Error: opencenter.cluster.kubernetes.version: required field missing
validation failed
```

## See Also

- [cluster validate](../cli-commands.md#cluster-validate) - Validate cluster configuration
- [cluster setup](setup.md) - Setup GitOps directory structure
- [cluster list](list.md) - List all configured clusters
- [cluster edit](edit.md) - Edit cluster configuration
