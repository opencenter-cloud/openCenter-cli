# Backup and Recovery


## Table of Contents

- [Task Summary](#task-summary)
- [Prerequisites](#prerequisites)
- [Backup Procedures](#backup-procedures)
- [Restore Procedures](#restore-procedures)
- [Backup Strategies](#backup-strategies)
- [Disaster Recovery Scenarios](#disaster-recovery-scenarios)
- [Backup Management](#backup-management)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)
- [Related Documentation](#related-documentation)
**doc_type: how-to**

Protect your cluster configurations and secrets with backups. Restore them when needed.

## Task Summary

This guide shows you how to:
- Back up cluster configurations, secrets, and state
- Restore from backups after data loss or corruption
- Set up automated backup schedules
- Verify backup integrity
- Handle disaster recovery scenarios

## Prerequisites

- openCenter CLI installed and configured
- At least one cluster initialized
- Write access to backup storage location
- For encrypted backups: a secure passphrase

## Backup Procedures

### Configuration Backup

Back up your cluster configuration and related files:

```bash
openCenter cluster backup create my-cluster
```

This creates a compressed archive containing:
- Cluster configuration YAML
- SOPS Age encryption keys
- SSH keys
- GitOps repository state
- Terraform state files

The backup is saved to `~/.config/openCenter/backups/` with a timestamped filename like `my-cluster-20260118-143000.tar.gz`.

**Expected output:**
```
Creating backup for cluster my-cluster...
✓ Backup created: my-cluster-20260118-143000
  Size: 45632 bytes
  Checksum: a3f5b8c9d2e1f4a7b6c5d8e9f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0
  Location: /Users/you/.config/openCenter/backups/my-cluster-20260118-143000.tar.gz

Backup created successfully!
Retention until: 2026-02-17T14:30:00Z
```

### Encrypted Backup

For sensitive environments, encrypt backups with a passphrase:

```bash
openCenter cluster backup create my-cluster --encrypt
```

You'll be prompted for a passphrase. The backup is encrypted with AES-256-GCM using Argon2 key derivation.

**Alternative:** Provide the passphrase directly (less secure):
```bash
openCenter cluster backup create my-cluster --passphrase="your-secure-passphrase"
```

**Warning:** Encrypted backups cannot be restored without the correct passphrase. Store passphrases securely in a password manager or secrets vault.

### SOPS Key Backup

Back up SOPS Age keys separately for additional protection:

```bash
openCenter sops backup-key
```

This creates a timestamped backup in `~/.config/sops/age/backups/` and includes:
- Age private key file
- SOPS configuration (`.sops.yaml`)
- Backup metadata

**Expected output:**
```
💾 Creating Age key backup...
📁 Backup directory: /Users/you/.config/sops/age/backups
✅ Age key backup created successfully!
📁 Backup directory: /Users/you/.config/sops/age/backups
✅ SOPS configuration backed up to: /Users/you/.config/sops/age/backups/sops-config-20260118-143000.yaml
```

**Custom backup location:**
```bash
openCenter sops backup-key --backup-dir=/secure/backup/location
```

### GitOps Repository Backup

Your GitOps repository contains the complete cluster state. Back it up using git:

```bash
cd ~/.config/openCenter/clusters/myorg
git bundle create ~/backups/gitops-$(date +%Y%m%d).bundle --all
```

This creates a portable git bundle containing all branches and history.

**Verify the bundle:**
```bash
git bundle verify ~/backups/gitops-20260118.bundle
```

### etcd Backup (Kubernetes State)

For running clusters, back up etcd to preserve Kubernetes state:

```bash
# SSH to a control plane node
ssh -i ~/.config/openCenter/clusters/myorg/secrets/ssh/my-cluster-dev-sjc3 ubuntu@control-plane-1

# Create etcd snapshot
sudo ETCDCTL_API=3 etcdctl snapshot save /tmp/etcd-snapshot.db \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key

# Copy snapshot to local machine
exit
scp -i ~/.config/openCenter/clusters/myorg/secrets/ssh/my-cluster-dev-sjc3 \
  ubuntu@control-plane-1:/tmp/etcd-snapshot.db \
  ~/backups/etcd-snapshot-$(date +%Y%m%d).db
```

### Persistent Volume Snapshots

For stateful applications, create volume snapshots using your cloud provider's tools:

**OpenStack (Cinder):**
```bash
openstack volume snapshot create --volume my-volume my-volume-snapshot
```

**AWS (EBS):**
```bash
aws ec2 create-snapshot --volume-id vol-1234567890abcdef0 --description "Backup $(date +%Y%m%d)"
```

## Restore Procedures

### Configuration Restore

Restore a cluster configuration from backup:

```bash
openCenter cluster backup restore my-cluster-20260118-143000
```

For encrypted backups, provide the passphrase:

```bash
openCenter cluster backup restore my-cluster-20260118-143000 --passphrase="your-secure-passphrase"
```

**Expected output:**
```
Restoring backup my-cluster-20260118-143000...
✓ Backup restored successfully!

Restored files are in the 'restored' directory:
  Config: ~/.config/openCenter/clusters/restored/.restored-config.yaml
  Age key: ~/.config/openCenter/secrets/age/restored-key.txt
  SSH keys: ~/.config/openCenter/secrets/ssh/restored-keys

Please review and move files to appropriate locations.
```

**Move restored files to active locations:**

```bash
# Review the restored configuration
cat ~/.config/openCenter/clusters/restored/.restored-config.yaml

# Move to active location (replace 'myorg' with your organization)
mv ~/.config/openCenter/clusters/restored/.restored-config.yaml \
   ~/.config/openCenter/clusters/myorg/.my-cluster-config.yaml

# Restore Age key
mv ~/.config/openCenter/secrets/age/restored-key.txt \
   ~/.config/openCenter/clusters/myorg/secrets/age/keys/my-cluster-key.txt

# Restore SSH keys
mv ~/.config/openCenter/secrets/ssh/restored-keys \
   ~/.config/openCenter/clusters/myorg/secrets/ssh/my-cluster-dev-sjc3
```

### GitOps Repository Restore

Restore a GitOps repository from a git bundle:

```bash
# Create new directory
mkdir -p ~/.config/openCenter/clusters/myorg-restored
cd ~/.config/openCenter/clusters/myorg-restored

# Clone from bundle
git clone ~/backups/gitops-20260118.bundle .

# Verify contents
git log --oneline
ls -la
```

### SOPS Key Restore

If you lose your SOPS Age key, restore from backup:

```bash
# Copy key from backup
cp ~/.config/sops/age/backups/keys-backup-20260118-143000.txt \
   ~/.config/openCenter/clusters/myorg/secrets/age/keys/my-cluster-key.txt

# Set correct permissions
chmod 600 ~/.config/openCenter/clusters/myorg/secrets/age/keys/my-cluster-key.txt

# Verify key works
openCenter sops validate --key-file ~/.config/openCenter/clusters/myorg/secrets/age/keys/my-cluster-key.txt
```

### etcd Restore

Restore Kubernetes state from an etcd snapshot:

```bash
# Copy snapshot to control plane node
scp -i ~/.config/openCenter/clusters/myorg/secrets/ssh/my-cluster-dev-sjc3 \
  ~/backups/etcd-snapshot-20260118.db \
  ubuntu@control-plane-1:/tmp/

# SSH to control plane
ssh -i ~/.config/openCenter/clusters/myorg/secrets/ssh/my-cluster-dev-sjc3 ubuntu@control-plane-1

# Stop kube-apiserver
sudo systemctl stop kube-apiserver

# Restore snapshot
sudo ETCDCTL_API=3 etcdctl snapshot restore /tmp/etcd-snapshot-20260118.db \
  --data-dir=/var/lib/etcd-restored \
  --name=control-plane-1 \
  --initial-cluster=control-plane-1=https://10.0.0.10:2380 \
  --initial-advertise-peer-urls=https://10.0.0.10:2380

# Move restored data
sudo mv /var/lib/etcd /var/lib/etcd-old
sudo mv /var/lib/etcd-restored /var/lib/etcd

# Start kube-apiserver
sudo systemctl start kube-apiserver

# Verify cluster state
kubectl get nodes
```

**Warning:** etcd restore is a destructive operation. All Kubernetes state created after the snapshot will be lost. Test in a non-production environment first.

### Volume Restore

Restore persistent volumes from snapshots:

**OpenStack (Cinder):**
```bash
# Create volume from snapshot
openstack volume create --snapshot my-volume-snapshot --size 100 my-volume-restored

# Attach to instance
openstack server add volume my-instance my-volume-restored
```

**AWS (EBS):**
```bash
# Create volume from snapshot
aws ec2 create-volume --snapshot-id snap-1234567890abcdef0 --availability-zone us-east-1a

# Attach to instance
aws ec2 attach-volume --volume-id vol-0987654321fedcba0 --instance-id i-1234567890abcdef0 --device /dev/sdf
```

## Backup Strategies

### Automated Scheduled Backups

Schedule periodic backups using cron:

```bash
# Edit crontab
crontab -e

# Add daily backup at 2 AM
0 2 * * * /usr/local/bin/openCenter cluster backup create my-cluster --encrypt --passphrase="$(cat ~/.backup-passphrase)" >> /var/log/opencenter-backup.log 2>&1
```

**Note:** The `schedule` command is planned for a future release:
```bash
# Future feature
openCenter cluster backup schedule my-cluster --interval=24h --retention=30d
```

### Retention Policies

Implement a retention policy to manage backup storage:

```bash
#!/bin/bash
# cleanup-old-backups.sh

BACKUP_DIR="$HOME/.config/openCenter/backups"
RETENTION_DAYS=30

# Delete backups older than retention period
find "$BACKUP_DIR" -name "*.tar.gz*" -mtime +$RETENTION_DAYS -delete

echo "Deleted backups older than $RETENTION_DAYS days"
```

Run this script weekly:
```bash
crontab -e
# Add weekly cleanup on Sunday at 3 AM
0 3 * * 0 /path/to/cleanup-old-backups.sh >> /var/log/opencenter-cleanup.log 2>&1
```

### Backup Verification

Verify backup integrity regularly:

```bash
# List all backups
openCenter cluster backup list my-cluster

# Verify checksum
sha256sum ~/.config/openCenter/backups/my-cluster-20260118-143000.tar.gz
cat ~/.config/openCenter/backups/my-cluster-20260118-143000.tar.gz.sha256

# Test restore to temporary location
openCenter cluster backup restore my-cluster-20260118-143000
# Verify restored files
ls -la ~/.config/openCenter/clusters/restored/
```

### Off-Site Backup Storage

Store backups in a separate location for disaster recovery:

**Cloud storage (S3):**
```bash
# Upload to S3
aws s3 cp ~/.config/openCenter/backups/my-cluster-20260118-143000.tar.gz.enc \
  s3://my-backup-bucket/opencenter/my-cluster/

# Download from S3
aws s3 cp s3://my-backup-bucket/opencenter/my-cluster/my-cluster-20260118-143000.tar.gz.enc \
  ~/restored-backups/
```

**Remote server (rsync):**
```bash
# Sync to remote server
rsync -avz --delete \
  ~/.config/openCenter/backups/ \
  backup-server:/backups/opencenter/

# Restore from remote server
rsync -avz \
  backup-server:/backups/opencenter/my-cluster-20260118-143000.tar.gz.enc \
  ~/restored-backups/
```

## Disaster Recovery Scenarios

### Complete Cluster Loss

If you lose all cluster infrastructure:

1. **Restore configuration:**
   ```bash
   openCenter cluster backup restore my-cluster-20260118-143000
   mv ~/.config/openCenter/clusters/restored/.restored-config.yaml \
      ~/.config/openCenter/clusters/myorg/.my-cluster-config.yaml
   ```

2. **Restore secrets:**
   ```bash
   mv ~/.config/openCenter/secrets/age/restored-key.txt \
      ~/.config/openCenter/clusters/myorg/secrets/age/keys/my-cluster-key.txt
   mv ~/.config/openCenter/secrets/ssh/restored-keys \
      ~/.config/openCenter/clusters/myorg/secrets/ssh/my-cluster-dev-sjc3
   ```

3. **Validate configuration:**
   ```bash
   openCenter cluster validate my-cluster
   ```

4. **Provision new infrastructure:**
   ```bash
   openCenter cluster bootstrap my-cluster
   ```

5. **Restore application data from volume snapshots** (see Volume Restore above)

### Partial Data Loss

If you lose only configuration files but infrastructure is intact:

1. **Restore configuration from backup**
2. **Validate against running cluster:**
   ```bash
   openCenter cluster validate my-cluster
   ```
3. **Update GitOps repository if needed:**
   ```bash
   openCenter cluster setup my-cluster --force
   ```

### Configuration Corruption

If your configuration file becomes corrupted:

1. **List available backups:**
   ```bash
   openCenter cluster backup list my-cluster
   ```

2. **Restore most recent backup:**
   ```bash
   openCenter cluster backup restore my-cluster-20260118-143000
   ```

3. **Compare with corrupted file:**
   ```bash
   diff ~/.config/openCenter/clusters/myorg/.my-cluster-config.yaml \
        ~/.config/openCenter/clusters/restored/.restored-config.yaml
   ```

4. **Replace corrupted file:**
   ```bash
   cp ~/.config/openCenter/clusters/restored/.restored-config.yaml \
      ~/.config/openCenter/clusters/myorg/.my-cluster-config.yaml
   ```

### Lost SOPS Key

If you lose your SOPS Age key:

1. **Check for backup:**
   ```bash
   ls -la ~/.config/sops/age/backups/
   ```

2. **Restore from most recent backup:**
   ```bash
   cp ~/.config/sops/age/backups/keys-backup-20260118-143000.txt \
      ~/.config/openCenter/clusters/myorg/secrets/age/keys/my-cluster-key.txt
   chmod 600 ~/.config/openCenter/clusters/myorg/secrets/age/keys/my-cluster-key.txt
   ```

3. **Validate key:**
   ```bash
   openCenter sops validate
   ```

4. **Test decryption:**
   ```bash
   export SOPS_AGE_KEY_FILE=~/.config/openCenter/clusters/myorg/secrets/age/keys/my-cluster-key.txt
   sops -d ~/.config/openCenter/clusters/myorg/infrastructure/clusters/my-cluster/secrets/example-secret.yaml
   ```

**If no backup exists:** You must rotate keys and re-encrypt all secrets:

```bash
# Generate new key
openCenter sops generate-key --update-sops-config

# Re-encrypt all secrets (requires access to plaintext values)
openCenter sops rotate-key --search-path ~/.config/openCenter/clusters/myorg
```

## Backup Management

### List Backups

View all backups for a cluster:

```bash
openCenter cluster backup list my-cluster
```

**Expected output:**
```
BACKUP ID                        CLUSTER      CREATED              SIZE   LOCATION
my-cluster-20260118-143000      my-cluster   2026-01-18 14:30:00  45632  /Users/you/.config/openCenter/backups/my-cluster-20260118-143000.tar.gz
my-cluster-20260117-143000      my-cluster   2026-01-17 14:30:00  45128  /Users/you/.config/openCenter/backups/my-cluster-20260117-143000.tar.gz
my-cluster-20260116-143000      my-cluster   2026-01-16 14:30:00  44892  /Users/you/.config/openCenter/backups/my-cluster-20260116-143000.tar.gz
```

List all backups:
```bash
openCenter cluster backup list
```

### Delete Backups

Remove old or unnecessary backups:

```bash
openCenter cluster backup delete my-cluster-20260116-143000
```

You'll be prompted for confirmation:
```
Are you sure you want to delete backup my-cluster-20260116-143000? (yes/no): yes
✓ Backup my-cluster-20260116-143000 deleted successfully
```

Skip confirmation with `--force`:
```bash
openCenter cluster backup delete my-cluster-20260116-143000 --force
```

## Troubleshooting

### Backup Creation Fails

**Symptom:** Error during backup creation.

**Possible causes:**
- Insufficient disk space
- Permission issues
- Missing configuration files

**Solutions:**

Check disk space:
```bash
df -h ~/.config/openCenter/backups
```

Check permissions:
```bash
ls -la ~/.config/openCenter/
chmod 755 ~/.config/openCenter/backups
```

Verify configuration exists:
```bash
ls -la ~/.config/openCenter/clusters/myorg/.my-cluster-config.yaml
```

### Restore Fails with "Backup Not Found"

**Symptom:** Cannot find backup file.

**Solution:**

List available backups:
```bash
openCenter cluster backup list
ls -la ~/.config/openCenter/backups/
```

Use exact backup ID from the list.

### Decryption Fails

**Symptom:** "Decryption failed" error during restore.

**Possible causes:**
- Incorrect passphrase
- Corrupted backup file
- Wrong encryption algorithm

**Solutions:**

Verify passphrase is correct. Check backup file integrity:
```bash
sha256sum ~/.config/openCenter/backups/my-cluster-20260118-143000.tar.gz.enc
cat ~/.config/openCenter/backups/my-cluster-20260118-143000.tar.gz.enc.sha256
```

If checksums don't match, the file is corrupted. Use an earlier backup.

### Checksum Mismatch

**Symptom:** "Checksum mismatch" error during restore.

**Cause:** Backup file was modified or corrupted.

**Solution:**

The backup file is not trustworthy. Use a different backup:
```bash
openCenter cluster backup list my-cluster
openCenter cluster backup restore my-cluster-20260117-143000
```

### SOPS Key Restore Doesn't Work

**Symptom:** Cannot decrypt secrets after restoring SOPS key.

**Solutions:**

Verify key format:
```bash
cat ~/.config/openCenter/clusters/myorg/secrets/age/keys/my-cluster-key.txt
# Should start with: AGE-SECRET-KEY-
```

Check file permissions:
```bash
ls -la ~/.config/openCenter/clusters/myorg/secrets/age/keys/my-cluster-key.txt
# Should be: -rw------- (600)
```

Validate key:
```bash
openCenter sops validate --key-file ~/.config/openCenter/clusters/myorg/secrets/age/keys/my-cluster-key.txt
```

### etcd Restore Breaks Cluster

**Symptom:** Cluster is unhealthy after etcd restore.

**Cause:** Restore process was incomplete or incorrect.

**Solutions:**

Check etcd logs:
```bash
sudo journalctl -u etcd -n 100
```

Verify etcd data directory:
```bash
sudo ls -la /var/lib/etcd/
```

If cluster is broken, restore from a different snapshot or rebuild the cluster.

## Best Practices

1. **Back up before changes:** Create a backup before making configuration changes or upgrades
2. **Test restores regularly:** Verify backups work by testing restore procedures quarterly
3. **Encrypt sensitive backups:** Always encrypt backups containing secrets or credentials
4. **Store off-site:** Keep at least one backup copy in a different physical location
5. **Document passphrases:** Store backup passphrases in a secure password manager
6. **Automate backups:** Use cron or similar tools for scheduled backups
7. **Monitor backup success:** Check backup logs regularly for failures
8. **Implement retention:** Delete old backups to manage storage costs
9. **Version control GitOps:** Your GitOps repository is already version-controlled; use git tags for important milestones
10. **Backup before key rotation:** Always back up SOPS keys before rotating them

## Related Documentation

- [Secrets Management](secrets-management.md) - SOPS key management and encryption
- [Troubleshooting](troubleshooting.md) - General troubleshooting procedures
- [Configuration Reference](../reference/configuration.md) - Configuration file structure
- [CLI Commands Reference](../reference/cli-commands.md) - Complete command documentation
