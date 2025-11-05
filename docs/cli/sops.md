# SOPS CLI Reference

The `openCenter sops` command group provides comprehensive SOPS (Secrets OPerationS) key management and automation for secure GitOps workflows. These commands handle the full lifecycle of Age encryption keys used for encrypting sensitive data in Kubernetes deployments.

## Overview

SOPS integration in openCenter enables:
- **Secure Secret Management**: Encrypt sensitive configuration files using Age encryption
- **Key Lifecycle Management**: Generate, rotate, backup, and validate encryption keys
- **GitOps Integration**: Seamlessly integrate encrypted secrets into GitOps workflows
- **Multi-Provider Support**: Handle provider-specific secret patterns (OpenStack, vSphere, etc.)

## Global Usage

```bash
openCenter sops [command] [flags]
```

**Available Commands:**
- `generate-key` - Generate new Age key pair for SOPS encryption
- `rotate-key` - Rotate Age keys and re-encrypt existing secrets
- `backup-key` - Create backup of Age keys and SOPS configuration
- `validate` - Validate Age key configuration and SOPS setup

---

## `openCenter sops generate-key`

Generates a new Age key pair for SOPS encryption with secure file permissions and optional SOPS configuration updates.

### Usage

```bash
openCenter sops generate-key [flags]
```

### Flags

| Flag | Description | Default |
| --- | --- | --- |
| `--key-file <path>` | Path to save the Age key file | `~/.config/sops/age/keys.txt` |
| `--update-sops-config` | Update .sops.yaml configuration with new public key | `true` |

### Behavior

1. **Key Generation**: Creates a cryptographically secure Age key pair
2. **Secure Storage**: Saves private key with 600 permissions, public key with 644 permissions
3. **Backup Protection**: Automatically backs up existing keys before overwriting
4. **SOPS Integration**: Optionally updates `.sops.yaml` configuration file
5. **Directory Management**: Creates key directories if they don't exist

### Examples

```bash
# Generate a new key with default settings
openCenter sops generate-key

# Generate a key with custom path
openCenter sops generate-key --key-file ~/.config/sops/age/my-cluster-key

# Generate a key without updating SOPS config
openCenter sops generate-key --update-sops-config=false

# Generate a cluster-specific key
openCenter sops generate-key --key-file ~/.config/sops/age/production-key
```

### Output

```
🔑 Generating new Age key pair for SOPS encryption...
✅ Age key pair generated successfully!
📁 Private key: /Users/user/.config/sops/age/keys.txt
🔑 Public key: age1q9dns6ylmkzsfhx4fvwjynr0k5k0qtlswnpn264750zmay764d4qyk0760
✅ Updated .sops.yaml configuration
```

---

## `openCenter sops rotate-key`

Rotates Age encryption keys and re-encrypts existing SOPS files with the new key. This is essential for maintaining security through regular key rotation.

### Usage

```bash
openCenter sops rotate-key [flags]
```

### Flags

| Flag | Description | Default |
| --- | --- | --- |
| `--key-file <path>` | Path to Age key file | `~/.config/sops/age/keys.txt` |
| `--search-path <path>` | Path to search for SOPS files to re-encrypt | `.` |
| `--dry-run` | Show what would be done without making changes | `false` |

### Rotation Process

1. **Backup Creation**: Backs up the existing Age key with timestamp
2. **Key Generation**: Generates a new Age key pair
3. **File Discovery**: Searches for SOPS-encrypted files in the specified path
4. **Re-encryption**: Re-encrypts each file with the new key
5. **Configuration Update**: Updates `.sops.yaml` with the new public key
6. **Rollback Protection**: Restores old key if any step fails

### Examples

```bash
# Rotate the default key and re-encrypt files in current directory
openCenter sops rotate-key

# Dry run to see what files would be affected
openCenter sops rotate-key --dry-run --search-path ./secrets

# Rotate a specific key file
openCenter sops rotate-key --key-file ~/.config/sops/age/production-key

# Rotate and search in a specific GitOps repository
openCenter sops rotate-key --search-path ./gitops-repo
```

### Output

