---
title: Secrets Management Reference
doc_type: reference
category: reference
weight: 20
---

# Secrets Management Reference


## Table of Contents

- [Overview](#overview)
- [Configuration Structure](#configuration-structure)
- [SOPS Configuration](#sops-configuration)
- [Age Encryption Keys](#age-encryption-keys)
- [Secret Types](#secret-types)
- [SSH Keys](#ssh-keys)
- [Encryption Operations](#encryption-operations)
- [Environment Variables](#environment-variables)
- [Security Best Practices](#security-best-practices)
- [Troubleshooting](#troubleshooting)
- [See Also](#see-also)
This document provides complete reference information for secrets management in openCenter CLI, including SOPS configuration, Age encryption keys, and secrets organization.

## Overview

openCenter CLI uses [SOPS](https://github.com/mozilla/sops) (Secrets OPerationS) with [Age](https://age-encryption.org/) encryption for managing sensitive data. All secrets are encrypted at rest and only decrypted when needed by the cluster.

### Key Concepts

- **SOPS**: Mozilla's tool for encrypting files with multiple key types
- **Age**: Modern, simple encryption tool used as the encryption backend
- **Age Keys**: Public/private key pairs for encryption/decryption
- **SOPS Configuration**: `.sops.yaml` files that define encryption rules

---

## Configuration Structure

### Secrets Section

The `secrets` section in the cluster configuration defines all secret values and encryption settings.

```yaml
secrets:
  # SOPS Age key file path
  sops_age_key_file: ~/.config/sops/age/my-cluster-key.txt
  
  # SSH key configuration
  ssh_key:
    private: ~/.ssh/my-cluster-ed25519
    public: ~/.ssh/my-cluster-ed25519.pub
    cypher: ed25519
  
  # Global secrets organized by scope
  global:
    aws:
      infrastructure:
        access_key: AKIAIOSFODNN7EXAMPLE
        secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
        region: us-east-1
      application:
        access_key: AKIAIOSFODNN7EXAMPLE
        secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
        region: us-east-1
  
  # Service-specific secrets
  cert_manager:
    aws_access_key: AKIAIOSFODNN7EXAMPLE
    aws_secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
  
  loki:
    swift_password: secret-password
    swift_application_credential_secret: secret-credential
    s3_access_key_id: AKIAIOSFODNN7EXAMPLE
    s3_secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
  
  keycloak:
    client_secret: secret-client-secret
    admin_password: secret-admin-password
  
  headlamp:
    oidc_client_secret: secret-oidc-client
  
  weave_gitops:
    password: secret-password
    password_hash: $2a$10$...
  
  grafana:
    admin_password: secret-admin-password
  
  tempo:
    access_key: AKIAIOSFODNN7EXAMPLE
    secret_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
  
  alert_proxy:
    core_device_id: device-id
    account_service_token: service-token
    core_account_number: account-number
  
  vsphere_csi:
    vcenter_host: vcenter.example.com
    username: administrator@vsphere.local
    password: secret-password
    datacenters: Datacenter1,Datacenter2
    insecure_flag: "false"
    port: "443"
```

---

## SOPS Configuration

### .sops.yaml Structure

The `.sops.yaml` file defines encryption rules for files in the GitOps repository.

```yaml
# SOPS configuration for cluster: my-cluster
creation_rules:
  # Default rule for all YAML files
  - path_regex: .*\.(yaml|yml)$
    age: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    encrypted_regex: '^(data|stringData|password|token|key|secret|credentials)'
  
  # OpenStack credentials
  - path_regex: secrets/openstack-credentials\.yaml$
    age: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
  
  # vSphere credentials
  - path_regex: secrets/vsphere-credentials\.yaml$
    age: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
  
  # Service secrets
  - path_regex: customer-managed/services/.*/secret\.yaml$
    age: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### Configuration Fields

#### creation_rules

Array of rules that define how files should be encrypted.

**Fields:**
- `path_regex` (string, required): Regular expression matching file paths
- `age` (string, required): Age public key for encryption
- `encrypted_regex` (string, optional): Regex matching fields to encrypt
- `unencrypted_regex` (string, optional): Regex matching fields to leave unencrypted
- `unencrypted_suffix` (string, optional): Suffix for unencrypted fields (default: `_unencrypted`)

#### encrypted_regex

Defines which fields in YAML files should be encrypted. Common patterns:

```regex
# Encrypt data, stringData, and secret-related fields
^(data|stringData|password|token|key|secret|credentials)

# Encrypt only data and stringData (Kubernetes secrets)
^(data|stringData)

# Encrypt all fields except metadata
^(?!metadata)
```

#### path_regex

Matches file paths for applying encryption rules. Examples:

```regex
# All YAML files
.*\.(yaml|yml)$

# Specific directory
secrets/.*\.yaml$

# Specific file
infrastructure/clusters/my-cluster/secrets\.yaml$

# Multiple directories
(secrets|customer-managed)/.*\.yaml$
```

---

## Age Encryption Keys

### Key Generation

Age keys are generated automatically during cluster initialization or can be created manually.

#### Automatic Generation

```bash
# During cluster init
openCenter cluster init my-cluster

# Generates key at: ~/.config/sops/age/my-cluster-key.txt
```

#### Manual Generation

```bash
# Generate Age key
age-keygen -o ~/.config/sops/age/my-cluster-key.txt

# View public key
age-keygen -y ~/.config/sops/age/my-cluster-key.txt
```

### Key Format

Age keys are text files containing the private key and comments.

```
# created: 2024-01-15T10:30:00Z
# public key: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
AGE-SECRET-KEY-1XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX
```

**Components:**
- **Comment lines**: Start with `#`, contain metadata
- **Public key**: Derived from private key, used for encryption
- **Private key**: Starts with `AGE-SECRET-KEY-1`, used for decryption

### Key Storage Locations

#### Default Locations

```
~/.config/sops/age/           # Default SOPS Age key directory
~/.config/openCenter/clusters/ # Cluster-specific keys
```

#### Organization-Based Structure

```
~/.config/openCenter/clusters/
└── <organization>/
    └── secrets/
        └── age/
            └── keys/
                └── <cluster>-key.txt
```

#### Legacy Structure

```
~/.config/openCenter/clusters/
└── <cluster>/
    └── secrets/
        └── age/
            └── keys/
                └── <cluster>-key.txt
```

### Key Permissions

Age keys must have restrictive permissions to prevent unauthorized access.

```bash
# Set correct permissions
chmod 600 ~/.config/sops/age/my-cluster-key.txt

# Verify permissions
ls -l ~/.config/sops/age/my-cluster-key.txt
# Should show: -rw------- (600)
```

---

## Secret Types

### Global Secrets

Global secrets provide fallback credentials for multiple services.

#### AWS Global Secrets

```yaml
secrets:
  global:
    aws:
      infrastructure:
        access_key: AKIAIOSFODNN7EXAMPLE
        secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
        region: us-east-1
      application:
        access_key: AKIAIOSFODNN7EXAMPLE
        secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
        region: us-east-1
```

**Purpose:**
- `infrastructure`: Used for OpenTofu S3 backend, infrastructure provisioning
- `application`: Used for application services (cert-manager, loki, tempo)

**Fallback Order:**
1. Service-specific secrets (e.g., `secrets.cert_manager.aws_access_key`)
2. Application-level global secrets (`secrets.global.aws.application`)
3. Infrastructure-level global secrets (`secrets.global.aws.infrastructure`)

### Service-Specific Secrets

#### Cert-Manager

```yaml
secrets:
  cert_manager:
    aws_access_key: AKIAIOSFODNN7EXAMPLE
    aws_secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

**Required when:** `services.cert-manager.enabled: true`

**Purpose:** Route53 DNS validation for Let's Encrypt certificates

#### Loki

```yaml
secrets:
  loki:
    # Swift storage (legacy)
    swift_password: secret-password
    swift_application_credential_secret: secret-credential
    
    # S3 storage (recommended)
    s3_access_key_id: AKIAIOSFODNN7EXAMPLE
    s3_secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

**Required when:** `services.loki.enabled: true`

**Purpose:** Object storage for log aggregation

**Storage Options:**
- Swift: OpenStack object storage (legacy)
- S3: AWS S3 or S3-compatible storage (recommended)

#### Keycloak

```yaml
secrets:
  keycloak:
    client_secret: secret-client-secret
    admin_password: secret-admin-password
```

**Required when:** `services.keycloak.enabled: true`

**Purpose:**
- `client_secret`: OIDC client secret for Kubernetes authentication
- `admin_password`: Keycloak admin console password

#### Headlamp

```yaml
secrets:
  headlamp:
    oidc_client_secret: secret-oidc-client
```

**Required when:** `services.headlamp.enabled: true`

**Purpose:** OIDC authentication for Headlamp dashboard

#### Weave GitOps

```yaml
secrets:
  weave_gitops:
    password: secret-password
    password_hash: $2a$10$...
```

**Required when:** `services.weave-gitops.enabled: true`

**Purpose:** Admin authentication for Weave GitOps UI

**Password Hash:** Generate with `htpasswd -nbBC 10 admin <password>`

#### Grafana

```yaml
secrets:
  grafana:
    admin_password: secret-admin-password
```

**Required when:** `services.kube-prometheus-stack.enabled: true`

**Purpose:** Grafana admin console password

#### Tempo

```yaml
secrets:
  tempo:
    access_key: AKIAIOSFODNN7EXAMPLE
    secret_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

**Required when:** `services.tempo.enabled: true`

**Purpose:** S3 storage for distributed tracing data

#### Alert Proxy

```yaml
secrets:
  alert_proxy:
    core_device_id: device-id
    account_service_token: service-token
    core_account_number: account-number
```

**Required when:** `managed-service.alert-proxy.enabled: true`

**Purpose:** Rackspace alert proxy integration

#### vSphere CSI

```yaml
secrets:
  vsphere_csi:
    vcenter_host: vcenter.example.com
    username: administrator@vsphere.local
    password: secret-password
    datacenters: Datacenter1,Datacenter2
    insecure_flag: "false"
    port: "443"
```

**Required when:** `services.vsphere-csi.enabled: true`

**Purpose:** vSphere storage integration

---

## SSH Keys

### Configuration

```yaml
secrets:
  ssh_key:
    private: ~/.ssh/my-cluster-ed25519
    public: ~/.ssh/my-cluster-ed25519.pub
    cypher: ed25519
```

### Supported Cyphers

- `ed25519` (recommended): Modern, secure, fast
- `rsa`: Traditional, widely supported (minimum 2048 bits)
- `ecdsa`: Elliptic curve, good security

### Generation

```bash
# Ed25519 (recommended)
ssh-keygen -t ed25519 -f ~/.ssh/my-cluster-ed25519 -C "my-cluster"

# RSA (4096 bits)
ssh-keygen -t rsa -b 4096 -f ~/.ssh/my-cluster-rsa -C "my-cluster"

# ECDSA
ssh-keygen -t ecdsa -b 521 -f ~/.ssh/my-cluster-ecdsa -C "my-cluster"
```

### Usage

SSH keys are used for:
- Node access during provisioning
- Ansible configuration management
- GitOps repository authentication (when using SSH URLs)

---

## Encryption Operations

### Encrypting Files

#### Manual Encryption

```bash
# Encrypt a file in-place
sops --encrypt --age <public-key> --in-place secrets.yaml

# Encrypt to a new file
sops --encrypt --age <public-key> secrets.yaml > secrets.enc.yaml

# Encrypt with .sops.yaml rules
sops --encrypt --in-place secrets.yaml
```

#### Programmatic Encryption

```go
import (
    "context"
    "github.com/rackerlabs/openCenter-cli/internal/sops"
)

manager := sops.NewSOPSManager()
ctx := context.Background()

config := sops.EncryptionConfig{
    AgeKeys: []string{"age1xxx..."},
    InPlace: true,
    Verbose: true,
}

err := manager.GetEncryptor().EncryptFile(ctx, "secrets.yaml", config)
```

### Decrypting Files

#### Manual Decryption

```bash
# Decrypt to stdout
sops --decrypt secrets.yaml

# Decrypt to a file
sops --decrypt secrets.yaml > secrets-plain.yaml

# Decrypt in-place
sops --decrypt --in-place secrets.yaml
```

#### Programmatic Decryption

```go
import (
    "context"
    "github.com/rackerlabs/openCenter-cli/internal/sops"
)

manager := sops.NewSOPSManager()
ctx := context.Background()

data, err := manager.GetEncryptor().DecryptFile(ctx, "secrets.yaml")
```

### Editing Encrypted Files

```bash
# Edit with default editor
sops secrets.yaml

# Edit with specific editor
EDITOR=vim sops secrets.yaml

# Edit specific key
sops --set '["data"]["password"] "new-password"' secrets.yaml
```

---

## Environment Variables

### SOPS Environment Variables

```bash
# Age key file location
export SOPS_AGE_KEY_FILE=~/.config/sops/age/my-cluster-key.txt

# Age key (alternative to file)
export SOPS_AGE_KEY=AGE-SECRET-KEY-1XXXXX...

# Default editor for sops edit
export EDITOR=vim

# SOPS configuration file
export SOPS_CONFIG=.sops.yaml
```

### openCenter Environment Variables

```bash
# Configuration directory
export OPENCENTER_CONFIG_DIR=~/.config/openCenter

# Test mode (uses placeholder credentials)
export OPENCENTER_TEST_MODE=true

# Debug mode (saves complete config)
export OPENCENTER_DEBUG=true
```

---

## Security Best Practices

### Key Management

1. **Generate unique keys per cluster**
   ```bash
   age-keygen -o ~/.config/sops/age/${CLUSTER_NAME}-key.txt
   ```

2. **Use restrictive permissions**
   ```bash
   chmod 600 ~/.config/sops/age/*.txt
   ```

3. **Back up keys securely**
   - Store in password manager
   - Use encrypted backup storage
   - Never commit to version control

4. **Rotate keys periodically**
   ```bash
   # Generate new key
   age-keygen -o ~/.config/sops/age/${CLUSTER_NAME}-key-new.txt
   
   # Re-encrypt with new key
   sops rotate --age $(age-keygen -y ~/.config/sops/age/${CLUSTER_NAME}-key-new.txt) secrets.yaml
   ```

### Secret Values

1. **Use strong passwords**
   - Minimum 16 characters
   - Mix of uppercase, lowercase, numbers, symbols
   - Use password generator

2. **Avoid plaintext secrets**
   - Never commit unencrypted secrets
   - Use environment variables for local development
   - Encrypt before committing to Git

3. **Limit secret scope**
   - Use service-specific secrets when possible
   - Avoid reusing secrets across services
   - Rotate secrets regularly

4. **Audit secret access**
   - Review who has access to Age keys
   - Monitor secret decryption operations
   - Use separate keys for different environments

### File Encryption

1. **Encrypt sensitive files**
   ```yaml
   # Files that should always be encrypted:
   - secrets/*.yaml
   - */secrets/*.yaml
   - *-credentials.yaml
   - */secret.yaml
   ```

2. **Use encrypted_regex**
   ```yaml
   creation_rules:
     - path_regex: .*\.yaml$
       encrypted_regex: '^(data|stringData|password|token|key|secret|credentials)'
   ```

3. **Verify encryption**
   ```bash
   # Check if file is encrypted
   grep -q "sops:" secrets.yaml && echo "Encrypted" || echo "Not encrypted"
   
   # Verify with SOPS
   sops --decrypt secrets.yaml > /dev/null && echo "Valid" || echo "Invalid"
   ```

---

## Troubleshooting

### Common Issues

#### Age Key Not Found

**Error:**
```
error decrypting key: no age key found
```

**Solutions:**
1. Set `SOPS_AGE_KEY_FILE` environment variable
2. Verify key file exists and has correct permissions
3. Check key file format (should start with `AGE-SECRET-KEY-1`)

#### Permission Denied

**Error:**
```
permission denied: ~/.config/sops/age/key.txt
```

**Solutions:**
```bash
# Fix permissions
chmod 600 ~/.config/sops/age/key.txt

# Fix directory permissions
chmod 700 ~/.config/sops/age
```

#### Invalid Key Format

**Error:**
```
invalid age key format
```

**Solutions:**
1. Regenerate key with `age-keygen`
2. Verify key file is not corrupted
3. Check for extra whitespace or newlines

#### SOPS Not Found

**Error:**
```
sops: command not found
```

**Solutions:**
```bash
# macOS
brew install sops

# Linux
# Download from https://github.com/mozilla/sops/releases

# Verify installation
sops --version
```

### Validation

#### Check SOPS Configuration

```bash
# Validate .sops.yaml syntax
sops --config .sops.yaml --show-master-keys secrets.yaml

# Test encryption rules
sops --config .sops.yaml --encrypt --in-place test-secret.yaml
```

#### Verify Age Key

```bash
# Extract public key
age-keygen -y ~/.config/sops/age/my-cluster-key.txt

# Test encryption/decryption
echo "test" | age -r $(age-keygen -y ~/.config/sops/age/my-cluster-key.txt) | age -d -i ~/.config/sops/age/my-cluster-key.txt
```

#### Check Encrypted Files

```bash
# List encrypted files
find . -name "*.yaml" -exec grep -l "sops:" {} \;

# Verify all encrypted files can be decrypted
find . -name "*.yaml" -exec grep -l "sops:" {} \; | while read f; do
  sops --decrypt "$f" > /dev/null && echo "✓ $f" || echo "✗ $f"
done
```

---

## See Also

- [Configuration Reference](configuration.md) - Complete configuration schema
- [API Reference](api.md) - Go package documentation
- [Template System Reference](templates.md) - Template functions
- [SOPS Documentation](https://github.com/mozilla/sops) - Official SOPS docs
- [Age Documentation](https://age-encryption.org/) - Official Age docs
