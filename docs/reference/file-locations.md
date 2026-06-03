---
id: file-locations
title: "File Locations"
sidebar_label: File Locations
description: Complete reference of configuration and data file locations used by openCenter CLI.
doc_type: reference
audience: "all users"
tags: [files, paths, locations, configuration]
---
# File Locations

**Purpose:** For all users, provides complete reference of configuration and data file locations used by openCenter CLI.

This reference documents the file and directory locations openCenter uses for configuration, runtime state, caches, and generated files.

## Overview

openCenter CLI separates persistent configuration from mutable runtime state:

* **Configuration:** `~/.config/opencenter/`
* **State:** `~/.local/state/opencenter/`
* **Cache:** `~/.cache/opencenter/`
* **GitOps repository output:** user-selected working tree

## Configuration Directory

### Base Configuration Directory

**Location:** `~/.config/opencenter/`

**Purpose:** Stores all openCenter configuration files.

**Contents:**

```
~/.config/opencenter/
├── config.yaml              # CLI defaults
├── clusters/                # Cluster configurations
└── plugins/                 # CLI plugins
```

**Environment variables:** `OPENCENTER_CONFIG_DIR`, `OPENCENTER_CLUSTERS_DIR`, `OPENCENTER_PLUGINS_DIR`

**Override:**

```bash
export OPENCENTER_CONFIG_DIR=/custom/opencenter-config
export OPENCENTER_CLUSTERS_DIR=/custom/opencenter-clusters
export OPENCENTER_PLUGINS_DIR=/custom/opencenter-plugins
```

### CLI Defaults

**Location:** `~/.config/opencenter/config.yaml`

**Purpose:** User-level CLI defaults (applied to all clusters).

**Example:**

```yaml
# CLI defaults
opencenter:
  cluster:
    kubernetes:
      version: "1.33.5"
    networking:
      cni_plugin: calico
```

**Precedence:** Lower than configuration file, higher than built-in defaults.

### Cluster Configurations

**Location:** `~/.config/opencenter/clusters/<organization>/<cluster>/.<cluster>-config.yaml`

**Purpose:** Cluster-specific configuration.

**Example path:**

```
~/.config/opencenter/clusters/my-company/.prod-cluster-config.yaml
```

**Pattern:**

* Organization directory: `<organization>/`
* Cluster config: `.<cluster>-config.yaml` (hidden file)

**Evidence:** `.kiro/steering/structure.md:118-128`, `tests/features/workflow.feature:19`

### Organization Structure

**Location:** `~/.config/opencenter/clusters/<organization>/`

**Purpose:** Organization-level configuration and secrets.

**Contents:**

```
~/.config/opencenter/clusters/my-company/
├── .defaults.yaml           # Organization defaults
├── .dev-config.yaml         # Dev cluster config
├── .staging-config.yaml     # Staging cluster config
├── .prod-config.yaml        # Production cluster config
└── secrets/
    ├── age/                 # SOPS Age keys
    │   ├── org-key.txt      # Organization key
    │   ├── dev-key.txt      # Dev cluster key
    │   ├── staging-key.txt  # Staging cluster key
    │   └── prod-key.txt     # Production cluster key
    └── ssh/                 # SSH keys
        ├── org-key          # Organization SSH key
        ├── dev-key          # Dev cluster SSH key
        ├── staging-key      # Staging cluster SSH key
        └── prod-key         # Production cluster SSH key
```

**Evidence:** Session 2 B0 section 14

## Secrets Directory

### SOPS Age Keys

**Location:** `~/.config/opencenter/clusters/<organization>/secrets/age/<cluster>-key.txt`

**Purpose:** SOPS Age encryption keys for secrets management.

**Example path:**

```
~/.config/opencenter/clusters/my-company/secrets/age/prod-cluster-key.txt
```

**Format:**

```
# created: 2026-02-17T10:00:00Z
# public key: age1abc123...
AGE-SECRET-KEY-1ABC123...
```

**Permissions:** `0600` (read/write for owner only)

**Evidence:** `internal/sops/manager.go`, Session 1 A11

### SSH Keys

**Location:** `~/.config/opencenter/clusters/<organization>/secrets/ssh/<cluster>-key`

**Purpose:** SSH keys for cluster node access.

**Example paths:**

```
~/.config/opencenter/clusters/my-company/secrets/ssh/prod-cluster-key
~/.config/opencenter/clusters/my-company/secrets/ssh/prod-cluster-key.pub
```

