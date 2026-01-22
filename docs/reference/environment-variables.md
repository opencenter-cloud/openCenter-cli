---
title: Environment Variables Reference
doc_type: reference
category: reference
weight: 50
---

# Environment Variables Reference


## Table of Contents

- [Core Configuration Variables](#core-configuration-variables)
- [Debug and Development Variables](#debug-and-development-variables)
- [Cluster Selection Variables](#cluster-selection-variables)
- [Container Runtime Variables](#container-runtime-variables)
- [Kubernetes Configuration Variables](#kubernetes-configuration-variables)
- [Cloud Provider Variables](#cloud-provider-variables)
- [Feature Flag Variables](#feature-flag-variables)
- [Variable Precedence](#variable-precedence)
- [Security Considerations](#security-considerations)
- [Testing and CI/CD](#testing-and-cicd)
- [Related Documentation](#related-documentation)
Complete reference for all environment variables used by openCenter CLI.

## Core Configuration Variables

### OPENCENTER_CONFIG_DIR

**Description**: Override the default configuration directory location.

**Default**: `~/.config/openCenter`

**Valid Values**: Any valid directory path

**Usage**:
```bash
# Use custom config directory
export OPENCENTER_CONFIG_DIR=/path/to/custom/config
openCenter cluster list

# Temporary override for single command
OPENCENTER_CONFIG_DIR=./testdata/config openCenter cluster init test-cluster
```

**Notes**:
- Takes precedence over `--config-dir` flag when both are set
- Useful for testing and CI/CD environments
- Directory will be created if it doesn't exist

### OPENCENTER_CLUSTERS_DIR

**Description**: Override the default clusters directory location.

**Default**: `~/.config/openCenter/clusters`

**Valid Values**: Any valid directory path

**Usage**:
```bash
# Use custom clusters directory
export OPENCENTER_CLUSTERS_DIR=/path/to/clusters
openCenter cluster list
```

**Notes**:
- Used for organization-based cluster storage
- Affects cluster path resolution

### OPENCENTER_PLUGINS_DIR

**Description**: Override the default plugins directory location.

**Default**: `<config-dir>/plugins`

**Valid Values**: Any valid directory path

**Usage**:
```bash
# Use custom plugins directory
export OPENCENTER_PLUGINS_DIR=./testdata/plugins
openCenter plugins list
```

**Notes**:
- Plugins are discovered in this order:
  1. `OPENCENTER_PLUGINS_DIR` (if set)
  2. `<config-dir>/plugins`
  3. `PATH` environment variable

## Debug and Development Variables

### OPENCENTER_DEBUG

**Description**: Enable debug mode for troubleshooting and development.

**Default**: Not set (debug disabled)

**Valid Values**: `true`, `1`, `yes` (any truthy value)

**Usage**:
```bash
# Enable debug mode
export OPENCENTER_DEBUG=true
openCenter cluster validate my-cluster

# Enable for single command
OPENCENTER_DEBUG=true openCenter cluster setup my-cluster
```

**Effects**:
- Enables verbose logging
- Automatically generates debug configuration files
- Creates additional artifacts for troubleshooting
- Shows detailed validation output

**Related Commands**:
- `cluster validate --generate-debug-config`
- `sops validate`

## Cluster Selection Variables

### OPENCENTER_ACTIVE_CLUSTER

**Description**: Indicates the currently active cluster (set by shell integration).

**Default**: Not set

**Valid Values**: Any valid cluster identifier (e.g., `org/cluster` or `cluster`)

**Usage**:
```bash
# Automatically set by cluster select
eval $(openCenter cluster select my-cluster --activate --export-only)
echo $OPENCENTER_ACTIVE_CLUSTER

# Check active cluster in scripts
if [ -n "$OPENCENTER_ACTIVE_CLUSTER" ]; then
  echo "Active cluster: $OPENCENTER_ACTIVE_CLUSTER"
fi
```

**Notes**:
- Set automatically by `cluster select --activate`
- Updated by shell integration on each prompt
- Used by shell prompts to display active cluster
- Read-only (managed by openCenter)

## Container Runtime Variables

### CONTAINER_RUNTIME

**Description**: Specify container runtime for Kind clusters.

**Default**: `docker`

**Valid Values**: `docker`, `podman`

**Usage**:
```bash
# Use Podman for Kind clusters
export CONTAINER_RUNTIME=podman
openCenter cluster bootstrap my-kind-cluster

# Use Docker explicitly
export CONTAINER_RUNTIME=docker
openCenter cluster bootstrap my-kind-cluster
```

**Notes**:
- Only affects Kind provider
- Overridden by `--container-runtime` flag
- Podman support requires Kind experimental provider

### KIND_EXPERIMENTAL_PROVIDER

**Description**: Enable experimental provider support in Kind (alternative to CONTAINER_RUNTIME).

**Default**: Not set

**Valid Values**: `podman`, `docker`

**Usage**:
```bash
# Enable Podman support in Kind
export KIND_EXPERIMENTAL_PROVIDER=podman
openCenter cluster bootstrap my-kind-cluster
```

**Notes**:
- Native Kind environment variable
- `CONTAINER_RUNTIME` is preferred in openCenter
- Both variables are checked during bootstrap

## Kubernetes Configuration Variables

### KUBECONFIG

**Description**: Path to Kubernetes configuration file.

**Default**: `~/.kube/config`

**Valid Values**: Any valid file path

**Usage**:
```bash
# Use custom kubeconfig
export KUBECONFIG=/path/to/kubeconfig
openCenter cluster bootstrap my-cluster

# Set by cluster activation
eval $(openCenter cluster select my-cluster --activate --export-only)
echo $KUBECONFIG
```

**Notes**:
- Standard Kubernetes environment variable
- Set automatically by `cluster select --activate`
- Used by kubectl and other Kubernetes tools
- Can specify multiple files separated by `:`

## Cloud Provider Variables

### AWS_ACCESS_KEY_ID

**Description**: AWS access key for authentication.

**Default**: Not set

**Valid Values**: Valid AWS access key (20 characters)

**Usage**:
```bash
# Set AWS credentials
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# Run preflight checks
openCenter cluster preflight my-aws-cluster
```

**Notes**:
- Required for AWS provider
- Used for Route53 DNS validation in cert-manager
- Used for S3 backend in OpenTofu
- Credentials are masked in logs

### AWS_SECRET_ACCESS_KEY

**Description**: AWS secret access key for authentication.

**Default**: Not set

**Valid Values**: Valid AWS secret key (40 characters)

**Usage**: See `AWS_ACCESS_KEY_ID` above

**Notes**:
- Must be used with `AWS_ACCESS_KEY_ID`
- Credentials are masked in logs
- Never logged in plaintext

### OPENSTACK_* Variables

**Description**: OpenStack authentication variables (standard OpenStack RC file format).

**Common Variables**:
- `OS_AUTH_URL`: OpenStack authentication endpoint
- `OS_USERNAME`: OpenStack username
- `OS_PASSWORD`: OpenStack password
- `OS_PROJECT_NAME`: OpenStack project name
- `OS_USER_DOMAIN_NAME`: User domain name
- `OS_PROJECT_DOMAIN_NAME`: Project domain name
- `OS_REGION_NAME`: OpenStack region

**Usage**:
```bash
# Source OpenStack RC file
source ~/openstack-rc.sh

# Run preflight checks
openCenter cluster preflight my-openstack-cluster
```

**Notes**:
- Standard OpenStack environment variables
- Used for OpenStack provider authentication
- Required for preflight checks
- Credentials are masked in logs

## Feature Flag Variables

### OPENCENTER_USE_NEW_TEMPLATE_ENGINE

**Description**: Enable new template engine implementation.

**Default**: `false`

**Valid Values**: `true`, `false`, `1`, `0`, `yes`, `no`

**Usage**:
```bash
# Enable new template engine
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true
openCenter cluster setup my-cluster
```

**Notes**:
- Experimental feature flag
- See [Feature Flag Documentation](../../internal/template/FEATURE_FLAG.md)
- Overridden by `OPENCENTER_ENABLE_ALL_NEW_FEATURES`

### OPENCENTER_USE_NEW_CONFIG_BUILDER

**Description**: Enable type-safe configuration builder.

**Default**: `false`

**Valid Values**: `true`, `false`, `1`, `0`, `yes`, `no`

**Usage**:
```bash
# Enable new config builder
export OPENCENTER_USE_NEW_CONFIG_BUILDER=true
openCenter cluster init my-cluster
```

**Notes**:
- Experimental feature flag
- Provides type-safe configuration construction
- Overridden by `OPENCENTER_ENABLE_ALL_NEW_FEATURES`

### OPENCENTER_USE_SERVICE_REGISTRY

**Description**: Enable plugin-based service registry.

**Default**: `false`

**Valid Values**: `true`, `false`, `1`, `0`, `yes`, `no`

**Usage**:
```bash
# Enable service registry
export OPENCENTER_USE_SERVICE_REGISTRY=true
openCenter cluster setup my-cluster
```

**Notes**:
- Experimental feature flag
- Enables plugin-based service management
- Overridden by `OPENCENTER_ENABLE_ALL_NEW_FEATURES`

### OPENCENTER_ENABLE_ALL_NEW_FEATURES

**Description**: Enable all experimental features at once.

**Default**: `false`

**Valid Values**: `true`, `false`, `1`, `0`, `yes`, `no`

**Usage**:
```bash
# Enable all new features
export OPENCENTER_ENABLE_ALL_NEW_FEATURES=true
openCenter cluster setup my-cluster

# Override specific feature
export OPENCENTER_ENABLE_ALL_NEW_FEATURES=true
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=false  # Disable this one
openCenter cluster setup my-cluster
```

**Notes**:
- Master switch for all feature flags
- Individual flags can override this setting
- Useful for testing new features

### OPENCENTER_FEATURE_FLAG_DEBUG

**Description**: Enable debug logging for feature flag evaluation.

**Default**: `false`

**Valid Values**: `true`, `false`, `1`, `0`, `yes`, `no`

**Usage**:
```bash
# Enable feature flag debug logging
export OPENCENTER_FEATURE_FLAG_DEBUG=true
openCenter cluster setup my-cluster
```

**Effects**:
- Shows which feature flags are enabled
- Logs feature flag evaluation decisions
- Displays feature flag sources (env, config, default)

## Variable Precedence

When multiple configuration sources are available, openCenter uses this precedence order (highest to lowest):

1. **Command-line flags** (e.g., `--config-dir`)
2. **Environment variables** (e.g., `OPENCENTER_CONFIG_DIR`)
3. **Configuration file values**
4. **Default values**

## Security Considerations

### Credential Masking

The following environment variables are automatically masked in logs and output:

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `OS_PASSWORD`
- Any variable containing `PASSWORD`, `SECRET`, `TOKEN`, or `KEY`

### Best Practices

1. **Never commit credentials** to version control
2. **Use SOPS** for encrypting sensitive configuration
3. **Rotate credentials** regularly
4. **Use environment-specific** credentials
5. **Limit credential scope** to minimum required permissions

## Testing and CI/CD

### Common Test Patterns

```bash
# Isolated test environment
export OPENCENTER_CONFIG_DIR=./testdata/config
export OPENCENTER_DEBUG=true
mise run test

# CI/CD environment
export OPENCENTER_CONFIG_DIR=/tmp/opencenter-ci
export OPENCENTER_CLUSTERS_DIR=/tmp/opencenter-clusters
export CONTAINER_RUNTIME=docker
mise run godog
```

### Environment Cleanup

```bash
# Unset all openCenter variables
unset OPENCENTER_CONFIG_DIR
unset OPENCENTER_CLUSTERS_DIR
unset OPENCENTER_PLUGINS_DIR
unset OPENCENTER_DEBUG
unset OPENCENTER_ACTIVE_CLUSTER
unset OPENCENTER_ENABLE_ALL_NEW_FEATURES
unset OPENCENTER_FEATURE_FLAG_DEBUG
```

## Related Documentation

- [CLI Commands Reference](./cli-commands.md)
- [Configuration Reference](./configuration.md)
- [Shell Integration](./shell-integration.md)
- [Feature Flags](../../internal/template/FEATURE_FLAG.md)
