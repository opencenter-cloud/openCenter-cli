---
id: "opencenter-secrets-sync"
title: "Opencenter_Secrets_Sync"
sidebar_label: "Opencenter_Secrets_Sync"
description: "Synchronize secrets from config to encrypted manifests"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter secrets sync

**Purpose:** For operators, developers, documents `opencenter secrets sync`: what it does, its flags, and its examples.

Synchronize secrets from config to encrypted manifests

### Synopsis

Synchronize secrets from the cluster configuration file to encrypted manifests.

This command reads secrets from the cluster's config file (.k8s-<cluster>-config.yaml)
and generates corresponding SOPS-encrypted manifests for each service. It ensures that
deployed secrets match the source of truth in the configuration.

The sync operation:
  • Reads secrets from the config file
  • Generates encrypted manifests for each service
  • Preserves non-secret fields in existing manifests
  • Uses the cluster's Age key for encryption
  • Reports created, updated, and unchanged files

If no cluster name is provided, uses the currently active cluster.

Multi-cluster mode (--all flag) processes all clusters in parallel with configurable
concurrency. Use --org to filter to a specific organization.

```
opencenter secrets sync [cluster] [flags]
```

### Examples

```
  # Sync secrets for active cluster
  opencenter secrets sync

  # Sync secrets for specific cluster
  opencenter secrets sync my-cluster

  # Sync only specific services
  opencenter secrets sync my-cluster --services=cert-manager,loki

  # Preview changes without applying (dry-run)
  opencenter secrets sync my-cluster --dry-run

  # Force sync even if no drift detected
  opencenter secrets sync my-cluster --force

  # Sync all clusters in organization
  opencenter secrets sync --all --org myorg

  # Sync all clusters with custom concurrency
  opencenter secrets sync --all --concurrency=8

  # Stop on first error
  opencenter secrets sync --all --stop-on-error
```

### Options

```
      --all                Sync secrets for all clusters in organization
      --cluster string     Cluster name (uses active cluster if not specified)
      --concurrency int    Maximum number of parallel cluster syncs (used with --all) (default 4)
      --dry-run            Preview changes without applying them
      --force              Overwrite manifests even if no drift detected
  -h, --help               help for sync
      --org string         Filter to specific organization (used with --all)
      --services strings   Comma-separated list of services to sync (e.g., cert-manager,loki)
      --skip-validation    Skip configuration validation before manifest refresh
      --stop-on-error      Stop processing on first failure (used with --all)
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter secrets](opencenter_secrets.md)	 - Manage secrets across backends