**Permissions:**

* Private key: `0600` (read/write for owner only)
* Public key: `0644` (readable by all)

**Evidence:** `internal/config/defaults.go`, Session 2 B0 section 14

## GitOps Repository

### Repository Location

**Location:** User-specified (typically `~/my-cluster-gitops/`)

**Purpose:** Generated GitOps repository with infrastructure and application manifests.

**Structure:**

```
~/my-cluster-gitops/
├── .gitignore
├── .sops.yaml               # SOPS encryption rules
├── README.md
│
├── applications/
│   └── overlays/<cluster>/
│       ├── flux-system/     # FluxCD bootstrap
│       ├── services/        # Platform services
│       └── managed-services/ # Customer applications
│
└── infrastructure/
    └── clusters/<cluster>/
        ├── main.tf          # Terraform/OpenTofu
        ├── inventory/       # Kubespray Ansible
        └── kubeconfig.yaml  # Generated after deployment
```

**Evidence:** `internal/gitops/`, Session 2 B0 section 15, Ecosystem.md

### Infrastructure Files

**Location:** `~/my-cluster-gitops/infrastructure/clusters/<cluster>/`

**Purpose:** Infrastructure provisioning and configuration.

**Contents:**

```
infrastructure/clusters/my-cluster/
├── main.tf                  # Terraform main configuration
├── provider.tf              # Provider configuration
├── variables.tf             # Terraform variables
├── outputs.tf               # Terraform outputs
├── inventory/               # Kubespray inventory
│   ├── inventory.yaml       # Ansible inventory
│   ├── group_vars/          # Ansible group variables
│   │   ├── all.yml
│   │   ├── k8s_cluster.yml
│   │   └── k8s_hardening.yml
│   └── credentials/         # Encrypted credentials
│       └── clouds.yaml      # OpenStack credentials (encrypted)
└── kubeconfig.yaml          # Kubernetes config (generated)
```

### Application Files

**Location:** `~/my-cluster-gitops/applications/overlays/<cluster>/`

**Purpose:** Kubernetes application manifests.

**Contents:**

```
applications/overlays/my-cluster/
├── kustomization.yaml       # Kustomize overlay
├── flux-system/             # FluxCD bootstrap
│   ├── gotk-components.yaml
│   └── gotk-sync.yaml
├── services/                # Platform services
│   ├── sources/             # GitRepository sources
│   │   ├── opencenter-cert-manager.yaml
│   │   └── ...
│   ├── fluxcd/              # Kustomization resources
│   │   ├── cert-manager.yaml
│   │   └── ...
│   └── <service>/           # Service-specific overrides
│       ├── kustomization.yaml
│       └── override-values.yaml
└── managed-services/        # Customer applications
    ├── sources/
    ├── fluxcd/
    └── <app>/
```

## Cache Directory

### Base Cache Directory

**Location:** `~/.cache/opencenter/`

**Purpose:** Temporary cache files for performance optimization.

**Contents:**

```
~/.cache/opencenter/
├── provider-cache/          # Provider API response cache
├── schema-cache/            # Schema validation cache
└── template-cache/          # Template rendering cache
```

**Cleanup:**

```bash
# Clear cache
rm -rf ~/.cache/opencenter/

# Cache is automatically recreated
```

### Provider Cache

**Location:** `~/.cache/opencenter/provider-cache/`

**Purpose:** Cache provider API responses (images, flavors, networks).

**Example:**

```
~/.cache/opencenter/provider-cache/
├── openstack-sjc3-images.json
├── openstack-sjc3-flavors.json
└── openstack-sjc3-networks.json
```

**TTL:** 1 hour (configurable)

## State Directory

### Base State Directory

**Location:** `~/.local/state/opencenter/`

**Purpose:** Runtime artifacts that should not dirty the GitOps repository.

**Contents:**

```
~/.local/state/opencenter/
├── audit/
│   └── audit.log            # Default audit log
├── bootstrap/
│   └── <organization>/<cluster>/
│       └── state.json       # Bootstrap resume checkpoint
├── locks/                   # File locks for cluster operations
└── logs/
    └── bootstrap/
        └── <organization>/<cluster>/
            └── bootstrap-YYYYMMDDTHHMMSSZ.log
```

**Environment variables:**

* `OPENCENTER_STATE_DIR`
* `XDG_STATE_HOME` (fallback base when `OPENCENTER_STATE_DIR` is unset)

### Bootstrap Resume State

