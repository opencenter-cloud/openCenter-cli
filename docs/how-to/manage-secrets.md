---
id: manage-secrets
title: "Manage Secrets"
sidebar_label: Manage Secrets
description: How to encrypt, rotate, and manage secrets with SOPS and Age encryption.
doc_type: how-to
audience: "operators, platform engineers"
tags: [secrets, sops, age, encryption, rotation]
---

# Manage Secrets

**Purpose:** For operators, shows how to encrypt, rotate, and manage secrets with SOPS and Age encryption, covering key generation through rotation.

openCenter uses SOPS with Age encryption to secure sensitive data in Git repositories. This guide shows you how to manage encryption keys and secrets throughout their lifecycle.

## Prerequisites

- openCenter CLI installed
- Cluster configuration created
- Git repository for GitOps (for encrypted secrets)

## Generate Age Encryption Key

Create a new Age key pair for encrypting secrets:

```bash
opencenter secrets keys generate
```

This creates:
- Private key: `~/.config/sops/age/keys.txt` by default
- Public key: Embedded in private key file
- SOPS configuration: `.sops.yaml` in the current directory when enabled

Key format:
```
# created: 2026-02-17T10:30:00Z
# public key: age1abc123...
AGE-SECRET-KEY-1ABC123...
```

### Custom Key Location

Specify custom key file location:

```bash
opencenter secrets keys generate --key-file /secure/location/my-key.txt
```

### Skip SOPS Configuration Updates

Generate only the Age key file:

```bash
opencenter secrets keys generate --key-file /secure/location/my-key.txt --update-sops-config=false
```

## Validate SOPS Configuration

Check that SOPS is properly configured:

```bash
opencenter secrets keys validate
```

This validates:
- Age key file exists and is readable
- Key format is valid
- SOPS configuration (`.sops.yaml`) is correct
- Encryption rules are properly defined

Expected output:
```
✓ Age key file found
✓ Key format valid
✓ SOPS configuration valid
✓ Encryption rules defined

SOPS configuration is valid
```

### Validate Specific Key File

```bash
opencenter secrets keys validate --key-file /path/to/key.txt
```

### Validate Secret Drift

Compare a cluster config with encrypted manifests:

```bash
opencenter secrets validate my-cluster
```

## Encrypt Secrets

Encrypt sensitive files in your GitOps repository:

```bash
opencenter secrets encrypt --path applications/overlays/my-cluster
```

This encrypts:
- `flux-system/gotk-sync.yaml` - FluxCD sync configuration
- `managed-services/sources/base-repo.yaml` - GitRepository sources
- `secrets/*.yaml` - Provider credentials
- Service-specific secrets (Keycloak, Grafana, etc.)

### Encrypt Specific File

```bash
opencenter secrets encrypt --path applications/overlays/my-cluster/secrets
```

### Fast Parallel Encryption

Encrypt multiple files in parallel:

```bash
opencenter secrets encrypt --path applications/overlays/my-cluster
```

This uses 4 parallel workers for faster encryption of large repositories.

## Decrypt Secrets

Decrypt secrets for viewing or editing:

```bash
opencenter secrets decrypt --path applications/overlays/my-cluster
```

### Decrypt Specific File

```bash
opencenter secrets decrypt --path applications/overlays/my-cluster/secrets
```

### Fast Parallel Decryption

```bash
opencenter secrets decrypt --path applications/overlays/my-cluster
```

## List Encrypted Secrets

See all encrypted files in repository:

```bash
opencenter secrets list
```

Output shows:
```
Encrypted secrets in repository:

applications/overlays/my-cluster/flux-system/gotk-sync.yaml
applications/overlays/my-cluster/secrets/openstack-credentials.yaml
applications/overlays/my-cluster/secrets/keycloak-secret.yaml
applications/overlays/my-cluster/secrets/grafana-secret.yaml

Total: 4 encrypted files
```

### Check Encryption Status

Alias for `secrets-list`:

```bash
opencenter secrets status --path applications/overlays/my-cluster
```

## Rotate Encryption Keys

Rotate Age keys for security (recommended every 90 days):

```bash
opencenter secrets keys rotate --cluster my-cluster --type age
```

This process:
1. Generates new Age key pair
2. Decrypts all secrets with old key
3. Re-encrypts all secrets with new key
4. Updates `.sops.yaml` configuration
5. Backs up old key

### Complete a Dual-Key Rotation

```bash
opencenter secrets keys rotate --cluster my-cluster --type age --complete
```

### Rotate SSH Keys

```bash
opencenter secrets keys rotate --cluster my-cluster --type ssh
```

## Backup Encryption Keys

Create backup of Age key:

```bash
opencenter secrets keys backup
```

Default backup location: `~/.config/sops/age/backups/`

### Custom Backup Location

```bash
opencenter secrets keys backup --backup-dir /secure/backup/location
```

### Backup a Specific Key File

```bash
opencenter secrets keys backup \
  --key-file /secure/location/my-key.txt \
  --backup-dir /backup
```

Backups are timestamped automatically.

## SOPS Configuration File

The `.sops.yaml` file defines encryption rules:

