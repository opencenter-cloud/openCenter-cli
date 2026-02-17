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
opencenter sops generate-key --cluster my-cluster
```

This creates:
- Private key: `~/.config/opencenter/clusters/<org>/secrets/age/<cluster>-key.txt`
- Public key: Embedded in private key file
- SOPS configuration: `.sops.yaml` in GitOps repository

Key format:
```
# created: 2026-02-17T10:30:00Z
# public key: age1abc123...
AGE-SECRET-KEY-1ABC123...
```

### Custom Key Location

Specify custom key file location:

```bash
opencenter sops generate-key --cluster my-cluster \
  --key-file /secure/location/my-key.txt
```

### Backup Key During Generation

Create backup copy automatically:

```bash
opencenter sops generate-key --cluster my-cluster \
  --backup-dir /backup/location
```

## Validate SOPS Configuration

Check that SOPS is properly configured:

```bash
opencenter sops validate --cluster my-cluster
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
opencenter sops validate --key-file /path/to/key.txt
```

### Verbose Validation

See detailed validation information:

```bash
opencenter sops validate --cluster my-cluster --verbose
```

## Encrypt Secrets

Encrypt sensitive files in your GitOps repository:

```bash
opencenter sops secrets-encrypt --cluster my-cluster
```

This encrypts:
- `flux-system/gotk-sync.yaml` - FluxCD sync configuration
- `managed-services/sources/base-repo.yaml` - GitRepository sources
- `secrets/*.yaml` - Provider credentials
- Service-specific secrets (Keycloak, Grafana, etc.)

### Encrypt Specific File

```bash
opencenter sops secrets-encrypt --cluster my-cluster \
  --file applications/overlays/my-cluster/secrets/credentials.yaml
```

### Fast Parallel Encryption

Encrypt multiple files in parallel:

```bash
opencenter sops secrets-encrypt-fast --cluster my-cluster
```

This uses 4 parallel workers for faster encryption of large repositories.

## Decrypt Secrets

Decrypt secrets for viewing or editing:

```bash
opencenter sops secrets-decrypt --cluster my-cluster
```

### Decrypt Specific File

```bash
opencenter sops secrets-decrypt --cluster my-cluster \
  --file applications/overlays/my-cluster/secrets/credentials.yaml
```

### Fast Parallel Decryption

```bash
opencenter sops secrets-decrypt-fast --cluster my-cluster
```

## List Encrypted Secrets

See all encrypted files in repository:

```bash
opencenter sops secrets-list --cluster my-cluster
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
opencenter sops secrets-status --cluster my-cluster
```

## Rotate Encryption Keys

Rotate Age keys for security (recommended every 90 days):

```bash
opencenter sops rotate-key --cluster my-cluster
```

This process:
1. Generates new Age key pair
2. Decrypts all secrets with old key
3. Re-encrypts all secrets with new key
4. Updates `.sops.yaml` configuration
5. Backs up old key

### Rotate with Custom Backup Location

```bash
opencenter sops rotate-key --cluster my-cluster \
  --backup-dir /secure/backup/location
```

### Rotate Specific Key File

```bash
opencenter sops rotate-key --cluster my-cluster \
  --key-file /path/to/current-key.txt \
  --new-key-file /path/to/new-key.txt
```

## Backup Encryption Keys

Create backup of Age key:

```bash
opencenter sops backup-key --cluster my-cluster
```

Default backup location: `~/.config/opencenter/clusters/<org>/secrets/age/backups/`

### Custom Backup Location

```bash
opencenter sops backup-key --cluster my-cluster \
  --backup-dir /secure/backup/location
```

### Backup with Timestamp

```bash
opencenter sops backup-key --cluster my-cluster \
  --backup-dir /backup \
  --timestamp
```

Creates: `/backup/<cluster>-key-20260217-103000.txt`

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
opencenter sops generate-key --cluster my-cluster --force
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