**Location:** `~/.local/state/opencenter/bootstrap/<organization>/<cluster>/state.json`

**Purpose:** Stores resumable bootstrap step state after a failed or interrupted bootstrap.

**Behavior:**

* Created during bootstrap when a resumable step is recorded
* Deleted automatically after a successful bootstrap
* Legacy repo-local state at `infrastructure/clusters/<cluster>/logs/bootstrap-state.json` is still read for compatibility during migration

**Permissions:** `0600`

### Bootstrap Logs

**Location:** `~/.local/state/opencenter/logs/bootstrap/<organization>/<cluster>/bootstrap-YYYYMMDDTHHMMSSZ.log`

**Purpose:** Per-run bootstrap logs written outside the GitOps repository.

**Example:**

```
~/.local/state/opencenter/logs/bootstrap/my-company/prod-cluster/
└── bootstrap-20260411T154500Z.log
```

**Override:** `opencenter cluster deploy --log /path/to/file.log`

**Permissions:** `0600`

### Audit Log

**Location:** `~/.local/state/opencenter/audit/audit.log`

**Purpose:** Default append-only audit log for security-sensitive operations.

### Audit Signing Key

**Location:** `~/.config/opencenter/audit/audit.key`

**Purpose:** 32-byte HMAC-SHA256 key used to sign audit log entries for tamper detection. Generated automatically on first audit write. See [Audit Signing Key](audit-key.md) for details.

**Permissions:** `0600`

### File Locks

**Location:** `~/.local/state/opencenter/locks/`

**Purpose:** Prevents concurrent cluster mutations against the same target.

## Plugin Directory

### Plugin Location

**Location:** `~/.config/opencenter/plugins/`

**Purpose:** CLI plugins and extensions.

**Structure:**

```
~/.config/opencenter/plugins/
├── checksums.txt            # Optional sha256sum allowlist
├── opencenter-plugin-name   # Plugin executable
└── opencenter-plugin-other  # Another plugin
```

**Discovery:** Plugins must be named `opencenter-<plugin-name>` and be executable.

**Verification:** `checksums.txt`, when present, uses standard `sha256sum` formatting (`<sha256>  <filename>`, two spaces) and is matched by plugin basename. Unverified plugins emit a warning; mismatched checksums block execution.

**See also:** [Create and Install a CLI Plugin](../operations/create-install-cli-plugin.md)

**Evidence:** `internal/plugins/`, `cmd/plugins.go`

## Kubeconfig Location

### Generated Kubeconfig

**Location:** `~/my-cluster-gitops/infrastructure/clusters/<cluster>/kubeconfig.yaml`

**Purpose:** Kubernetes cluster access configuration.

**Generated by:** Kubespray during cluster deployment

**Usage:**

```bash
export KUBECONFIG=~/my-cluster-gitops/infrastructure/clusters/my-cluster/kubeconfig.yaml
kubectl get nodes
```

**Permissions:** `0600` (read/write for owner only)

## Environment Variable Overrides

Supported location overrides:

| Location | Environment Variable | Default |
| --- | --- | --- |
| Config directory | `OPENCENTER_CONFIG_DIR` | `~/.config/opencenter` |
| Cluster directory | `OPENCENTER_CLUSTERS_DIR` | `${OPENCENTER_CONFIG_DIR:-~/.config/opencenter}/clusters` |
| Plugins directory | `OPENCENTER_PLUGINS_DIR` | `${OPENCENTER_CONFIG_DIR:-~/.config/opencenter}/plugins` |
| State directory | `OPENCENTER_STATE_DIR` | `${XDG_STATE_HOME:-~/.local/state}/opencenter` |

**Example:**

```bash
export OPENCENTER_CONFIG_DIR=/custom/config
export OPENCENTER_CLUSTERS_DIR=/custom/clusters
export OPENCENTER_PLUGINS_DIR=/custom/plugins
export OPENCENTER_STATE_DIR=/custom/state
```

## File Permissions

### Recommended Permissions

| File Type | Permissions | Reason |
| --- | --- | --- |
| Configuration files | `0600` | Contains sensitive data |
| SSH private keys | `0600` | Security requirement |
| SSH public keys | `0644` | Can be shared |
| SOPS Age keys | `0600` | Security requirement |
| Kubeconfig | `0600` | Contains cluster credentials |
| Runtime logs | `0600` | May contain command output and error details |
| Runtime state | `0600` | Resume checkpoints may contain sensitive context |
| Cache files | `0644` | Non-sensitive |

