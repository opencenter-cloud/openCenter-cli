# CLI Commands Reference

**Purpose:** Complete reference of all openCenter CLI commands, flags, and options for quick lookup.

This reference documents all available CLI commands with their syntax, flags, and examples.

## Global Flags

Available for all commands:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | | Alternative cluster configuration file path |
| `--config-dir` | string | `~/.config/opencenter` | Configuration directory (legacy) |
| `--dry-run` | bool | false | Print planned actions without executing |
| `--log-level` | string | warn | Log level (debug, info, warn, error) |
| `--set` | string[] | | Override config values (dot notation) |
| `--show-active` | bool | false | Display current active cluster |
| `--help, -h` | bool | false | Show help for command |
| `--version` | bool | false | Show version information |

## opencenter

Root command for cluster management.

```bash
opencenter [command]
```

### Subcommands

- `cluster` - Manage cluster configurations
- `config` - Manage CLI configuration
- `sops` - SOPS secrets management
- `secrets` - Secrets operations (alias for sops)
- `plugins` - Plugin management
- `version` - Show version information

## opencenter cluster

Manage Kubernetes cluster configurations.

```bash
opencenter cluster [subcommand]
```

### cluster init

Initialize new cluster configuration.

```bash
opencenter cluster init <name> [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--org, --organization` | string | opencenter | Organization name |
| `--provider` | string | openstack | Infrastructure provider |
| `--force` | bool | false | Overwrite existing configuration |

**Examples:**

```bash
# Initialize with defaults
opencenter cluster init my-cluster

# Initialize with organization
opencenter cluster init my-cluster --org my-org

# Initialize for VMware
opencenter cluster init my-cluster --provider vmware

# Force overwrite existing
opencenter cluster init my-cluster --force
```

### cluster list

List all cluster configurations.

```bash
opencenter cluster list [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--org, --organization` | string | | Filter by organization |
| `--provider` | string | | Filter by provider |
| `--format` | string | table | Output format (table, json, yaml) |

**Examples:**

```bash
# List all clusters
opencenter cluster list

# List clusters in organization
opencenter cluster list --org my-org

# List OpenStack clusters
opencenter cluster list --provider openstack

# JSON output
opencenter cluster list --format json
```

### cluster select

Select active cluster for session or persistently.

```bash
opencenter cluster select [name] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--persistent` | bool | false | Save selection across terminals |
| `--activate` | bool | false | Activate cluster environment |
| `--export-only` | bool | false | Print export commands only |
| `--clear` | bool | false | Clear session selection |
| `--clear-persistent` | bool | false | Clear persistent selection |

**Examples:**

```bash
# Select for current session
opencenter cluster select my-cluster

# Select persistently
opencenter cluster select my-cluster --persistent

# Activate environment
eval "$(opencenter cluster select my-cluster --activate --export-only)"

# Clear selection
opencenter cluster select --clear
```

### cluster current

Show currently active cluster.

```bash
opencenter cluster current
```

**Examples:**

```bash
# Show active cluster
opencenter cluster current
```

### cluster env

Export cluster environment variables.

```bash
opencenter cluster env [name]
```

**Examples:**

```bash
# Export environment for active cluster
eval "$(opencenter cluster env)"

# Export for specific cluster
eval "$(opencenter cluster env my-cluster)"
```

### cluster status

Show cluster status and health.

```bash
opencenter cluster status [name]
```

**Examples:**

```bash
# Status of active cluster
opencenter cluster status

# Status of specific cluster
opencenter cluster status my-cluster
```

### cluster info

Show detailed cluster information including enabled services and GitOps status.

```bash
opencenter cluster info [name] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | false | Output in JSON format |
| `--validate` | bool | false | Validate cluster configuration invariants |
| `--export-only` | bool | false | Only output export commands for shell evaluation |
| `--shell` | string | auto | Override shell detection (bash, zsh, fish, powershell) |

**Output Sections:**

The command displays the following information:

1. **Cluster Metadata** - Name, organization, provider, environment, region, status
2. **GitOps Configuration** - Git directory and repository URL
3. **Enabled Services** - List of enabled services with their deployment status
4. **GitOps Status** (if cluster is in bootstrap or deployed stage) - FluxCD Kustomization reconciliation status
5. **Lock Status** - Current cluster lock information if locked

**Examples:**

```bash
# Show cluster info for active cluster
opencenter cluster info

