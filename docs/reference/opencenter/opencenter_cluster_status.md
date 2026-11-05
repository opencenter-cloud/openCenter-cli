---
id: "opencenter-cluster-status"
title: "Opencenter_Cluster_Status"
sidebar_label: "Opencenter_Cluster_Status"
description: "Show cluster status information"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster status

**Purpose:** For operators, developers, documents `opencenter cluster status`: what it does, its flags, and its examples.

Show cluster status information

### Synopsis

Show cluster status information.

This command displays:
- The requested cluster, or the currently active cluster when no name is passed
- Basic cluster metadata (environment, region, organization)
- Cluster lifecycle state (stage and status)
- Network and node inventory from local configuration and OpenTofu state
- Key file paths (with --paths flag)

By default this command is offline and does not contact Kubernetes or provider APIs.
Use --refresh to collect live Kubernetes node IPs and API readiness.

If no cluster is requested and no cluster is active, it will show available clusters
and suggest using 'opencenter cluster use' to set one.

```
opencenter cluster status [name] [flags]
```

### Examples

```
  # Show active cluster status
  opencenter cluster status

  # Show a specific cluster
  opencenter cluster status my-cluster

  # Show active cluster with file paths
  opencenter cluster status --paths

  # Refresh status from live Kubernetes/provider checks
  opencenter cluster status my-cluster --refresh

  # Sync service status from the live cluster into configuration
  opencenter cluster status my-cluster --sync

  # Quiet output (just the cluster name)
  opencenter cluster status --quiet
```

### Options

```
  -h, --help                    help for status
      --paths                   show cluster file paths and their status
  -q, --quiet                   quiet output (just the cluster name)
      --refresh                 refresh live Kubernetes/provider status
      --sync                    sync service status from the live cluster into configuration
      --sync-timeout duration   timeout for live cluster status sync (default 30s)
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --dry-run             preview mutating operations without writing or acting
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter cluster](opencenter_cluster.md)	 - Manage cluster configurations
