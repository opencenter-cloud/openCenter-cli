---
id: environment-variables
title: "Environment Variables"
sidebar_label: Environment Variables
description: Complete reference of environment variables recognized by openCenter CLI.
doc_type: reference
audience: "all users"
tags: [environment, variables, configuration, cli]
---

# Environment Variables

**Purpose:** For all users, provides complete reference of environment variables and configuration precedence.

This reference documents all environment variables recognized by openCenter CLI and how they interact with configuration files.

## Overview

openCenter CLI uses environment variables for:
- Configuration overrides
- Credential management
- Behavior customization
- CI/CD integration
- Runtime state location

**Configuration Precedence (highest to lowest):**
1. Command-line flags (`--set`, `--log-level`, etc.)
2. Environment variables
3. Cluster config file (`.<cluster>-config.yaml`)
4. CLI config file (`~/.config/opencenter/config.yaml`)
5. Built-in defaults

For the full breakdown including directory resolution and provider credentials, see [Configuration Precedence](configuration-precedence.md).

## Core Environment Variables

### OPENCENTER_CONFIG_DIR

Configuration directory location.

**Default:** `~/.config/opencenter`

**Usage:**
```bash
export OPENCENTER_CONFIG_DIR=/custom/path
opencenter cluster init my-cluster
```

**What it affects:**
- Cluster configuration location
- Secrets storage location
- CLI defaults location

### OPENCENTER_STATE_DIR

Runtime state directory location.

**Default:** `${XDG_STATE_HOME:-~/.local/state}/opencenter`

**Usage:**
```bash
export OPENCENTER_STATE_DIR=/custom/state
opencenter cluster bootstrap my-cluster
```

**What it affects:**
- Bootstrap resume state location
- Default bootstrap log location
- Audit log location
- File lock location

**Example:**
```bash
export OPENCENTER_STATE_DIR=/tmp/opencenter-state
opencenter cluster bootstrap dev-cluster

# Files created under:
# /tmp/opencenter-state/bootstrap/<org>/<cluster>/state.json
# /tmp/opencenter-state/logs/bootstrap/<org>/<cluster>/bootstrap-YYYYMMDDTHHMMSSZ.log
# /tmp/opencenter-state/audit/audit.log
# /tmp/opencenter-state/locks/
```

### XDG_STATE_HOME

Base XDG state directory used when `OPENCENTER_STATE_DIR` is unset.

**Default:** `~/.local/state`

**Usage:**
```bash
export XDG_STATE_HOME=/srv/state
opencenter cluster bootstrap my-cluster
```

**What it affects:**
- Default base directory for openCenter runtime state
- Expands to `${XDG_STATE_HOME}/opencenter`

**Example:**
```bash
# Use custom config directory
export OPENCENTER_CONFIG_DIR=/tmp/opencenter
opencenter cluster init test-cluster

# Configuration created at:
# /tmp/opencenter/clusters/my-org/.test-cluster-config.yaml
```

### OPENCENTER_CLUSTER

Active cluster name.

**Default:** None (must be set or use `--cluster` flag)

**Usage:**
```bash
export OPENCENTER_CLUSTER=my-cluster
opencenter cluster validate
```

**What it affects:**
- Default cluster for commands
- Avoids need for `--cluster` flag

**Example:**
```bash
# Set active cluster
export OPENCENTER_CLUSTER=prod-cluster

# Commands use active cluster
opencenter cluster validate  # Validates prod-cluster
opencenter cluster status    # Shows prod-cluster status
```

### OPENCENTER_ORG

Active organization name.

**Default:** None (must be set or use `--org` flag)

**Usage:**
```bash
export OPENCENTER_ORG=my-company
opencenter cluster list
```

**What it affects:**
- Default organization for commands
- Cluster lookup path

**Example:**
```bash
# Set active organization
export OPENCENTER_ORG=my-company

# Commands use active organization
opencenter cluster list  # Lists clusters in my-company
opencenter cluster init dev  # Creates cluster in my-company
```

### OPENCENTER_LOG_LEVEL

