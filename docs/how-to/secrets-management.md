# Secrets Management with SOPS and Age


## Table of Contents

- [What SOPS provides](#what-sops-provides)
- [Prerequisites](#prerequisites)
- [Quick start](#quick-start)
- [Age key management](#age-key-management)
- [SOPS configuration](#sops-configuration)
- [Encrypting secrets](#encrypting-secrets)
- [Decrypting secrets](#decrypting-secrets)
- [Integration with openCenter](#integration-with-opencenter)
- [Common workflows](#common-workflows)
- [Troubleshooting](#troubleshooting)
- [Security best practices](#security-best-practices)
- [Advanced usage](#advanced-usage)
- [Command reference](#command-reference)
- [Related documentation](#related-documentation)
- [External resources](#external-resources)
**doc_type:** how-to

This guide shows you how to manage secrets in openCenter using SOPS (Secrets OPerationS) with Age encryption.

## What SOPS provides

SOPS encrypts sensitive data in YAML files while keeping the structure readable. This lets you:
- Store encrypted secrets in Git safely
- Decrypt secrets automatically during deployment
- Rotate encryption keys without re-encrypting all files
- Audit who accessed secrets through Git history

## Prerequisites

- SOPS installed (`brew install sops` or download from GitHub)
- Age encryption tool installed (`brew install age`)
- openCenter CLI installed and configured

## Quick start

Generate an Age key and encrypt a secret:

```bash
# Generate Age key
openCenter sops generate-key

# Encrypt a secret file
openCenter sops secrets-encrypt --search-path ./secrets
```

The key is stored at `~/.config/sops/age/keys.txt` by default.

## Age key management

### Generate a new Age key

Create a new encryption key pair:

```bash
openCenter sops generate-key
```


This creates:
- Private key at `~/.config/sops/age/keys.txt`
- Public key displayed in output
- Updated `.sops.yaml` configuration

Specify a custom location:

```bash
openCenter sops generate-key --key-file ~/.config/myproject/age-key.txt
```

Skip SOPS config update:

```bash
openCenter sops generate-key --update-sops-config=false
```

### View your public key

Extract the public key from your private key file:

```bash
grep "public key:" ~/.config/sops/age/keys.txt
```

Or use Age directly:

```bash
age-keygen -y ~/.config/sops/age/keys.txt
```

### Back up Age keys

Create a timestamped backup:

```bash
openCenter sops backup-key
```

Backups are stored in `~/.config/sops/age/backups/` by default.

Specify a custom backup location:

```bash
openCenter sops backup-key --backup-dir ~/secure-backups
```

Store backups securely:
- Encrypted external drive
- Password manager with file attachments
- Hardware security key
- Offline storage

### Rotate Age keys

Replace an existing key and re-encrypt all secrets:

```bash
openCenter sops rotate-key --search-path ./gitops
```


The rotation process:
1. Backs up the old key
2. Generates a new key pair
3. Finds all SOPS-encrypted files
4. Re-encrypts each file with the new key
5. Updates `.sops.yaml` configuration

If rotation fails, the old key is restored automatically.

### Validate Age key setup

Check that your Age key and SOPS configuration are correct:

```bash
openCenter sops validate
```

This verifies:
- Age key file exists and is readable
- Age key format is valid
- SOPS configuration is correct
- SOPS is installed and accessible

Specify a custom key file:

```bash
openCenter sops validate --key-file ~/.config/myproject/age-key.txt
```

## SOPS configuration

### Create .sops.yaml

SOPS uses `.sops.yaml` to determine which files to encrypt and which keys to use.

Basic configuration:

```yaml
creation_rules:
  - path_regex: .*\.(yaml|yml)$
    age: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    encrypted_regex: '^(data|stringData|password|token|key|secret|credentials)'
```


Multiple keys for team access:

```yaml
creation_rules:
  - path_regex: .*\.(yaml|yml)$
    age: >-
      age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx,
      age1yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy,
      age1zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz
    encrypted_regex: '^(data|stringData|password|token|key|secret|credentials)'
```

Path-specific rules:

```yaml
creation_rules:
  # Production secrets - multiple keys required
  - path_regex: production/.*\.yaml$
    age: >-
      age1prod1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx,
      age1prod2xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    encrypted_regex: '^(data|stringData)'
  
  # Development secrets - single key
  - path_regex: development/.*\.yaml$
    age: age1devxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    encrypted_regex: '^(data|stringData)'
```

### Update .sops.yaml

After generating a new key, update your SOPS configuration:

```bash
# Automatic update during key generation
openCenter sops generate-key --update-sops-config

# Manual update
vim .sops.yaml
```

Replace the old public key with the new one.


## Encrypting secrets

### List files to encrypt

Find files that should be encrypted:

```bash
openCenter sops secrets-list --search-path ./gitops
```

This shows:
- Files already encrypted
- Files that should be encrypted (contain sensitive patterns)

### Encrypt secrets with backups

Encrypt files and create backups:

```bash
openCenter sops secrets-encrypt --search-path ./gitops
```

This:
1. Creates timestamped backups of original files
2. Encrypts files matching SOPS rules
3. Validates successful encryption
4. Reports results

Backups are saved as `<filename>.backup-<timestamp>`.

### Fast encryption without backups

Skip backup creation for faster operation:

```bash
openCenter sops secrets-encrypt-fast --search-path ./gitops
```

Use this when:
- Files are already in version control
- You have external backups
- Speed is critical

### Encrypt specific files

Encrypt individual files with SOPS directly:

```bash
sops --encrypt --in-place secrets/credentials.yaml
```


Or specify the Age key explicitly:

```bash
sops --encrypt --age age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
  --in-place secrets/credentials.yaml
```

### Dry run

Preview what would be encrypted:

```bash
openCenter sops secrets-encrypt --search-path ./gitops --dry-run
```

## Decrypting secrets

### Decrypt secrets with backups

Decrypt files and create backups of encrypted versions:

```bash
openCenter sops secrets-decrypt --search-path ./gitops
```

This:
1. Creates backups of encrypted files
2. Decrypts files in place
3. Validates successful decryption
4. Reports results

### Fast decryption without backups

Skip backup creation:

```bash
openCenter sops secrets-decrypt-fast --search-path ./gitops
```

### Decrypt specific files

Decrypt and view a file without modifying it:

```bash
sops --decrypt secrets/credentials.yaml
```

Decrypt to a new file:

```bash
sops --decrypt secrets/credentials.yaml > secrets/credentials-plain.yaml
```


Decrypt in place:

```bash
sops --decrypt --in-place secrets/credentials.yaml
```

### Edit encrypted files

Edit a file while keeping it encrypted:

```bash
sops secrets/credentials.yaml
```

SOPS:
1. Decrypts the file
2. Opens it in your editor
3. Re-encrypts on save

Set your preferred editor:

```bash
export EDITOR=vim
sops secrets/credentials.yaml
```

## Integration with openCenter

### Cluster initialization

openCenter generates Age keys automatically during cluster init:

```bash
openCenter cluster init my-cluster
```

Keys are stored in the cluster's secrets directory:
```
~/.config/openCenter/clusters/<organization>/secrets/age/<cluster>-key.txt
```

Skip automatic key generation:

```bash
openCenter cluster init my-cluster --no-keygen
```

Regenerate keys for an existing cluster:

```bash
openCenter cluster init my-cluster --regenerate-keys --force
```


### GitOps repository encryption

After running `cluster setup`, encrypt sensitive files:

```bash
openCenter cluster setup my-cluster
cd ~/.config/openCenter/clusters/myorg/gitops
openCenter sops secrets-encrypt --search-path .
```

Files automatically encrypted:
- `flux-system/gotk-sync.yaml` - FluxCD sync credentials
- `managed-services/sources/base-repo.yaml` - Git repository credentials
- `secrets/openstack-credentials.yaml` - Cloud provider credentials
- Service-specific secrets in `applications/overlays/<cluster>/secrets/`

### FluxCD integration

FluxCD decrypts secrets automatically using the SOPS Age key.

The key is stored as a Kubernetes secret:

```bash
kubectl get secret sops-age -n flux-system
```

FluxCD Kustomizations reference this secret:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: my-app
  namespace: flux-system
spec:
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  # ... rest of spec
```

## Common workflows

### Add a new secret to existing cluster

1. Create the secret file:

```bash
cat > secrets/new-secret.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  namespace: default
type: Opaque
stringData:
  username: admin
  password: changeme
EOF
```


2. Encrypt the file:

```bash
sops --encrypt --in-place secrets/new-secret.yaml
```

3. Commit and push:

```bash
git add secrets/new-secret.yaml
git commit -m "Add new secret"
git push
```

FluxCD decrypts and applies the secret automatically.

### Share access with team member

1. Get their Age public key:

```bash
# They run:
age-keygen -y ~/.config/sops/age/keys.txt
```

2. Add their key to `.sops.yaml`:

```yaml
creation_rules:
  - path_regex: .*\.(yaml|yml)$
    age: >-
      age1yourkey...,
      age1theirkey...
```

3. Re-encrypt all files:

```bash
openCenter sops rotate-key --search-path .
```

Now both keys can decrypt the files.

### Revoke access

1. Remove their key from `.sops.yaml`

2. Rotate to a new key:

```bash
openCenter sops rotate-key --search-path .
```


3. Distribute the new private key to authorized users only

### Migrate from plaintext secrets

1. Identify plaintext secret files:

```bash
openCenter sops secrets-list --search-path .
```

2. Back up plaintext files:

```bash
tar czf secrets-backup-$(date +%Y%m%d).tar.gz secrets/
```

3. Encrypt all secrets:

```bash
openCenter sops secrets-encrypt --search-path .
```

4. Verify encryption:

```bash
openCenter sops secrets-list --search-path .
```

5. Remove plaintext backups from version control:

```bash
git rm secrets/*.backup-*
git commit -m "Remove plaintext secret backups"
```

## Troubleshooting

### "no key could be found" error

Error: `Failed to get the data key required to decrypt the SOPS file`

Cause: SOPS cannot find your Age private key.

Solution: Set the key file location:

```bash
export SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt
sops --decrypt secrets/credentials.yaml
```


Or specify it in the command:

```bash
openCenter sops secrets-decrypt --key-file ~/.config/sops/age/keys.txt
```

### "MAC mismatch" error

Error: `MAC mismatch`

Cause: File was modified after encryption or is corrupted.

Solution: Restore from backup or Git history:

```bash
git checkout HEAD -- secrets/credentials.yaml
```

### File not encrypting

Check if the file matches SOPS rules:

```bash
# View SOPS rules
cat .sops.yaml

# Test if file matches
sops --encrypt --in-place secrets/test.yaml
```

If SOPS skips the file, check:
- File extension matches `path_regex`
- File contains fields matching `encrypted_regex`
- `.sops.yaml` is in the current directory or parent

### FluxCD cannot decrypt secrets

Check the SOPS Age secret exists:

```bash
kubectl get secret sops-age -n flux-system
```

If missing, create it:

```bash
kubectl create secret generic sops-age \
  --namespace=flux-system \
  --from-file=age.agekey=$HOME/.config/sops/age/keys.txt
```


Check FluxCD logs:

```bash
kubectl logs -n flux-system -l app=kustomize-controller | grep -i sops
```

### Key file permissions error

Error: `permission denied` when reading key file

Solution: Fix file permissions:

```bash
chmod 600 ~/.config/sops/age/keys.txt
```

Age keys should be readable only by the owner.

## Security best practices

### Key storage

- **Never commit private keys**: Add `*.txt` to `.gitignore` in key directories
- **Restrict permissions**: `chmod 600` on private key files
- **Back up keys securely**: Store backups offline or in encrypted storage
- **Use separate keys per environment**: Different keys for dev, staging, production

### Key rotation

- **Rotate regularly**: Every 90 days for production environments
- **Rotate after team changes**: When someone leaves or changes roles
- **Rotate after exposure**: Immediately if a key is compromised
- **Test rotation**: Practice key rotation in non-production first

### Access control

- **Principle of least privilege**: Only grant access to those who need it
- **Audit access**: Review who has keys regularly
- **Use multiple keys**: Require multiple keys for critical secrets
- **Document key holders**: Maintain a list of who has which keys


### Git hygiene

- **Never commit plaintext secrets**: Always encrypt before committing
- **Review diffs carefully**: Check that secrets are encrypted in `git diff`
- **Use pre-commit hooks**: Prevent accidental plaintext commits
- **Audit history**: Scan Git history for leaked secrets

### Operational security

- **Encrypt in CI/CD**: Store Age keys as encrypted CI/CD secrets
- **Limit key distribution**: Use secure channels to share keys
- **Monitor access**: Log and alert on secret access
- **Incident response**: Have a plan for key compromise

## Advanced usage

### Multiple Age keys per file

Encrypt with multiple keys so any key can decrypt:

```bash
sops --encrypt \
  --age age1key1...,age1key2...,age1key3... \
  --in-place secrets/shared.yaml
```

### Partial encryption

Encrypt only specific fields:

```yaml
# .sops.yaml
creation_rules:
  - path_regex: .*\.yaml$
    encrypted_regex: '^(password|token|key)$'
```

This encrypts only fields named `password`, `token`, or `key`.

### Environment-specific keys

Use different keys per environment:

```yaml
# .sops.yaml
creation_rules:
  - path_regex: production/.*
    age: age1prod...
  - path_regex: staging/.*
    age: age1staging...
  - path_regex: development/.*
    age: age1dev...
```


### Automated encryption in CI/CD

Encrypt secrets in CI pipelines:

```bash
# GitHub Actions example
- name: Encrypt secrets
  env:
    SOPS_AGE_KEY: ${{ secrets.SOPS_AGE_KEY }}
  run: |
    echo "$SOPS_AGE_KEY" > /tmp/age-key.txt
    export SOPS_AGE_KEY_FILE=/tmp/age-key.txt
    openCenter sops secrets-encrypt --search-path ./gitops
    rm /tmp/age-key.txt
```

### Key management with HashiCorp Vault

Store Age keys in Vault:

```bash
# Store key in Vault
vault kv put secret/sops/age-key value=@~/.config/sops/age/keys.txt

# Retrieve and use
vault kv get -field=value secret/sops/age-key > /tmp/age-key.txt
export SOPS_AGE_KEY_FILE=/tmp/age-key.txt
sops --decrypt secrets/credentials.yaml
rm /tmp/age-key.txt
```

## Command reference

### Key management commands

```bash
# Generate new Age key
openCenter sops generate-key [--key-file PATH] [--update-sops-config]

# Rotate Age keys
openCenter sops rotate-key [--key-file PATH] [--search-path PATH]

# Back up Age keys
openCenter sops backup-key [--key-file PATH] [--backup-dir PATH]

# Validate Age key setup
openCenter sops validate [--key-file PATH] [--config-file PATH]
```


### Secrets management commands

```bash
# List secrets status
openCenter sops secrets-list [--search-path PATH] [--key-file PATH]

# Encrypt secrets with backups
openCenter sops secrets-encrypt [--search-path PATH] [--backups]

# Encrypt secrets without backups (fast)
openCenter sops secrets-encrypt-fast [--search-path PATH]

# Decrypt secrets with backups
openCenter sops secrets-decrypt [--search-path PATH] [--backups]

# Decrypt secrets without backups (fast)
openCenter sops secrets-decrypt-fast [--search-path PATH]

# Show status (alias for secrets-list)
openCenter sops secrets-status [--search-path PATH]
```

### Common flags

- `--key-file PATH` - Path to Age key file (default: `~/.config/sops/age/keys.txt`)
- `--search-path PATH` - Directory to search for files (default: `.`)
- `--dry-run` - Show what would be done without making changes
- `--backups` - Create backups before encryption/decryption (default: `true`)

## Related documentation

- [Deploying Changes](deploying-changes.md) - Apply configuration updates
- [Troubleshooting](troubleshooting.md) - Debug common issues
- [CLI Commands Reference](../reference/cli-commands.md) - Complete command reference

## External resources

- [SOPS Documentation](https://github.com/mozilla/sops)
- [Age Encryption](https://age-encryption.org/)
- [FluxCD SOPS Guide](https://fluxcd.io/flux/guides/mozilla-sops/)
