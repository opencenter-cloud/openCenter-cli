# `openCenter cluster validate` - Validate Cluster Configuration


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Arguments](#arguments)
- [Options](#options)
- [Validation Checks](#validation-checks)
- [Examples](#examples)
- [Output](#output)
- [Debug Configuration](#debug-configuration)
- [Exit Codes](#exit-codes)
- [Troubleshooting](#troubleshooting)
- [Notes](#notes)
- [See Also](#see-also)
## Synopsis
```bash
openCenter cluster validate [name] [OPTIONS]
```

## Description

Validate cluster configuration against schema and business rules. This command performs comprehensive validation including schema validation, required field validation, cross-field dependency validation, cloud provider credential validation, network configuration validation, and SOPS key validation.

If no cluster name is provided, validates the currently active cluster.

## Arguments

### `[name]`
- **Required/Optional**: Optional
- **Description**: Name of the cluster to validate (format: `cluster` or `organization/cluster`). If not provided, validates the currently active cluster
- **Example**: `my-cluster` or `production/my-cluster`

## Options

### `--generate-debug-config`
- **Description**: Generate complete openCenter.yaml config file for debugging purposes
- **Type**: Boolean
- **Default**: `false`

### `--output-dir <path>`
- **Description**: Directory to save debug config (defaults to GitOps directory or current directory)
- **Type**: String
- **Default**: GitOps directory or `.`

### `-h, --help`
- **Description**: Display help information for this subcommand

## Validation Checks

The command performs the following validation checks:

### Schema Validation
- Validates configuration against JSON schema
- Checks data types and formats
- Verifies enum values

### Required Field Validation
- Ensures all required fields are present
- Validates field values are not empty
- Checks for missing configuration sections

### Cross-Field Dependency Validation
- Validates relationships between configuration fields
- Checks conditional requirements
- Verifies logical consistency

### Cloud Provider Credential Validation
- Validates provider-specific credentials
- Checks authentication configuration
- Verifies API endpoint accessibility

### Network Configuration Validation
- Validates CIDR ranges
- Checks for network conflicts
- Verifies subnet configurations

### SOPS Key Validation
- Checks SOPS key file existence
- Validates key format
- Verifies key readability

## Examples

### Validate active cluster
```bash
openCenter cluster validate
```
Validates the currently active cluster configuration.

### Validate specific cluster
```bash
openCenter cluster validate my-cluster
```
Validates the specified cluster configuration.

### Validate cluster in organization
```bash
openCenter cluster validate production/prod-cluster
```
Validates a cluster within a specific organization.

### Validate and generate debug config
```bash
openCenter cluster validate my-cluster --generate-debug-config
```
Validates and generates a complete configuration file for debugging.

### Validate and save debug config to specific directory
```bash
openCenter cluster validate my-cluster --generate-debug-config --output-dir=/tmp
```
Saves the debug configuration to a specific directory.

### Validate with debug mode
```bash
OPENCENTER_DEBUG=1 openCenter cluster validate my-cluster
```
Enables debug mode and automatically generates debug config.

## Output

### Successful Validation

```
Validation successful.
```

### Validation Failures

```
validation error: kubernetes version must be specified
validation error: infrastructure provider must be one of: openstack, baremetal, kind, vmware
validation error: network CIDR 10.0.0.0/8 conflicts with service CIDR 10.96.0.0/12
validation failed
```

### With Debug Config Generation

```
Debug config saved to /home/user/.config/openCenter/clusters/myorg/gitops/.openCenter.yaml
Validation successful.
```

## Debug Configuration

The `--generate-debug-config` flag generates a complete `.openCenter.yaml` file that includes:

- All configuration values (including defaults)
- Resolved paths and references
- Computed values
- Template variables

This file is useful for:
- Debugging configuration issues
- Understanding default values
- Troubleshooting template rendering
- Verifying configuration resolution

## Exit Codes

- `0` - Validation successful
- `1` - Validation failed or error loading configuration

## Troubleshooting

### Kubernetes version not specified
**Error**: `validation error: kubernetes version must be specified`

**Solution**: Set the Kubernetes version in configuration:
```bash
openCenter cluster update my-cluster --opencenter.cluster.kubernetes.version=1.31.4
```

### Invalid provider
**Error**: `validation error: infrastructure provider must be one of: openstack, baremetal, kind, vmware`

**Solution**: Set a valid provider:
```bash
openCenter cluster update my-cluster --opencenter.infrastructure.provider=openstack
```

### Network CIDR conflicts
**Error**: `validation error: network CIDR 10.0.0.0/8 conflicts with service CIDR 10.96.0.0/12`

**Solution**: Adjust network CIDRs to avoid conflicts:
```bash
openCenter cluster update my-cluster \
  --opencenter.cluster.network.pod_cidr=10.244.0.0/16 \
  --opencenter.cluster.network.service_cidr=10.96.0.0/12
```

### SOPS key not found
**Error**: `validation error: SOPS key file not found: /path/to/key.txt`

**Solution**: Generate SOPS key or update path:
```bash
# Regenerate keys
openCenter cluster init my-cluster --force

# Or update path
openCenter cluster update my-cluster --secrets.sops_age_key_file=/correct/path/to/key.txt
```

### Missing required fields
**Error**: `validation error: field 'opencenter.meta.env' is required`

**Solution**: Provide required fields:
```bash
openCenter cluster update my-cluster --opencenter.meta.env=dev
```

## Notes

- Validation is performed automatically during `cluster init` with `--strict` flag
- Use `--generate-debug-config` to troubleshoot complex configuration issues
- Debug config includes all resolved values and defaults
- The `OPENCENTER_DEBUG` environment variable automatically enables debug config generation
- Validation checks are defined in the configuration schema
- Provider-specific validation is performed based on the infrastructure provider
- Network validation ensures no CIDR conflicts
- SOPS key validation checks file existence and readability
- Cross-field validation ensures logical consistency between related fields

## See Also

- `openCenter cluster init` - Initialize cluster with validation
- `openCenter cluster update` - Update cluster configuration
- `openCenter cluster info` - Show cluster information
- `openCenter cluster schema` - Export JSON schema
- `openCenter cluster preflight` - Run preflight checks