Logging verbosity level.

**Default:** `info`

**Allowed values:** `debug`, `info`, `warn`, `error`

**Usage:**
```bash
export OPENCENTER_LOG_LEVEL=debug
opencenter cluster validate my-cluster
```

**What it affects:**
- Log output verbosity
- Debug information visibility

**Example:**
```bash
# Enable debug logging
export OPENCENTER_LOG_LEVEL=debug
opencenter cluster bootstrap my-cluster

# Disable most logging
export OPENCENTER_LOG_LEVEL=error
opencenter cluster validate my-cluster
```

### KIND_EXPERIMENTAL_PROVIDER

Optional runtime selector for Kind when using non-default container engines.

**Typical value:** `podman`

**Usage:**
```bash
export KIND_EXPERIMENTAL_PROVIDER=podman
opencenter cluster bootstrap dev-cluster
```

**What it affects:**
- Kind bootstrap and destroy flows
- Local developer and CI environments using Podman instead of Docker

**GA note:** This variable is relevant only for the local Kind provider.

## Provider and Integration Environment Variables

### OpenStack

#### OS_CLOUD

OpenStack cloud profile name (from `clouds.yaml`).

**Default:** None

**Usage:**
```bash
export OS_CLOUD=openstack
opencenter cluster bootstrap my-cluster
```

**What it affects:**
- OpenStack authentication
- Uses credentials from `~/.config/openstack/clouds.yaml`

**Example:**
```bash
# Use specific cloud profile
export OS_CLOUD=production-openstack
opencenter cluster validate my-cluster
```

#### OS_AUTH_URL

OpenStack authentication URL.

**Default:** None (from configuration or clouds.yaml)

**Usage:**
```bash
export OS_AUTH_URL=https://identity.api.rackspacecloud.com/v3
opencenter cluster bootstrap my-cluster
```

**What it affects:**
- OpenStack API endpoint
- Overrides configuration file value

#### OS_USERNAME

OpenStack username.

**Default:** None (from configuration or clouds.yaml)

**Usage:**
```bash
export OS_USERNAME=my-username
opencenter cluster bootstrap my-cluster
```

#### OS_PASSWORD

OpenStack password.

**Default:** None (from configuration or clouds.yaml)

**Usage:**
```bash
export OS_PASSWORD=my-password
opencenter cluster bootstrap my-cluster
```

**Security note:** Avoid using this in production. Use `clouds.yaml` or configuration file with SOPS encryption instead.

#### OS_PROJECT_NAME

OpenStack project name.

**Default:** None (from configuration or clouds.yaml)

**Usage:**
```bash
export OS_PROJECT_NAME=my-project
opencenter cluster bootstrap my-cluster
```

#### OS_REGION_NAME

OpenStack region name.

**Default:** None (from configuration or clouds.yaml)

**Usage:**
```bash
export OS_REGION_NAME=sjc3
opencenter cluster bootstrap my-cluster
```

### VMware

#### VSPHERE_SERVER

vSphere server hostname.

**Default:** None (from configuration)

**Usage:**
```bash
export VSPHERE_SERVER=vcenter.example.com
opencenter cluster bootstrap my-cluster
```

#### VSPHERE_USER

vSphere username.

**Default:** None (from configuration)

**Usage:**
```bash
export VSPHERE_USER=administrator@vsphere.local
opencenter cluster bootstrap my-cluster
```

#### VSPHERE_PASSWORD

vSphere password.

**Default:** None (from configuration)

**Usage:**
```bash
export VSPHERE_PASSWORD=my-password
opencenter cluster bootstrap my-cluster
```

**Security note:** Avoid using this in production. Use configuration file with SOPS encryption instead.

### AWS Service Integrations

AWS environment variables remain relevant for GA features that integrate with AWS services such as Route53 or S3-compatible backends. They do not make AWS a supported GA infrastructure provider.

#### AWS_ACCESS_KEY_ID

AWS access key ID.

**Default:** None (from configuration or AWS credentials file)

**Usage:**
```bash
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
opencenter cluster validate my-cluster
```

