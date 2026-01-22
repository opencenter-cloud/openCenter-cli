# CLI Command Reference


## Table of Contents

- [Global Flags](#global-flags)
- [Cluster Commands](#cluster-commands)
- [SOPS Commands](#sops-commands)
- [Config Commands](#config-commands)
- [Version Command](#version-command)
- [Plugins Commands](#plugins-commands)
- [Environment Variables](#environment-variables)
- [Exit Codes](#exit-codes)
- [See Also](#see-also)
Complete reference for all openCenter CLI commands.

## Global Flags

These flags are available for all commands:

```bash
--config string          Alternative cluster configuration file path
--config-dir string      Configuration directory (default: ~/.config/openCenter)
--dry-run               Enable dry-run mode (show actions without executing)
--log-level string      Set log level (debug, info, warn, error) (default: "warn")
--set stringArray       Override configuration values using dot notation
--verbose               Enable verbose logging (sets log level to debug)
```

### Examples

```bash
# Use custom config directory
openCenter --config-dir=/tmp/configs cluster list

# Dry-run mode
openCenter --dry-run cluster init test-cluster

# Override configuration values
openCenter --set opencenter.meta.env=prod cluster init my-cluster

# Verbose logging
openCenter --verbose cluster validate my-cluster
```

## Cluster Commands

### cluster init

Initialize a new cluster configuration with default values.

```bash
openCenter cluster init <name> [flags]
```

**Flags:**
- `--force`: Overwrite existing configuration
- `--no-sops-keygen`: Skip automatic SOPS key generation
- `--strict`: Enable strict validation during initialization
- `--opencenter.<path>=<value>`: Override any configuration value using dot notation

**Examples:**

```bash
# Basic initialization
openCenter cluster init my-cluster

# Initialize with organization
openCenter cluster init my-cluster --opencenter.meta.organization=myorg

# Initialize with custom values
openCenter cluster init my-cluster \
  --opencenter.meta.env=prod \
  --opencenter.cluster.kubernetes.version=1.31.4 \
  --opencenter.infrastructure.provider=aws

# Force overwrite existing
openCenter cluster init my-cluster --force

# Skip SOPS key generation
openCenter cluster init my-cluster --no-sops-keygen
```

**Output:**
- Creates cluster configuration file
- Generates SOPS Age key (unless --no-sops-keygen)
- Creates organization directory structure
- Displays paths to created files

### cluster validate

Validate cluster configuration against schema and business rules.

```bash
openCenter cluster validate [name] [flags]
```

**Flags:**
- `--generate-debug-config`: Generate complete configuration for debugging
- `--output-dir string`: Directory to save debug config (default: GitOps directory)

**Examples:**

```bash
# Validate active cluster
openCenter cluster validate

# Validate specific cluster
openCenter cluster validate my-cluster

# Generate debug configuration
openCenter cluster validate my-cluster --generate-debug-config

# Save debug config to specific directory
openCenter cluster validate my-cluster --generate-debug-config --output-dir=/tmp
```

**Validation Checks:**
- Schema validation
- Required field validation
- Cross-field dependency validation
- Cloud provider credential validation
- Network configuration validation
- SOPS key validation
- Network plugin mutual exclusivity

### cluster list

List all configured clusters across all organizations.

```bash
openCenter cluster list
```

**Output:**
- Sorted list of cluster names
- Supports both organization-based and legacy structures

**Example:**

```bash
$ openCenter cluster list
cluster1
cluster2
my-prod-cluster
test-cluster
```

### cluster select

Set the active cluster for subsequent commands.

```bash
openCenter cluster select <name>
```

**Examples:**

```bash
# Select a cluster
openCenter cluster select my-cluster

# Verify selection
openCenter cluster current
```

### cluster current

Display the currently active cluster.

```bash
openCenter cluster current
```

**Example:**

```bash
$ openCenter cluster current
my-cluster
```

### cluster info

Display detailed information about a cluster.

```bash
openCenter cluster info [name]
```

**Examples:**

```bash
# Show info for active cluster
openCenter cluster info

# Show info for specific cluster
openCenter cluster info my-cluster
```

**Output:**
- Cluster name and organization
- Provider and region
- Kubernetes version
- Node counts
- Network plugin
- GitOps repository location
- SOPS key location

### cluster setup

Setup GitOps directory structure and initialize Git repository.

```bash
openCenter cluster setup [name] [flags]
```

**Flags:**
- `--render`: Render templates (rather than copy)
- `--force`: Overwrite existing files and reinitialize

**Examples:**

```bash
# Setup GitOps for active cluster
openCenter cluster setup

# Setup with template rendering
openCenter cluster setup my-cluster --render

# Force reinitialize
openCenter cluster setup my-cluster --force
```

**Using Feature Flags:**

```bash
# Use new template engine for better performance
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true
openCenter cluster setup my-cluster

# Use pipeline-based generation with rollback support
export OPENCENTER_USE_PIPELINE_GENERATOR=true
openCenter cluster setup my-cluster

# Enable all new features
export OPENCENTER_ENABLE_ALL_NEW_FEATURES=true
openCenter cluster setup my-cluster

# Enable for a single command
OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true openCenter cluster setup my-cluster
```

**Actions:**
- Creates GitOps directory structure
- Copies/renders base templates
- Renders cluster-specific manifests
- Provisions OpenTofu configuration
- Initializes Git repository
- Creates SOPS configuration

### cluster bootstrap

Run provider-specific bootstrap actions.

```bash
openCenter cluster bootstrap [name] [flags]
```

**Flags:**
- `--dry-run`: Show planned actions without executing
- `--kubeconfig string`: Path to kubeconfig (default: "./kubeconfig.yaml")
- `--log string`: Log file path (default: <git_dir>/infrastructure/clusters/<name>/bootstrap.log)
- `--container-runtime string`: Container runtime for Kind clusters (docker or podman)

**Examples:**

```bash
# Bootstrap active cluster
openCenter cluster bootstrap

# Bootstrap with specific kubeconfig
openCenter cluster bootstrap my-cluster --kubeconfig=/path/to/kubeconfig

# Bootstrap Kind cluster with Podman
openCenter cluster bootstrap kind-cluster --container-runtime=podman

# Dry-run mode
openCenter cluster bootstrap my-cluster --dry-run
```

**Provider-Specific Actions:**

**OpenStack/AWS/GCP/Azure:**
- Runs `make` in cluster infrastructure directory
- Applies OpenTofu/Terraform configuration
- Provisions infrastructure resources

**Kind:**
- Creates Kind cluster with specified runtime
- Exports kubeconfig
- Disables default CNI for custom network plugin

### cluster render

Render cluster templates without full setup.

```bash
openCenter cluster render [name]
```

**Examples:**

```bash
# Render templates for active cluster
openCenter cluster render

# Render for specific cluster
openCenter cluster render my-cluster
```

**Using Feature Flags:**

```bash
# Use new template engine with caching
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true
openCenter cluster render my-cluster

# Compare legacy vs new template engine output
openCenter cluster render my-cluster > /tmp/legacy.txt
OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true openCenter cluster render my-cluster > /tmp/new.txt
diff /tmp/legacy.txt /tmp/new.txt
```

### cluster schema

Generate and display JSON schema for cluster configuration.

```bash
openCenter cluster schema [flags]
```

**Flags:**
- `--pretty`: Pretty-print JSON output
- `--out string`: Output file path (default: stdout)

**Examples:**

```bash
# Display schema
openCenter cluster schema --pretty

# Save to file
openCenter cluster schema --pretty --out schema/cluster.schema.json
```

**Output:**
- JSON Schema Draft 2020-12 format
- Complete validation rules
- Property descriptions
- Default values
- Examples

### cluster update

Update cluster configuration with new values.

```bash
openCenter cluster update <name> [flags]
```

**Flags:**
- `--<path>=<value>`: Override any configuration value using dot notation

**Examples:**

```bash
# Update Kubernetes version
openCenter cluster update my-cluster --opencenter.cluster.kubernetes.version=1.31.4

# Update provider
openCenter cluster update my-cluster --opencenter.infrastructure.provider=aws

# Update multiple values
openCenter cluster update my-cluster \
  --opencenter.meta.env=prod \
  --opencenter.cluster.kubernetes.master_count=5
```

### cluster migrate

Migrate cluster configuration to new schema version.

```bash
openCenter cluster migrate <name> [flags]
```

**Flags:**
- `--to-version string`: Target schema version
- `--backup`: Create backup before migration (default: true)

**Examples:**

```bash
# Migrate to latest schema
openCenter cluster migrate my-cluster

# Migrate to specific version
openCenter cluster migrate my-cluster --to-version=2.0.0

# Migrate without backup
openCenter cluster migrate my-cluster --backup=false
```

### cluster preflight

Run preflight checks before cluster deployment.

```bash
openCenter cluster preflight [name]
```

**Examples:**

```bash
# Run preflight checks for active cluster
openCenter cluster preflight

# Run for specific cluster
openCenter cluster preflight my-cluster
```

**Checks:**
- Configuration validation
- Cloud provider connectivity
- Required tools availability
- Network connectivity
- Resource quotas
- DNS resolution

### cluster destroy

Destroy cluster infrastructure and clean up resources.

```bash
openCenter cluster destroy <name> [flags]
```

**Flags:**
- `--force`: Skip confirmation prompt
- `--keep-config`: Keep configuration files after destruction

**Examples:**

```bash
# Destroy cluster (with confirmation)
openCenter cluster destroy my-cluster

# Force destroy without confirmation
openCenter cluster destroy my-cluster --force

# Destroy but keep configuration
openCenter cluster destroy my-cluster --keep-config
```

## SOPS Commands

### sops generate-key

Generate new Age key pair for SOPS encryption.

```bash
openCenter sops generate-key [flags]
```

**Flags:**
- `--key-file string`: Path to save Age key (default: ~/.config/sops/age/keys.txt)
- `--update-sops-config`: Update .sops.yaml with new public key (default: true)
- `--dry-run`: Show what would be done without making changes

**Examples:**

```bash
# Generate new key
openCenter sops generate-key

# Generate with custom path
openCenter sops generate-key --key-file=/path/to/key.txt

# Dry-run mode
openCenter sops generate-key --dry-run
```

### sops rotate-key

Rotate Age keys and re-encrypt existing secrets.

```bash
openCenter sops rotate-key [flags]
```

**Flags:**
- `--key-file string`: Path to Age key file
- `--search-path string`: Path to search for SOPS files (default: ".")
- `--dry-run`: Show what would be done without making changes

**Examples:**

```bash
# Rotate key and re-encrypt all secrets
openCenter sops rotate-key

# Rotate with custom search path
openCenter sops rotate-key --search-path=./gitops

# Dry-run mode
openCenter sops rotate-key --dry-run
```

**Process:**
1. Backs up existing key
2. Generates new key pair
3. Finds all SOPS-encrypted files
4. Re-encrypts each file with new key
5. Updates .sops.yaml configuration

### sops backup-key

Create backup of Age keys and SOPS configuration.

```bash
openCenter sops backup-key [flags]
```

**Flags:**
- `--key-file string`: Path to Age key file
- `--backup-dir string`: Backup directory (default: ~/.config/sops/age/backups)
- `--dry-run`: Show what would be done without making changes

**Examples:**

```bash
# Create backup
openCenter sops backup-key

# Backup to custom directory
openCenter sops backup-key --backup-dir=/secure/backups

# Dry-run mode
openCenter sops backup-key --dry-run
```

### sops validate

Validate Age key configuration and SOPS setup.

```bash
openCenter sops validate [flags]
```

**Flags:**
- `--key-file string`: Path to Age key file
- `--config-file string`: Path to SOPS config (default: ".sops.yaml")
- `--dry-run`: Show what would be done without making changes

**Examples:**

```bash
# Validate SOPS setup
openCenter sops validate

# Validate with custom config
openCenter sops validate --config-file=/path/to/.sops.yaml

# Dry-run mode
openCenter sops validate --dry-run
```

**Validation Checks:**
- Age key file existence and permissions
- Age key format validation
- SOPS configuration validation
- Key access test
- SOPS installation check

### sops secrets-list

List all SOPS-encrypted files.

```bash
openCenter sops secrets-list [flags]
```

**Flags:**
- `--search-path string`: Path to search (default: ".")
- `--dry-run`: Show what would be done without making changes

**Examples:**

```bash
# List encrypted files
openCenter sops secrets-list

# List in specific directory
openCenter sops secrets-list --search-path=./gitops
```

### sops secrets-encrypt

Encrypt secrets with automatic backup creation.

```bash
openCenter sops secrets-encrypt [flags]
```

**Flags:**
- `--search-path string`: Path to search for files (default: ".")
- `--backups`: Create backups before encryption (default: true)
- `--dry-run`: Show what would be done without making changes

**Examples:**

```bash
# Encrypt all secrets
openCenter sops secrets-encrypt

# Encrypt without backups (faster)
openCenter sops secrets-encrypt --backups=false

# Dry-run mode
openCenter sops secrets-encrypt --dry-run
```

### sops secrets-decrypt

Decrypt secrets with automatic backup creation.

```bash
openCenter sops secrets-decrypt [flags]
```

**Flags:**
- `--search-path string`: Path to search for files (default: ".")
- `--backups`: Create backups before decryption (default: true)
- `--dry-run`: Show what would be done without making changes

**Examples:**

```bash
# Decrypt all secrets
openCenter sops secrets-decrypt

# Decrypt without backups (faster)
openCenter sops secrets-decrypt --backups=false

# Dry-run mode
openCenter sops secrets-decrypt --dry-run
```

## Config Commands

### config features

Display feature flag status and manage gradual migration to refactored systems.

```bash
openCenter config features [flags]
```

**Flags:**
- `--output, -o string`: Output format: json, table, or env (default: "table")

**Examples:**

```bash
# Display feature status (default table format)
openCenter config features

# Display as JSON for scripting
openCenter config features --output json

# Generate environment variable exports
openCenter config features --output env

# Save to file for sourcing
openCenter config features --output env > feature-flags.sh
source feature-flags.sh
```

**Available Feature Flags:**
- `OPENCENTER_USE_NEW_TEMPLATE_ENGINE`: Enhanced template engine with caching
- `OPENCENTER_USE_PIPELINE_GENERATOR`: Pipeline-based GitOps generation
- `OPENCENTER_USE_NEW_CONFIG_BUILDER`: Type-safe configuration builder
- `OPENCENTER_USE_SERVICE_REGISTRY`: Plugin-based service registry
- `OPENCENTER_ENABLE_ALL_NEW_FEATURES`: Enable all new features at once
- `OPENCENTER_FEATURE_FLAG_DEBUG`: Enable debug logging for feature flags

**Setting Feature Flags:**

```bash
# Enable a specific feature
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true

# Enable all new features
export OPENCENTER_ENABLE_ALL_NEW_FEATURES=true

# Enable for a single command
OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true openCenter cluster setup my-cluster

# Enable debug logging
export OPENCENTER_FEATURE_FLAG_DEBUG=true
```

**Valid Values:** `true`, `1`, `yes`, `on` (case-insensitive) to enable; `false`, `0`, `no`, `off`, or unset to disable

See [config features documentation](config/features.md) for detailed information.

### config ide

Generate IDE configuration files for enhanced development experience.

```bash
openCenter config ide [flags]
```

**Flags:**
- `--vscode`: Generate VS Code settings
- `--jetbrains`: Generate JetBrains IDE settings
- `--all`: Generate all IDE configurations

**Examples:**

```bash
# Generate VS Code config
openCenter config ide --vscode

# Generate all IDE configs
openCenter config ide --all
```

**Generated Files:**
- `.vscode/settings.json`: VS Code YAML schema association
- `.idea/jsonSchemas.xml`: JetBrains schema configuration

## Version Command

### version

Display version and build information.

```bash
openCenter version [flags]
```

**Flags:**
- `--short`: Display short version only

**Examples:**

```bash
# Show full version information
openCenter version

# Show short version only
openCenter version --short

# Alternative: use --version flag
openCenter --version
```

**Output (Full):**
```
openCenter version: 0.0.1-3ddfb1c
Git commit:         3ddfb1c764b3aa4cc481cdb2a56ab0fea5a2a47d
Git branch:         main
Build date:         2025-11-07T20:33:55Z
Go version:         go1.25.2
Platform:           darwin/amd64
```

**Output (Short):**
```
0.0.1-3ddfb1c
```

**Version String Format:**
- If built from a git tag: Uses the tag as version (e.g., `1.0.0`)
- If not from a tag: Uses `version-shortcommit` format (e.g., `0.0.1-3ddfb1c`)
- Development builds: Shows `dev-shortcommit`

## Plugins Commands

### plugins list

List all available plugins.

```bash
openCenter plugins list
```

**Output:**
- Plugin name
- Version
- Description
- Status (enabled/disabled)

### plugins install

Install a plugin from a repository.

```bash
openCenter plugins install <name> [flags]
```

**Flags:**
- `--source string`: Plugin source URL or path
- `--version string`: Plugin version to install

**Examples:**

```bash
# Install from default repository
openCenter plugins install my-plugin

# Install from custom source
openCenter plugins install my-plugin --source=https://github.com/org/plugin

# Install specific version
openCenter plugins install my-plugin --version=1.0.0
```

### plugins remove

Remove an installed plugin.

```bash
openCenter plugins remove <name>
```

**Example:**

```bash
openCenter plugins remove my-plugin
```

## Environment Variables

### General Configuration
- `OPENCENTER_CONFIG_DIR`: Override default config directory
- `OPENCENTER_DEBUG`: Enable debug logging and artifacts
- `OPENCENTER_PLUGINS_DIR`: Directory for external plugins
- `KIND_EXPERIMENTAL_PROVIDER`: Set to "podman" for Podman support
- `CONTAINER_RUNTIME`: Set to "podman" if using Podman instead of Docker

### Feature Flags

Feature flags enable gradual migration to refactored systems while maintaining backward compatibility.

#### Individual Feature Flags
- `OPENCENTER_USE_NEW_TEMPLATE_ENGINE`: Enable enhanced template engine with caching and better error messages
- `OPENCENTER_USE_PIPELINE_GENERATOR`: Enable pipeline-based GitOps generation with rollback support
- `OPENCENTER_USE_NEW_CONFIG_BUILDER`: Enable type-safe fluent configuration builder
- `OPENCENTER_USE_SERVICE_REGISTRY`: Enable plugin-based service registry with dependency resolution

#### Global Feature Flags
- `OPENCENTER_ENABLE_ALL_NEW_FEATURES`: Enable all new features at once (overrides individual flags)
- `OPENCENTER_FEATURE_FLAG_DEBUG`: Enable debug logging for feature flag evaluation

#### Valid Values
- **Enable**: `true`, `1`, `yes`, `on` (case-insensitive)
- **Disable**: `false`, `0`, `no`, `off`, or unset

#### Examples

```bash
# Enable specific feature
export OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true

# Enable all new features
export OPENCENTER_ENABLE_ALL_NEW_FEATURES=true

# Enable for single command
OPENCENTER_USE_NEW_TEMPLATE_ENGINE=true openCenter cluster setup my-cluster

# Check feature status
openCenter config features

# Enable debug logging
export OPENCENTER_FEATURE_FLAG_DEBUG=true
openCenter config features
```

See `openCenter config features --help` for more information.

## Exit Codes

- `0`: Success
- `1`: General error
- `2`: Configuration error
- `3`: Validation error
- `4`: Network error
- `5`: Provider error

## See Also

- [Configuration Reference](configuration.md)
- [Schema Reference](schema.md)
- [How-To Guides](../how-to/)
- [Tutorials](../tutorials/)
