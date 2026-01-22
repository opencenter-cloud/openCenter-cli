# `openCenter cluster update` - Update Cluster Configuration Fields


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Arguments](#arguments)
- [Options](#options)
- [Configuration Paths](#configuration-paths)
- [Examples](#examples)
- [Output](#output)
- [Available IAC Keys](#available-iac-keys)
- [Value Types](#value-types)
- [Notes](#notes)
- [Workflow](#workflow)
- [See Also](#see-also)
## Synopsis
```bash
openCenter cluster update [name] [OPTIONS]
```

## Description

Update specific fields in an existing cluster configuration using dynamic dotted flags. This command allows you to modify configuration values without manually editing the YAML file.

If no cluster name is provided, the currently active cluster is updated.

## Arguments

### `[name]`
- **Required/Optional**: Optional
- **Description**: Name of the cluster to update (format: `cluster` or `organization/cluster`). If not provided, updates the currently active cluster
- **Example**: `my-cluster` or `production/my-cluster`

## Options

### `--strict`
- **Description**: Fail if the resulting configuration is not valid
- **Type**: Boolean
- **Default**: `false`

### `--<config.path>=<value>`
- **Description**: Update any configuration value using dot notation
- **Type**: Dynamic (string, int, bool based on schema)
- **Example**: `--iac.main.master_count=5`

### `-h, --help`
- **Description**: Display help information and available IAC configuration keys

### `-h, --help`
- **Description**: Display help information for this subcommand

## Configuration Paths

### IAC Configuration

The command supports updating IAC (Infrastructure as Code) configuration fields:

```bash
--iac.main.master_count=<value>
--iac.main.worker_count=<value>
--iac.main.kubernetes_version=<value>
--iac.network.pod_cidr=<value>
--iac.network.service_cidr=<value>
--iac.storage.volume_size=<value>
```

### OpenCenter Configuration

Update core openCenter configuration:

```bash
--opencenter.meta.env=<value>
--opencenter.meta.region=<value>
--opencenter.meta.status=<value>
--opencenter.cluster.kubernetes.version=<value>
--opencenter.cluster.cluster_name=<value>
--opencenter.infrastructure.provider=<value>
```

### Secrets Configuration

Update secrets-related configuration:

```bash
--secrets.sops_age_key_file=<path>
--secrets.ssh_key.private=<path>
--secrets.ssh_key.public=<path>
--secrets.ssh_key.cypher=<value>
```

## Examples

### Show available configuration keys
```bash
openCenter cluster update --help
```
Displays all available IAC configuration keys.

### Update master node count
```bash
openCenter cluster update --iac.main.master_count=5
```
Updates the master node count for the active cluster.

### Update specific cluster
```bash
openCenter cluster update my-cluster --iac.main.worker_count=3
```
Updates worker node count for a specific cluster.

### Update Kubernetes version
```bash
openCenter cluster update --iac.main.kubernetes_version=1.30.4
```
Updates the Kubernetes version.

### Update multiple fields
```bash
openCenter cluster update my-cluster \
  --iac.main.master_count=3 \
  --iac.main.worker_count=5 \
  --iac.main.kubernetes_version=1.31.4
```
Updates multiple configuration fields at once.

### Update with strict validation
```bash
openCenter cluster update my-cluster \
  --iac.main.master_count=3 \
  --strict
```
Updates configuration and validates the result.

### Update environment
```bash
openCenter cluster update my-cluster --opencenter.meta.env=prod
```
Changes the cluster environment designation.

### Update provider
```bash
openCenter cluster update my-cluster --opencenter.infrastructure.provider=aws
```
Changes the infrastructure provider.

### Update organization cluster
```bash
openCenter cluster update production/prod-cluster --iac.main.worker_count=10
```
Updates a cluster within a specific organization.

## Output

```
Updated cluster configuration my-cluster
```

With `--strict` validation:
```
Updated cluster configuration my-cluster
```

Or if validation fails:
```
validation error: master_count must be an odd number for HA
validation error: worker_count must be at least 1
validation failed
```

## Available IAC Keys

Run `openCenter cluster update --help` to see all available IAC configuration keys. The command dynamically generates the list from the JSON schema, showing only keys that are specific to IAC configuration (excluding those that overlap with opencenter configuration).

Example output:
```
Available IAC Configuration Keys:
  Use any of the following keys with --<key>=<value> format:

  --iac.main.master_count=<value>
  --iac.main.worker_count=<value>
  --iac.main.kubernetes_version=<value>
  --iac.network.pod_cidr=<value>
  --iac.network.service_cidr=<value>
  --iac.storage.volume_size=<value>
  ...
```

## Value Types

The command automatically converts values to the appropriate type:

### String Values
```bash
--opencenter.meta.env=prod
--opencenter.cluster.cluster_name=my-k8s-cluster
```

### Integer Values
```bash
--iac.main.master_count=3
--iac.main.worker_count=5
--iac.storage.volume_size=100
```

### Boolean Values
```bash
--iac.features.enable_monitoring=true
--iac.features.enable_logging=false
```

## Notes

- The command updates the configuration file in place
- No validation is performed unless `--strict` flag is used
- Use dot notation to specify nested configuration paths
- Values are automatically converted to the appropriate type
- The command supports both IAC and OpenCenter configuration paths
- Changes are saved immediately to the configuration file
- Use `cluster validate` after updates to verify configuration
- The command does not re-render GitOps templates automatically
- Run `cluster setup --render` to apply configuration changes to GitOps

## Workflow

Typical workflow for updating configuration:

```bash
# 1. Update configuration
openCenter cluster update my-cluster --iac.main.worker_count=5

# 2. Validate changes
openCenter cluster validate my-cluster

# 3. Re-render GitOps templates
openCenter cluster setup my-cluster --render

# 4. Review changes
cd ~/.config/openCenter/clusters/org/gitops
git diff

# 5. Commit and apply
git add .
git commit -m "Update worker count to 5"
git push
```

## See Also

- `openCenter cluster validate` - Validate configuration after updates
- `openCenter cluster edit` - Edit configuration file directly
- `openCenter cluster info` - Show current configuration
- `openCenter cluster setup` - Re-render GitOps templates