#### AWS_SECRET_ACCESS_KEY

AWS secret access key.

**Default:** None (from configuration or AWS credentials file)

**Usage:**
```bash
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
opencenter cluster validate my-cluster
```

**Security note:** Avoid using this in production. Use AWS credentials file or IAM roles instead.

#### AWS_REGION

AWS region.

**Default:** None (from configuration)

**Usage:**
```bash
export AWS_REGION=us-east-1
opencenter cluster validate my-cluster
```

## Secrets Environment Variables

### SOPS_AGE_KEY

SOPS Age private key for decryption.

**Default:** None (from key file)

**Usage:**
```bash
export SOPS_AGE_KEY="AGE-SECRET-KEY-1..."
opencenter sops secrets-decrypt
```

**What it affects:**
- SOPS decryption operations
- Secrets management commands

**Example:**
```bash
# Use Age key from environment
export SOPS_AGE_KEY=$(cat ~/.config/opencenter/clusters/my-org/secrets/age/my-cluster-key.txt)
opencenter sops secrets-decrypt
```

### SOPS_AGE_KEY_FILE

Path to SOPS Age key file.

**Default:** `~/.config/opencenter/clusters/<org>/secrets/age/<cluster>-key.txt`

**Usage:**
```bash
export SOPS_AGE_KEY_FILE=/path/to/age-key.txt
opencenter sops secrets-decrypt
```

**What it affects:**
- SOPS key file location
- Secrets management commands

## Kubernetes Environment Variables

### KUBECONFIG

Kubernetes configuration file path.

**Default:** `~/.kube/config`

**Usage:**
```bash
export KUBECONFIG=~/my-cluster-gitops/infrastructure/clusters/my-cluster/kubeconfig.yaml
kubectl get nodes
```

**What it affects:**
- kubectl commands
- Kubernetes API access
- Cluster operations

**Example:**
```bash
# Use cluster-specific kubeconfig
export KUBECONFIG=~/prod-cluster-gitops/infrastructure/clusters/prod-cluster/kubeconfig.yaml
kubectl get nodes
opencenter cluster status
```

## CI/CD Environment Variables

### CI

Indicates running in CI environment.

**Default:** None (set by CI platform)

**Values:** `true` (set by CI platforms)

**Usage:**
```bash
# Automatically set by CI platforms
# GitHub Actions: CI=true
# GitLab CI: CI=true
# Jenkins: CI=true
```

**What it affects:**
- Output formatting (less interactive)
- Error handling (fail fast)
- Logging (more verbose)

### GITHUB_ACTIONS

Indicates running in GitHub Actions.

**Default:** None (set by GitHub Actions)

**Values:** `true` (set by GitHub Actions)

**Usage:**
```bash
# Automatically set by GitHub Actions
# GITHUB_ACTIONS=true
```

**What it affects:**
- GitHub-specific output formatting
- Annotations and warnings

### GITLAB_CI

Indicates running in GitLab CI.

**Default:** None (set by GitLab CI)

**Values:** `true` (set by GitLab CI)

**Usage:**
```bash
# Automatically set by GitLab CI
# GITLAB_CI=true
```

**What it affects:**
- GitLab-specific output formatting
- Job logs and artifacts

## Configuration Precedence Examples

### Example 1: Override Worker Count

```bash
# Configuration file
opencenter:
  cluster:
    worker_count: 3

# Environment variable (higher precedence)
export OPENCENTER_WORKER_COUNT=5

# Command-line flag (highest precedence)
opencenter cluster init my-cluster --set cluster.worker_count=7

# Result: worker_count = 7 (command-line flag wins)
```

### Example 2: Override Provider Credentials

```bash
# Configuration file
opencenter:
  infrastructure:
    openstack:
      username: "config-user"
      password: "config-password"

# Environment variables (higher precedence)
export OS_USERNAME="env-user"
export OS_PASSWORD="env-password"

# Result: Uses env-user and env-password
```

### Example 3: Multiple Configuration Sources

