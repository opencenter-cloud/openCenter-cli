# Migrating Clusters and Configurations


## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Understanding Migration Types](#understanding-migration-types)
- [Task 1: Migrate to Organization Structure](#task-1-migrate-to-organization-structure)
- [Task 2: Migrate Between Cloud Providers](#task-2-migrate-between-cloud-providers)
- [Task 3: Upgrade Configuration Schema](#task-3-upgrade-configuration-schema)
- [Task 4: Migrate Secrets and Keys](#task-4-migrate-secrets-and-keys)
- [Task 5: Migrate GitOps Repository](#task-5-migrate-gitops-repository)
- [Task 6: Rollback Migration](#task-6-rollback-migration)
- [Task 7: Migrate Multiple Clusters](#task-7-migrate-multiple-clusters)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)
**doc_type**: how-to  
**priority**: 3  
**audience**: Platform engineers managing cluster migrations  
**related_docs**:
- [Configuration Management](./configuration-management.md)
- [Disaster Recovery](./disaster-recovery.md)
- [GitOps Workflows](./gitops-workflows.md)

## Overview

This guide shows you how to migrate clusters and configurations in openCenter. You'll learn how to migrate from legacy flat structure to organization-based structure, migrate between cloud providers, and upgrade cluster configurations.

## Prerequisites

- openCenter CLI installed and configured
- Existing cluster configurations
- Backup of current configurations
- Understanding of target migration scenario

## Understanding Migration Types

openCenter supports several migration scenarios:

1. **Structure Migration**: Legacy flat → Organization-based structure
2. **Provider Migration**: OpenStack → AWS, bare metal → cloud
3. **Version Migration**: Configuration schema upgrades
4. **Cluster Migration**: Move workloads between clusters

## Task 1: Migrate to Organization Structure

openCenter uses an organization-based directory structure for better multi-tenancy support.

### Step 1: Detect Legacy Clusters

```bash
# Build CLI
mise run build

# Check for legacy clusters
./bin/openCenter config list

# Legacy structure:
# ~/.config/openCenter/clusters/my-cluster/

# New structure:
# ~/.config/openCenter/clusters/opencenter/my-cluster/
```

### Step 2: Backup Existing Configuration

```bash
# Create backup before migration
CLUSTER_NAME="my-cluster"
BACKUP_DIR="${HOME}/.config/openCenter/backups/$(date +%Y%m%d-%H%M%S)"

mkdir -p "$BACKUP_DIR"

# Backup cluster directory
cp -r "${HOME}/.config/openCenter/clusters/${CLUSTER_NAME}" \
      "$BACKUP_DIR/"

echo "Backup created: $BACKUP_DIR"
```

### Step 3: Run Migration

```bash
# Migrate single cluster to default organization
./bin/openCenter config migrate $CLUSTER_NAME

# Migrate to specific organization
./bin/openCenter config migrate $CLUSTER_NAME --organization my-org

# Migrate all legacy clusters
./bin/openCenter config migrate --all
```

### Step 4: Verify Migration

```bash
# Check new structure
ls -la ~/.config/openCenter/clusters/opencenter/$CLUSTER_NAME/

# Verify configuration
./bin/openCenter config show $CLUSTER_NAME

# Validate migrated configuration
./bin/openCenter config validate $CLUSTER_NAME
```

### Step 5: Update GitOps Repository Path

After migration, update GitOps repository references:

```bash
# Check current GitOps path
./bin/openCenter config show $CLUSTER_NAME | grep git_dir

# Update if needed
vim ~/.config/openCenter/clusters/opencenter/$CLUSTER_NAME/.$CLUSTER_NAME-config.yaml

# Update git_dir to point to organization root
# opencenter:
#   gitops:
#     git_dir: /home/user/gitops/opencenter
```

### Step 6: Clean Up Legacy Structure

```bash
# After verifying migration success, remove legacy directory
LEGACY_DIR="${HOME}/.config/openCenter/clusters/${CLUSTER_NAME}"

if [ -d "$LEGACY_DIR" ]; then
  echo "Removing legacy directory: $LEGACY_DIR"
  rm -rf "$LEGACY_DIR"
fi
```

## Task 2: Migrate Between Cloud Providers

### Step 1: Export Current Configuration

```bash
# Export current cluster configuration
CLUSTER_NAME="openstack-cluster"
./bin/openCenter config show $CLUSTER_NAME --format yaml > current-config.yaml

# Review configuration
cat current-config.yaml
```

### Step 2: Create Target Provider Configuration

```bash
# Initialize new cluster for target provider
./bin/openCenter cluster init aws-cluster --provider aws

# Copy service configurations from source
SOURCE_CONFIG="current-config.yaml"
TARGET_CONFIG="${HOME}/.config/openCenter/clusters/opencenter/aws-cluster/.aws-cluster-config.yaml"

# Extract and copy service configurations
yq eval '.opencenter.services' "$SOURCE_CONFIG" > services.yaml
yq eval -i '.opencenter.services = load("services.yaml")' "$TARGET_CONFIG"
```

### Step 3: Update Provider-Specific Settings

```bash
# Edit target configuration
vim "$TARGET_CONFIG"

# Update infrastructure section for AWS:
# opencenter:
#   infrastructure:
#     provider: aws
#     aws:
#       region: us-east-1
#       vpc_id: vpc-xxxxx
#       subnet_ids:
#         - subnet-xxxxx
#         - subnet-yyyyy
#       instance_type: t3.large
#       ami_id: ami-xxxxx
```

### Step 4: Validate Target Configuration

```bash
# Validate new configuration
./bin/openCenter config validate aws-cluster

# Run preflight checks
./bin/openCenter cluster preflight aws-cluster
```

### Step 5: Generate GitOps Repository

```bash
# Generate GitOps repo for new cluster
./bin/openCenter cluster setup aws-cluster --render

# Review generated manifests
ls -la ~/gitops/aws-cluster/
```

### Step 6: Migrate Workloads

```bash
# Export workloads from source cluster
kubectl config use-context openstack-cluster
kubectl get all -A -o yaml > workloads-export.yaml

# Apply to target cluster (after bootstrap)
kubectl config use-context aws-cluster
kubectl apply -f workloads-export.yaml
```

## Task 3: Upgrade Configuration Schema

### Step 1: Check Current Schema Version

```bash
# Check configuration version
./bin/openCenter config show $CLUSTER_NAME | grep version

# Example output:
# opencenter:
#   version: "1.0"
```

### Step 2: Backup Before Upgrade

```bash
# Create backup
CLUSTER_NAME="my-cluster"
BACKUP_FILE="${HOME}/.config/openCenter/backups/${CLUSTER_NAME}-$(date +%Y%m%d-%H%M%S).yaml"

mkdir -p "$(dirname "$BACKUP_FILE")"
cp "${HOME}/.config/openCenter/clusters/opencenter/${CLUSTER_NAME}/.${CLUSTER_NAME}-config.yaml" \
   "$BACKUP_FILE"

echo "Backup created: $BACKUP_FILE"
```

### Step 3: Run Schema Migration

```bash
# Migrate to latest schema version
./bin/openCenter config migrate-schema $CLUSTER_NAME

# Migrate to specific version
./bin/openCenter config migrate-schema $CLUSTER_NAME --target-version 2.0
```

### Step 4: Review Changes

```bash
# Show differences
diff "$BACKUP_FILE" \
     "${HOME}/.config/openCenter/clusters/opencenter/${CLUSTER_NAME}/.${CLUSTER_NAME}-config.yaml"

# Validate upgraded configuration
./bin/openCenter config validate $CLUSTER_NAME
```

### Step 5: Update GitOps Repository

```bash
# Regenerate GitOps manifests with new schema
./bin/openCenter cluster setup $CLUSTER_NAME --render --force

# Review changes
cd ~/gitops/$CLUSTER_NAME
git diff
```

## Task 4: Migrate Secrets and Keys

### Step 1: Export Existing Keys

```bash
# Export Age keys
CLUSTER_NAME="my-cluster"
AGE_KEY_PATH="${HOME}/.config/openCenter/clusters/opencenter/${CLUSTER_NAME}/secrets/age/${CLUSTER_NAME}-key.txt"

# Backup Age key
cp "$AGE_KEY_PATH" "${AGE_KEY_PATH}.backup"

# Export SSH keys
SSH_KEY_PATH="${HOME}/.config/openCenter/clusters/opencenter/${CLUSTER_NAME}/secrets/ssh"
tar -czf ssh-keys-backup.tar.gz -C "$SSH_KEY_PATH" .
```

### Step 2: Migrate to New Cluster

```bash
# Copy Age key to new cluster
NEW_CLUSTER="new-cluster"
NEW_AGE_KEY_PATH="${HOME}/.config/openCenter/clusters/opencenter/${NEW_CLUSTER}/secrets/age/${NEW_CLUSTER}-key.txt"

mkdir -p "$(dirname "$NEW_AGE_KEY_PATH")"
cp "$AGE_KEY_PATH" "$NEW_AGE_KEY_PATH"

# Copy SSH keys
NEW_SSH_KEY_PATH="${HOME}/.config/openCenter/clusters/opencenter/${NEW_CLUSTER}/secrets/ssh"
mkdir -p "$NEW_SSH_KEY_PATH"
tar -xzf ssh-keys-backup.tar.gz -C "$NEW_SSH_KEY_PATH"
```

### Step 3: Re-encrypt Secrets

```bash
# Export decrypted secrets from source
export SOPS_AGE_KEY_FILE="$AGE_KEY_PATH"
sops -d ~/gitops/$CLUSTER_NAME/secrets.yaml > secrets-plaintext.yaml

# Re-encrypt for new cluster
export SOPS_AGE_KEY_FILE="$NEW_AGE_KEY_PATH"
sops -e secrets-plaintext.yaml > ~/gitops/$NEW_CLUSTER/secrets.yaml

# Securely delete plaintext
shred -u secrets-plaintext.yaml
```

### Step 4: Update SOPS Configuration

```bash
# Update .sops.yaml in new GitOps repo
cd ~/gitops/$NEW_CLUSTER

cat > .sops.yaml <<EOF
creation_rules:
  - path_regex: .*\.yaml$
    encrypted_regex: ^(data|stringData|password|secret|token)$
    age: $(cat "$NEW_AGE_KEY_PATH" | grep "public key:" | cut -d: -f2 | tr -d ' ')
EOF

git add .sops.yaml
git commit -m "chore: update SOPS configuration for new cluster"
```

## Task 5: Migrate GitOps Repository

### Step 1: Clone Existing Repository

```bash
# Clone source GitOps repository
git clone https://github.com/myorg/gitops-source.git
cd gitops-source
```

### Step 2: Create New Repository Structure

```bash
# Create new repository
mkdir -p ~/gitops-new
cd ~/gitops-new
git init

# Copy base structure
cp -r ~/gitops-source/applications ./
cp -r ~/gitops-source/infrastructure ./
cp ~/gitops-source/README.md ./
```

### Step 3: Update Cluster References

```bash
# Update cluster name references
OLD_CLUSTER="old-cluster"
NEW_CLUSTER="new-cluster"

# Find and replace cluster names
find . -type f -name "*.yaml" -exec sed -i "s/${OLD_CLUSTER}/${NEW_CLUSTER}/g" {} +

# Update namespace references if needed
find . -type f -name "*.yaml" -exec sed -i "s/namespace: ${OLD_CLUSTER}/namespace: ${NEW_CLUSTER}/g" {} +
```

### Step 4: Commit and Push

```bash
# Commit changes
git add .
git commit -m "feat: migrate GitOps repository for ${NEW_CLUSTER}"

# Add remote and push
git remote add origin https://github.com/myorg/gitops-new.git
git push -u origin main
```

### Step 5: Update Cluster Configuration

```bash
# Update GitOps repository URL in cluster config
vim ~/.config/openCenter/clusters/opencenter/${NEW_CLUSTER}/.${NEW_CLUSTER}-config.yaml

# Update:
# opencenter:
#   gitops:
#     git_dir: /home/user/gitops-new
#     git_url: https://github.com/myorg/gitops-new.git
```

## Task 6: Rollback Migration

### Step 1: Identify Backup

```bash
# List available backups
ls -la ~/.config/openCenter/backups/

# Identify backup to restore
BACKUP_DIR="${HOME}/.config/openCenter/backups/20240115-103000"
```

### Step 2: Stop Cluster Operations

```bash
# Ensure no operations are running
ps aux | grep openCenter

# Stop any running operations
# pkill -f openCenter
```

### Step 3: Restore Configuration

```bash
# Restore cluster configuration
CLUSTER_NAME="my-cluster"
CURRENT_CONFIG="${HOME}/.config/openCenter/clusters/opencenter/${CLUSTER_NAME}"

# Remove current configuration
rm -rf "$CURRENT_CONFIG"

# Restore from backup
cp -r "$BACKUP_DIR/${CLUSTER_NAME}" \
      "${HOME}/.config/openCenter/clusters/"

echo "Configuration restored from backup"
```

### Step 4: Verify Restoration

```bash
# Validate restored configuration
mise run build
./bin/openCenter config validate $CLUSTER_NAME

# Check cluster status
./bin/openCenter cluster status $CLUSTER_NAME
```

### Step 5: Restore GitOps Repository

```bash
# Restore GitOps repository from backup
cd ~/gitops/$CLUSTER_NAME
git reset --hard HEAD~1  # Undo last commit

# Or restore from backup
rm -rf ~/gitops/$CLUSTER_NAME
cp -r "$BACKUP_DIR/gitops-${CLUSTER_NAME}" ~/gitops/$CLUSTER_NAME
```

## Task 7: Migrate Multiple Clusters

### Step 1: Create Migration Plan

```bash
# List all clusters
./bin/openCenter config list > clusters.txt

# Create migration plan
cat > migration-plan.txt <<EOF
# Migration Plan
# Date: $(date)

Clusters to migrate:
$(cat clusters.txt)

Migration order:
1. Development clusters
2. Staging clusters
3. Production clusters

Rollback plan:
- Backups stored in: ~/.config/openCenter/backups/
- Rollback window: 24 hours
- Validation required before next cluster
EOF

cat migration-plan.txt
```

### Step 2: Create Migration Script

```bash
cat > migrate-all-clusters.sh <<'EOF'
#!/bin/bash

set -e

CLUSTERS_FILE="clusters.txt"
LOG_FILE="migration-$(date +%Y%m%d-%H%M%S).log"

echo "Starting cluster migration..." | tee -a "$LOG_FILE"
echo "Log file: $LOG_FILE" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

while IFS= read -r cluster; do
  echo "Migrating cluster: $cluster" | tee -a "$LOG_FILE"
  
  # Backup
  echo "  Creating backup..." | tee -a "$LOG_FILE"
  BACKUP_DIR="${HOME}/.config/openCenter/backups/$(date +%Y%m%d-%H%M%S)-${cluster}"
  mkdir -p "$BACKUP_DIR"
  cp -r "${HOME}/.config/openCenter/clusters/${cluster}" "$BACKUP_DIR/" || {
    echo "  ❌ Backup failed for $cluster" | tee -a "$LOG_FILE"
    continue
  }
  
  # Migrate
  echo "  Running migration..." | tee -a "$LOG_FILE"
  if ./bin/openCenter config migrate "$cluster" >> "$LOG_FILE" 2>&1; then
    echo "  ✓ Migration successful" | tee -a "$LOG_FILE"
    
    # Validate
    echo "  Validating..." | tee -a "$LOG_FILE"
    if ./bin/openCenter config validate "$cluster" >> "$LOG_FILE" 2>&1; then
      echo "  ✓ Validation successful" | tee -a "$LOG_FILE"
    else
      echo "  ⚠️  Validation failed, rolling back..." | tee -a "$LOG_FILE"
      rm -rf "${HOME}/.config/openCenter/clusters/opencenter/${cluster}"
      cp -r "$BACKUP_DIR/${cluster}" "${HOME}/.config/openCenter/clusters/"
    fi
  else
    echo "  ❌ Migration failed for $cluster" | tee -a "$LOG_FILE"
  fi
  
  echo "" | tee -a "$LOG_FILE"
done < "$CLUSTERS_FILE"

echo "Migration complete. See $LOG_FILE for details."
EOF

chmod +x migrate-all-clusters.sh
```

### Step 3: Run Batch Migration

```bash
# Run migration script
./migrate-all-clusters.sh

# Monitor progress
tail -f migration-*.log
```

### Step 4: Verify All Migrations

```bash
# Verify all migrated clusters
for cluster in $(cat clusters.txt); do
  echo "Verifying: $cluster"
  ./bin/openCenter config validate "$cluster"
  echo ""
done
```

## Best Practices

1. **Always Backup**: Create backups before any migration
2. **Test First**: Test migration on non-production clusters first
3. **Validate After**: Always validate configuration after migration
4. **Document Changes**: Keep detailed migration logs
5. **Plan Rollback**: Have rollback procedures ready
6. **Migrate Incrementally**: Migrate one cluster at a time for production
7. **Verify Secrets**: Ensure secrets are properly migrated and encrypted
8. **Update Documentation**: Update cluster documentation after migration
9. **Monitor Post-Migration**: Monitor clusters closely after migration
10. **Retain Backups**: Keep backups for at least 30 days

## Troubleshooting

### Migration Fails with Permission Error

**Problem**: `Error: permission denied`

**Solution**: Check directory permissions:
```bash
chmod -R u+w ~/.config/openCenter/clusters/
```

### Configuration Validation Fails After Migration

**Problem**: Validation errors after migration

**Solution**: Review and fix validation errors:
```bash
# Show detailed validation errors
./bin/openCenter config validate $CLUSTER_NAME --verbose

# Compare with backup
diff ~/.config/openCenter/backups/*/my-cluster/.my-cluster-config.yaml \
     ~/.config/openCenter/clusters/opencenter/my-cluster/.my-cluster-config.yaml
```

### GitOps Repository Path Incorrect

**Problem**: GitOps repository not found after migration

**Solution**: Update git_dir in configuration:
```bash
vim ~/.config/openCenter/clusters/opencenter/$CLUSTER_NAME/.$CLUSTER_NAME-config.yaml

# Update git_dir to correct path
# opencenter:
#   gitops:
#     git_dir: /correct/path/to/gitops
```

### Secrets Cannot Be Decrypted

**Problem**: SOPS cannot decrypt secrets after migration

**Solution**: Verify Age key is correctly migrated:
```bash
# Check Age key exists
ls -la ~/.config/openCenter/clusters/opencenter/$CLUSTER_NAME/secrets/age/

# Test decryption
export SOPS_AGE_KEY_FILE=~/.config/openCenter/clusters/opencenter/$CLUSTER_NAME/secrets/age/$CLUSTER_NAME-key.txt
sops -d ~/gitops/$CLUSTER_NAME/secrets.yaml
```

## Next Steps

- [Configuration Management](./configuration-management.md) - Learn configuration best practices
- [Disaster Recovery](./disaster-recovery.md) - Implement backup and restore
- [GitOps Workflows](./gitops-workflows.md) - Manage GitOps repositories
- [Multi-Cluster Management](./multi-cluster.md) - Manage multiple clusters