```yaml
# SOPS configuration for cluster: my-cluster
creation_rules:
  # Encrypt Age keys themselves
  - path_regex: 'secrets/age/keys/.*-key\.txt$'
    age: >-
      age1abc123...
  
  # Encrypt SSH private keys
  - path_regex: 'secrets/ssh/(?!.*\.pub$).*'
    age: >-
      age1abc123...
  
  # Encrypt service secrets
  - path_regex: 'applications/overlays/[^/]+/(managed-services|services)/.*/.*\.ya?ml$'
    encrypted_regex: "^(secret)$"
    age: >-
      age1abc123...
  
  # Encrypt infrastructure secrets
  - path_regex: '^infrastructure\/clusters\/my-cluster\/(?!(?:venv|kubespray|\.terraform|\.bin)\/)(.*)'
    encrypted_regex: "^(secret)$"
    age: >-
      age1abc123...
```

This configuration:
- Encrypts Age keys with themselves (for Git storage)
- Encrypts SSH private keys (excludes `.pub` files)
- Encrypts service secrets (only `secret` fields in YAML)
- Encrypts infrastructure secrets (excludes build directories)

## Environment Variables

### SOPS_AGE_KEY_FILE

Point SOPS to your Age key:

```bash
export SOPS_AGE_KEY_FILE=~/.config/opencenter/clusters/my-org/secrets/age/my-cluster-key.txt
```

Add to shell profile for persistence:

```bash
# ~/.bashrc or ~/.zshrc
export SOPS_AGE_KEY_FILE="$HOME/.config/opencenter/clusters/my-org/secrets/age/my-cluster-key.txt"
```

### SOPS_AGE_RECIPIENTS

Specify Age public keys for encryption:

```bash
export SOPS_AGE_RECIPIENTS=age1abc123...
```

## Manual SOPS Operations

### Encrypt File Manually

```bash
sops --encrypt --age age1abc123... \
  --encrypted-regex '^(secret)$' \
  --in-place secrets/credentials.yaml
```

### Decrypt File Manually

```bash
sops --decrypt secrets/credentials.yaml
```

### Edit Encrypted File

```bash
sops secrets/credentials.yaml
```

This decrypts, opens in editor, and re-encrypts on save.

## FluxCD Integration

FluxCD automatically decrypts secrets during reconciliation.

### Configure FluxCD SOPS Decryption

Create Age key secret in cluster:

```bash
kubectl create secret generic sops-age \
  --from-file=age.agekey=$HOME/.config/opencenter/clusters/my-org/secrets/age/my-cluster-key.txt \
  -n flux-system
```

### Kustomization with SOPS

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: my-service
  namespace: flux-system
spec:
  interval: 5m
  path: ./applications/overlays/my-cluster/services/my-service
  sourceRef:
    kind: GitRepository
    name: my-cluster
  decryption:
    provider: sops
    secretRef:
      name: sops-age
```

## Security Best Practices

### Key Storage

- Store private keys securely (encrypted filesystem, secrets manager)
- Never commit unencrypted private keys to Git
- Backup keys to secure, offline location
- Use different keys for different environments (dev, staging, prod)

### Key Rotation

- Rotate keys every 90 days
- Rotate immediately if key is compromised
- Keep old keys for 30 days (for rollback)
- Document rotation in change log

### Access Control

- Limit access to private keys (need-to-know basis)
- Use separate keys per team/project
- Audit key access regularly
- Revoke access when team members leave

### Git Hygiene

- Never commit plaintext secrets
- Use pre-commit hooks to prevent accidents
- Scan Git history for leaked secrets
- Rotate keys if secrets are committed unencrypted

## Troubleshooting

### SOPS Not Found

**Problem:** `sops: command not found`

**Solution:** Install SOPS:

```bash
# macOS
brew install sops

# Linux
wget https://github.com/mozilla/sops/releases/download/v3.8.1/sops-v3.8.1.linux.amd64
sudo mv sops-v3.8.1.linux.amd64 /usr/local/bin/sops
sudo chmod +x /usr/local/bin/sops
```

### Decryption Failed

**Problem:** `Failed to decrypt: no key could decrypt the data`

**Solution:** Ensure `SOPS_AGE_KEY_FILE` points to correct key:

```bash
export SOPS_AGE_KEY_FILE=/path/to/correct/key.txt
sops --decrypt secrets/credentials.yaml
```

### Invalid Key Format

**Problem:** `Invalid age key format`

**Solution:** Regenerate key:

```bash
opencenter secrets keys generate --key-file /path/to/new-key.txt
```

### FluxCD Decryption Fails

**Problem:** FluxCD shows `decryption failed` error

**Solution:** Verify Age key secret exists:

```bash
kubectl get secret sops-age -n flux-system
```

Recreate if missing:

```bash
kubectl create secret generic sops-age \
  --from-file=age.agekey=$SOPS_AGE_KEY_FILE \
  -n flux-system
```

## Next Steps

- [Customize Services](customize-services.md) - Configure platform services with encrypted secrets
- [Backup and Restore](backup-and-restore.md) - Include encryption keys in backups
- [Troubleshoot Deployment](troubleshoot-deployment.md) - Fix SOPS-related issues

---

## Evidence

This how-to guide is based on:

- SOPS commands: `cmd/sops.go:57-1035`
- SOPS manager: `internal/sops/manager.go:1-600`
- Key generation: `cmd/sops.go:57-86`
- Key rotation: `cmd/sops.go:88-123`
- Validation: `cmd/sops.go:158-219`
- Encryption: `cmd/sops.go:882-957`
- Tech guide SOPS: `.kiro/steering/tech.md:137-139`
- Session 1 security review: A11
- Session 2 facts inventory: B0 section 9