### Setting Permissions

```bash
# Configuration files
chmod 600 ~/.config/opencenter/clusters/my-org/.prod-cluster-config.yaml

# SSH keys
chmod 600 ~/.config/opencenter/clusters/my-org/secrets/ssh/prod-cluster-key
chmod 644 ~/.config/opencenter/clusters/my-org/secrets/ssh/prod-cluster-key.pub

# SOPS Age keys
chmod 600 ~/.config/opencenter/clusters/my-org/secrets/age/prod-cluster-key.txt

# Kubeconfig
chmod 600 ~/prod-cluster-gitops/infrastructure/clusters/prod-cluster/kubeconfig.yaml
```

## Backup Recommendations

### Critical Files to Backup

1. **Cluster configurations:** `~/.config/opencenter/clusters/`
2. **SOPS Age keys:** `~/.config/opencenter/clusters/*/secrets/age/`
3. **SSH keys:** `~/.config/opencenter/clusters/*/secrets/ssh/`
4. **GitOps repository:** `~/my-cluster-gitops/`

### Backup Command

```bash
# Backup all openCenter data
tar -czf opencenter-backup-$(date +%Y%m%d).tar.gz \
  ~/.config/opencenter/ \
  ~/my-cluster-gitops/

# Restore from backup
tar -xzf opencenter-backup-20260217.tar.gz -C ~/
```

### Exclude from Backup

* Cache directory: `~/.cache/opencenter/`
* Temporary files: `/tmp/opencenter/`
* Logs: `~/.local/share/opencenter/logs/`

## Cleanup

### Safe to Delete

```bash
# Cache (will be recreated)
rm -rf ~/.cache/opencenter/

# Temporary files (will be recreated)
rm -rf /tmp/opencenter/

# Old logs (keep recent logs)
find ~/.local/share/opencenter/logs/ -name "*.log.*" -mtime +7 -delete
```

### Do Not Delete

* Configuration files: `~/.config/opencenter/clusters/`
* Secrets: `~/.config/opencenter/clusters/*/secrets/`
* GitOps repository: `~/my-cluster-gitops/`

## Troubleshooting

### Configuration File Not Found

**Symptom:** `Error: Configuration file not found`

**Diagnosis:**

```bash
# Check configuration directory
ls -la ~/.config/opencenter/clusters/my-org/

# Check expected path
echo ~/.config/opencenter/clusters/my-org/.prod-cluster-config.yaml
```

**Solution:**

```bash
# Verify cluster name and organization
opencenter cluster list

# Initialize cluster if missing
opencenter cluster init prod-cluster --org my-org
```

### Permission Denied

**Symptom:** `Error: Permission denied`

**Diagnosis:**

```bash
# Check file permissions
ls -l ~/.config/opencenter/clusters/my-org/.prod-cluster-config.yaml

# Check directory permissions
ls -ld ~/.config/opencenter/clusters/my-org/
```

**Solution:**

```bash
# Fix file permissions
chmod 600 ~/.config/opencenter/clusters/my-org/.prod-cluster-config.yaml

# Fix directory permissions
chmod 700 ~/.config/opencenter/clusters/my-org/
```

### Disk Space Issues

**Symptom:** `Error: No space left on device`

**Diagnosis:**

```bash
# Check disk usage
df -h ~/.config/opencenter/
df -h ~/.cache/opencenter/
df -h ~/.local/share/opencenter/
```

**Solution:**

```bash
# Clear cache
rm -rf ~/.cache/opencenter/

# Clear old logs
find ~/.local/share/opencenter/logs/ -name "*.log.*" -mtime +7 -delete

# Clear old backups
find ~/.local/share/opencenter/backups/ -name "*.yaml" -mtime +30 -delete
```

## Related Topics

* [Configuration Schema](configuration-schema.md) - Configuration file structure
* [CLI Commands](cli-commands.md) - Command reference
* [Environment Variables](environment-variables.md) - Environment variable reference
* [Manage Secrets](../operations/manage-secrets.md) - Secrets management

---

## Evidence

This reference is based on:

* File locations: `.kiro/steering/structure.md:118-128`, Session 2 B0 section 14
* Configuration path: `tests/features/workflow.feature:19`
* GitOps structure: `internal/gitops/`, Session 2 B0 section 15, Ecosystem.md
* Secrets management: `internal/sops/manager.go`, Session 1 A11
* Plugin system: `internal/plugins/`, `cmd/plugins.go`