```bash
# Built-in default: worker_count = 2
# CLI default (~/.config/opencenter/config.yaml): worker_count = 3
# Configuration file: worker_count = 4
# Environment variable: OPENCENTER_WORKER_COUNT=5
# Command-line flag: --set cluster.worker_count=6

# Result: worker_count = 6 (command-line flag has highest precedence)
```

## Setting Environment Variables

### Temporary (Current Session)

```bash
# Set for current session
export OPENCENTER_CONFIG_DIR=/tmp/opencenter
opencenter cluster init test-cluster

# Unset after use
unset OPENCENTER_CONFIG_DIR
```

### Permanent (Shell Profile)

```bash
# Add to ~/.bashrc or ~/.zshrc
echo 'export OPENCENTER_CONFIG_DIR=~/opencenter' >> ~/.bashrc
source ~/.bashrc

# Or add to ~/.profile
echo 'export OPENCENTER_ORG=my-company' >> ~/.profile
source ~/.profile
```

### Per-Command

```bash
# Set for single command
OPENCENTER_LOG_LEVEL=debug opencenter cluster validate my-cluster

# Multiple variables
OS_CLOUD=openstack OPENCENTER_LOG_LEVEL=debug opencenter cluster bootstrap my-cluster
```

### CI/CD Secrets

```bash
# GitHub Actions
# Settings → Secrets → New repository secret
# Name: OPENSTACK_PASSWORD
# Value: your-password

# Use in workflow
env:
  OS_PASSWORD: ${{ secrets.OPENSTACK_PASSWORD }}

# GitLab CI
# Settings → CI/CD → Variables → Add variable
# Key: OPENSTACK_PASSWORD
# Value: your-password
# Protected: Yes
# Masked: Yes

# Use in pipeline
variables:
  OS_PASSWORD: $OPENSTACK_PASSWORD
```

## Best Practices

1. **Use configuration files for persistent settings:** Environment variables for temporary overrides only
2. **Never commit credentials:** Use SOPS encryption or secret management
3. **Use CI/CD secrets:** For credentials in pipelines
4. **Document required variables:** In README or CI/CD configuration
5. **Use descriptive names:** `OPENCENTER_*` prefix for clarity
6. **Validate before use:** Check environment variables are set correctly
7. **Unset after use:** Clean up temporary variables

## Troubleshooting

### Variable Not Recognized

**Symptom:** Environment variable has no effect

**Diagnosis:**
```bash
# Check variable is set
echo $OPENCENTER_CONFIG_DIR

# Check variable name (case-sensitive)
env | grep OPENCENTER
```

**Solution:**
```bash
# Ensure correct variable name
export OPENCENTER_CONFIG_DIR=/custom/path  # Correct
export opencenter_config_dir=/custom/path  # Wrong (lowercase)

# Verify variable is exported
export OPENCENTER_CONFIG_DIR=/custom/path
echo $OPENCENTER_CONFIG_DIR
```

### Precedence Issues

**Symptom:** Configuration value not as expected

**Diagnosis:**
```bash
# Check all configuration sources
opencenter cluster config my-cluster --show-precedence

# Check environment variables
env | grep -E '(OPENCENTER|OS_|AWS_|VSPHERE_)'
```

**Solution:**
```bash
# Unset conflicting environment variables
unset OPENCENTER_WORKER_COUNT

# Or use command-line flag to override
opencenter cluster init my-cluster --set cluster.worker_count=5
```

## Related Topics

- [Configuration Schema](configuration-schema.md) - Complete field reference
- [CLI Commands](cli-commands.md) - Command-line flags
- [Integrate CI/CD](../how-to/integrate-ci-cd.md) - CI/CD integration
- [Configuration Lifecycle](../explanation/configuration-lifecycle.md) - Configuration management

---

## Evidence

This reference is based on:

- Configuration precedence: `internal/config/manager.go`, Session 2 B0 section 3
- Environment variables: `cmd/root.go`, `internal/config/`
- Provider variables: OpenStack and VMware documentation; AWS-backed service integration references
- SOPS variables: `internal/sops/manager.go`