```
🔄 Starting Age key rotation...
📁 Key name: keys
🔍 Search path: .
✅ Old key backed up as: keys-backup-20241029-120245
🔄 Re-encrypting: ./secrets/database-credentials.yaml
🔄 Re-encrypting: ./secrets/api-tokens.yaml
✅ Age key rotation completed successfully!
🔑 New public key: age1new9dns6ylmkzsfhx4fvwjynr0k5k0qtlswnpn264750zmay764d4q
✅ Updated .sops.yaml configuration
```

---

## `openCenter sops backup-key`

Creates secure backups of Age keys and SOPS configuration files with timestamps for disaster recovery.

### Usage

```bash
openCenter sops backup-key [flags]
```

### Flags

| Flag | Description | Default |
| --- | --- | --- |
| `--key-file <path>` | Path to Age key file | `~/.config/sops/age/keys.txt` |
| `--backup-dir <path>` | Backup directory | `~/.config/sops/age/backups` |

### Backup Contents

- **Age Private Keys**: All private key files with secure permissions
- **Age Public Keys**: All public key files
- **SOPS Configuration**: `.sops.yaml` file if present
- **Metadata**: Timestamped backup organization

### Examples

```bash
# Create backup with default settings
openCenter sops backup-key

# Backup to a specific directory
openCenter sops backup-key --backup-dir /secure/backup/location

# Backup specific key file
openCenter sops backup-key --key-file ~/.config/sops/age/production-key

# Create backup before key rotation
openCenter sops backup-key --backup-dir ./pre-rotation-backup
```

### Output

```
💾 Creating Age key backup...
✅ Age key backup created successfully!
📁 Backup directory: /Users/user/.config/sops/age/backups
✅ SOPS configuration backed up to: /Users/user/.config/sops/age/backups/sops-config-20241029-120245.yaml
```

---

## `openCenter sops validate`

Performs comprehensive validation of Age key configuration and SOPS setup to ensure proper functionality.

### Usage

```bash
openCenter sops validate [flags]
```

### Flags

| Flag | Description | Default |
| --- | --- | --- |
| `--key-file <path>` | Path to Age key file | `~/.config/sops/age/keys.txt` |
| `--config-file <path>` | Path to SOPS configuration file | `.sops.yaml` |

### Validation Checks

1. **Key File Existence**: Verifies Age key files exist and are readable
2. **Key Format Validation**: Validates Age key format and structure
3. **Permission Checks**: Ensures proper file permissions (600 for private keys)
4. **SOPS Configuration**: Validates `.sops.yaml` configuration syntax
5. **Key Matching**: Verifies public key matches configuration
6. **Functionality Testing**: Tests encryption/decryption operations
7. **Tool Installation**: Checks SOPS binary installation and version

### Examples

```bash
# Validate default configuration
openCenter sops validate

# Validate specific key file
openCenter sops validate --key-file ~/.config/sops/age/production-key

# Validate with custom SOPS config
openCenter sops validate --config-file ./custom-sops.yaml

# Validate cluster-specific setup
openCenter sops validate --key-file ~/.config/sops/age/cluster-prod-key --config-file ./clusters/prod/.sops.yaml
```

### Output (Success)

```
🔍 Validating SOPS configuration...
📁 Key name: keys
📄 Config file: .sops.yaml
✅ Age key validation passed
🔑 Public key: age1q9dns6ylmkzsfhx4fvwjynr0k5k0qtlswnpn264750zmay764d4qyk0760
✅ SOPS configuration contains current Age public key
🧪 Testing key access...
✅ Key access test passed
✅ SOPS is installed: sops 3.11.0 (latest)
✅ All validations completed successfully!
```

### Output (Issues Found)

```
🔍 Validating SOPS configuration...
📁 Key name: keys
📄 Config file: .sops.yaml
❌ Age key file not found: /Users/user/.config/sops/age/keys.txt
⚠️  SOPS configuration file not found: .sops.yaml
⚠️  SOPS not found or not executable: exec: "sops": executable file not found in $PATH
```

---

## Integration with openCenter Workflows

### Cluster Configuration

SOPS keys integrate with openCenter cluster configurations through the `secrets` section:

```yaml
secrets:
  sops_age_key_file: ~/.config/sops/age/keys/my-cluster-key.txt
```