# Show cluster info for specific cluster
opencenter cluster info my-cluster

# Show cluster info with organization prefix
opencenter cluster info 1861184-Metro-Bank-PLC/k8s-sandbox

# JSON format
opencenter cluster info my-cluster --json

# Validate configuration
opencenter cluster info my-cluster --validate

# Export environment variables only
opencenter cluster info my-cluster --export-only
```

**Example Output:**

```
Active cluster: 1861184-Metro-Bank-PLC/k8s-sandbox
Config Path: /Users/user/.config/opencenter/clusters/1861184-Metro-Bank-PLC/k8s-sandbox/.k8s-sandbox-config.yaml

git_dir: ~/customers/1861184-Metro-Bank-PLC
git_url: git@github.com:1861184-Metro-Bank-PLC/metro-bank-gitops.git

Metadata:
  name: Metro Bank Sandbox
  cluster_name: k8s-sandbox
  organization: 1861184-Metro-Bank-PLC
  provider: vmware
  env: sandbox
  region: sjc3
  status: deployed

Enabled Services:
  - calico (status: success)
  - cert-manager (status: success)
  - gateway (status: success)
  - harbor (status: success)
  - keycloak (status: success)
  - kube-prometheus-stack (status: success)
  - kyverno (status: success)
  - loki (status: success)
  - longhorn (status: success)
  - metallb (status: success)
  - velero (status: success)

GitOps Status:
  Kustomizations: 15 total
  Ready: 15/15

Lock Status:
  status: available
```

### cluster edit

Edit cluster configuration in editor.

```bash
opencenter cluster edit [name]
```

**Examples:**

```bash
# Edit active cluster
opencenter cluster edit

# Edit specific cluster
opencenter cluster edit my-cluster
```

### cluster validate

Validate cluster configuration.

```bash
opencenter cluster validate [name] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--check-connectivity` | bool | false | Check cloud provider connectivity |
| `--check-provider` | bool | false | Provider-specific validation |
| `--generate-debug-config` | bool | false | Generate complete config for debugging |
| `--output-dir` | string | . | Directory for debug config |
| `--verbose, -v` | bool | false | Verbose output |

**Examples:**

```bash
# Basic validation
opencenter cluster validate

# With connectivity checks
opencenter cluster validate --check-connectivity

# Generate debug config
opencenter cluster validate --generate-debug-config --output-dir /tmp
```

### cluster preflight

Run preflight checks before deployment.

```bash
opencenter cluster preflight [name] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--skip-connectivity` | bool | false | Skip connectivity checks |
| `--skip-quotas` | bool | false | Skip quota checks |

**Examples:**

```bash
# Run all preflight checks
opencenter cluster preflight

# Skip connectivity checks
opencenter cluster preflight --skip-connectivity
```

### cluster setup

Generate GitOps repository structure.

```bash
opencenter cluster setup [name] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--render` | bool | false | Render templates with config values |
| `--skip-git-init` | bool | false | Skip Git initialization |

**Examples:**

```bash
# Setup without rendering
opencenter cluster setup

# Setup and render templates
opencenter cluster setup --render

# Skip Git init
opencenter cluster setup --render --skip-git-init
```

### cluster render

Render templates without full setup.

```bash
opencenter cluster render [name] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--output-dir` | string | | Output directory for rendered files |

**Examples:**

```bash
# Render to default location
opencenter cluster render

# Render to custom directory
opencenter cluster render --output-dir /tmp/rendered
```

### cluster bootstrap

Bootstrap cluster with infrastructure and GitOps.

```bash
opencenter cluster bootstrap [name] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--skip-infrastructure` | bool | false | Skip infrastructure provisioning |
| `--skip-kubernetes` | bool | false | Skip Kubernetes deployment |
| `--skip-gitops` | bool | false | Skip GitOps bootstrap |

**Examples:**

```bash
# Full bootstrap
opencenter cluster bootstrap

# Skip infrastructure (already provisioned)
opencenter cluster bootstrap --skip-infrastructure