### GitOps Repository Setup

When setting up GitOps repositories, SOPS configuration is automatically generated:

```bash
# Initialize cluster with SOPS key generation
openCenter cluster init my-cluster --opencenter.gitops.git_dir=./gitops-repo

# Generate SOPS key for the cluster
openCenter sops generate-key --key-file ~/.config/sops/age/my-cluster-key

# Set up GitOps repository with SOPS integration
openCenter cluster setup --render
```

### Automated Workflows

```bash
# Complete SOPS setup workflow
openCenter sops generate-key --key-file ~/.config/sops/age/production-key
openCenter cluster init production --secrets.sops_age_key_file=~/.config/sops/age/production-key.txt
openCenter cluster setup --render
openCenter sops validate --key-file ~/.config/sops/age/production-key
```

---

## Security Best Practices

### Key Management

1. **Regular Rotation**: Rotate keys periodically using `rotate-key`
2. **Secure Backups**: Create regular backups with `backup-key`
3. **Access Control**: Ensure proper file permissions (600 for private keys)
4. **Separation**: Use different keys for different environments/clusters

### File Organization

```
~/.config/sops/age/
├── keys/
│   ├── production-key.txt      # Production cluster key
│   ├── staging-key.txt         # Staging cluster key
│   └── development-key.txt     # Development cluster key
├── backups/
│   ├── production-key-backup-20241029-120000.txt
│   └── sops-config-20241029-120000.yaml
└── keys.txt                    # Default key
```

### Environment-Specific Configuration

```yaml
# Production .sops.yaml
creation_rules:
  - path_regex: .*\.(yaml|yml)$
    age: age1prod9dns6ylmkzsfhx4fvwjynr0k5k0qtlswnpn264750zmay764d4q
    encrypted_regex: '^(data|stringData|password|token|key|secret|credentials)'

# Development .sops.yaml  
creation_rules:
  - path_regex: .*\.(yaml|yml)$
    age: age1dev9dns6ylmkzsfhx4fvwjynr0k5k0qtlswnpn264750zmay764d4q
    encrypted_regex: '^(data|stringData)'
```

---

## Troubleshooting

### Common Issues

**Key Not Found**
```bash
# Check key location
ls -la ~/.config/sops/age/
# Generate new key if missing
openCenter sops generate-key
```

**Permission Denied**
```bash
# Fix key permissions
chmod 600 ~/.config/sops/age/keys.txt
chmod 644 ~/.config/sops/age/keys.pub
```

**SOPS Not Installed**
```bash
# Install SOPS (macOS)
brew install sops
# Install SOPS (Linux)
curl -LO https://github.com/mozilla/sops/releases/latest/download/sops-v3.8.1.linux.amd64
```

**Configuration Mismatch**
```bash
# Validate configuration
openCenter sops validate
# Update configuration with current key
openCenter sops generate-key --update-sops-config=true
```

### Validation Workflow

```bash
# Complete validation workflow
openCenter sops validate                    # Check current setup
openCenter sops backup-key                  # Create backup
openCenter sops generate-key --dry-run      # Test key generation
openCenter sops rotate-key --dry-run        # Test rotation
```

---

## Related Commands


- `openCenter cluster init` - Initialize cluster with SOPS key generation
- `openCenter cluster setup` - Set up GitOps repository with SOPS integration
- `openCenter cluster validate` - Validate cluster configuration including SOPS setup

---

## Environment Variables

| Variable | Description | Default |
| --- | --- | --- |
| `SOPS_AGE_KEY_FILE` | Path to Age key file for SOPS operations | Set by key manager |
| `OPENCENTER_CONFIG_DIR` | Configuration directory for openCenter | `~/.config/openCenter` |

---

## File Locations

| File | Purpose | Permissions |
| --- | --- | --- |
| `~/.config/sops/age/keys.txt` | Default Age private key | 600 |
| `~/.config/sops/age/keys.pub` | Default Age public key | 644 |
| `~/.config/sops/age/backups/` | Key backup directory | 700 |
| `.sops.yaml` | SOPS configuration file | 644 |

For more information about SOPS itself, see the [official SOPS documentation](https://github.com/mozilla/sops).