# Only GitOps bootstrap
opencenter cluster bootstrap --skip-infrastructure --skip-kubernetes
```

### cluster schema

Generate or view JSON schema.

```bash
opencenter cluster schema [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--pretty` | bool | false | Pretty-print JSON output |
| `--output` | string | | Write schema to file |

**Examples:**

```bash
# View schema
opencenter cluster schema

# Pretty-print
opencenter cluster schema --pretty

# Save to file
opencenter cluster schema --output schema.json
```

### cluster template

Generate configuration template.

```bash
opencenter cluster template [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--provider` | string | openstack | Provider for template |
| `--out` | string | | Output file path |

**Examples:**

```bash
# OpenStack template
opencenter cluster template --provider openstack --out config.yaml

# VMware template
opencenter cluster template --provider vmware --out vmware-config.yaml
```

### cluster destroy

Destroy cluster infrastructure.

```bash
opencenter cluster destroy [name] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--force` | bool | false | Skip confirmation prompt |
| `--keep-config` | bool | false | Keep configuration files |

**Examples:**

```bash
# Destroy with confirmation
opencenter cluster destroy my-cluster

# Force destroy
opencenter cluster destroy my-cluster --force

# Destroy but keep config
opencenter cluster destroy my-cluster --keep-config
```

### cluster update

Update cluster configuration.

```bash
opencenter cluster update [name] [flags]
```

**Examples:**

```bash
# Update active cluster
opencenter cluster update

# Update specific cluster
opencenter cluster update my-cluster
```

### cluster service

Manage cluster services.

```bash
opencenter cluster service [subcommand]
```

**Subcommands:**

- `list` - List available services
- `enable` - Enable service
- `disable` - Disable service
- `status` - Show service status

**Examples:**

```bash
# List services
opencenter cluster service list

# Enable service
opencenter cluster service enable keycloak

# Disable service
opencenter cluster service disable loki
```

### cluster credentials

Manage cloud provider credentials.

```bash
opencenter cluster credentials [subcommand]
```

**Subcommands:**

- `validate` - Validate credentials
- `rotate` - Rotate credentials

**Examples:**

```bash
# Validate credentials
opencenter cluster credentials validate

# Rotate credentials
opencenter cluster credentials rotate
```

### cluster drift

Detect configuration drift.

```bash
opencenter cluster drift [name]
```

**Examples:**

```bash
# Check drift
opencenter cluster drift
```

### cluster backup

Backup cluster configuration.

```bash
opencenter cluster backup [name] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--output-dir` | string | | Backup directory |

**Examples:**

```bash
# Backup to default location
opencenter cluster backup

# Backup to custom directory
opencenter cluster backup --output-dir /backups
```

### cluster lock

Lock cluster to prevent modifications.

```bash
opencenter cluster lock [name]
```

**Examples:**

```bash
# Lock cluster
opencenter cluster lock my-cluster
```

### cluster unlock

Unlock cluster.

```bash
opencenter cluster unlock [name]
```

**Examples:**

```bash
# Unlock cluster
opencenter cluster unlock my-cluster
```

### cluster config

Manage cluster configuration values.

```bash
opencenter cluster config [subcommand]
```

**Subcommands:**

- `get` - Get configuration value
- `set` - Set configuration value
- `unset` - Remove configuration value
- `view` - View entire configuration

**Examples:**

```bash
# Get value
opencenter cluster config get opencenter.cluster.kubernetes.version

# Set value
opencenter cluster config set opencenter.cluster.kubernetes.version 1.33.5

# View all
opencenter cluster config view
```

### cluster validate-manifests

Validate generated Kubernetes manifests.

```bash
opencenter cluster validate-manifests [name]
```

**Examples:**

```bash
# Validate manifests
opencenter cluster validate-manifests
```

### cluster sync-secrets

Synchronize secrets across environments.

```bash
opencenter cluster sync-secrets [name]
```

**Examples:**

```bash
# Sync secrets
opencenter cluster sync-secrets
```

### cluster validate-secrets

Validate secrets configuration.

```bash
opencenter cluster validate-secrets [name]
```

**Examples:**

```bash
# Validate secrets
opencenter cluster validate-secrets
```

### cluster rotate-keys

Rotate encryption keys.

```bash
opencenter cluster rotate-keys [name] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--backup-dir` | string | | Backup directory for old keys |

**Examples:**

```bash
# Rotate keys
opencenter cluster rotate-keys

# With custom backup location
opencenter cluster rotate-keys --backup-dir /secure/backups
```

### cluster check-keys

Check encryption key expiration.

```bash
opencenter cluster check-keys [name]
```

**Examples:**

```bash
# Check keys
opencenter cluster check-keys
```

### cluster audit-log

View cluster audit log.

```bash
opencenter cluster audit-log [name] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--since` | string | | Show logs since timestamp |
| `--tail` | int | 100 | Number of recent entries |

**Examples:**

```bash
# View recent logs
opencenter cluster audit-log

# Last 50 entries
opencenter cluster audit-log --tail 50

# Since specific time
opencenter cluster audit-log --since "2026-02-17T10:00:00Z"
```

### cluster revoke-key

Revoke encryption key.

```bash
opencenter cluster revoke-key [name] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--key-id` | string | | Key ID to revoke |

**Examples:**

```bash
# Revoke key
opencenter cluster revoke-key --key-id abc123
```

### cluster install-hooks

Install Git hooks for validation.

```bash
opencenter cluster install-hooks [name]
```

**Examples:**

```bash
# Install hooks
opencenter cluster install-hooks
```

### cluster keys

Manage SSH and encryption keys.

```bash
opencenter cluster keys [subcommand]
```

**Subcommands:**

- `list` - List keys
- `generate` - Generate new key
- `import` - Import existing key
- `export` - Export key

**Examples:**

```bash
# List keys
opencenter cluster keys list

# Generate SSH key
opencenter cluster keys generate --type ssh

# Import key
opencenter cluster keys import --file /path/to/key
```

## opencenter config

Manage CLI configuration.

```bash
opencenter config [subcommand]
```

**Subcommands:**

- `view` - View CLI configuration
- `set` - Set configuration value
- `get` - Get configuration value
- `init` - Initialize CLI configuration

**Examples:**

```bash
# View config
opencenter config view

# Set default provider
opencenter config set defaults.provider openstack

# Get value
opencenter config get defaults.provider
```

## opencenter sops

SOPS secrets management.

```bash
opencenter sops [subcommand]
```

**Subcommands:**

- `generate-key` - Generate Age key pair
- `rotate-key` - Rotate Age key
- `backup-key` - Backup Age key
- `validate` - Validate SOPS configuration
- `secrets-list` - List encrypted secrets
- `secrets-status` - Show encryption status
- `secrets-encrypt` - Encrypt secrets
- `secrets-encrypt-fast` - Parallel encryption
- `secrets-decrypt` - Decrypt secrets
- `secrets-decrypt-fast` - Parallel decryption

**Examples:**

```bash
# Generate key
opencenter sops generate-key --cluster my-cluster

# Rotate key
opencenter sops rotate-key --cluster my-cluster

# Encrypt secrets
opencenter sops secrets-encrypt --cluster my-cluster

# List encrypted files
opencenter sops secrets-list --cluster my-cluster
```

## opencenter plugins

Plugin management.

```bash
opencenter plugins [subcommand]
```

**Subcommands:**

- `list` - List installed plugins
- `install` - Install plugin
- `uninstall` - Uninstall plugin
- `update` - Update plugin

**Examples:**

```bash
# List plugins
opencenter plugins list

# Install plugin
opencenter plugins install my-plugin

# Update all plugins
opencenter plugins update
```

## opencenter version

Show version information.

```bash
opencenter version [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--short` | bool | false | Show version number only |

**Examples:**

```bash
# Full version info
opencenter version

# Version number only
opencenter version --short
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `OPENCENTER_CONFIG_DIR` | Configuration directory | `~/.config/opencenter` |
| `SOPS_AGE_KEY_FILE` | Path to Age key file | |
| `SOPS_AGE_RECIPIENTS` | Age public keys for encryption | |
| `KUBECONFIG` | Kubernetes config file | `~/.kube/config` |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Validation error |
| 3 | Configuration error |
| 4 | Provider error |
| 5 | Network error |

---

## Evidence

This reference is based on:

- Root command: `cmd/root.go:1-300`
- Cluster commands: `cmd/cluster.go:1-200`
- Global flags: `cmd/root.go:250-270`
- Command structure: `.kiro/steering/structure.md:20-29`
- Session 2 facts inventory: B0 section 2
