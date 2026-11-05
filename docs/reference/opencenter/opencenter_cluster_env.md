---
id: "opencenter-cluster-env"
title: "Opencenter_Cluster_Env"
sidebar_label: "Opencenter_Cluster_Env"
description: "Export cluster environment variables"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster env

**Purpose:** For operators, developers, documents `opencenter cluster env`: what it does, its flags, and its examples.

Export cluster environment variables

### Synopsis

Export environment variables for the specified cluster or current active cluster.

This command generates shell commands to set up the cluster environment including:
- Cloud provider credentials (AWS, OpenStack)
- KUBECONFIG path
- ANSIBLE_INVENTORY path
- SOPS_AGE_KEY_FILE path
- Cluster-specific binary paths
- Virtual environment activation

If no cluster name is provided, uses the current active cluster.

The output is designed to be evaluated by your shell:
  eval "$(opencenter cluster env)"
  eval "$(opencenter cluster env my-cluster)"

This is useful for:
- Re-exporting environment variables after they've changed
- Setting up environment in a new terminal session
- Refreshing credentials that may have been updated

```
opencenter cluster env [cluster-name] [flags]
```

### Examples

```
  # Export current cluster environment
  eval "$(opencenter cluster env)"

  # Export specific cluster environment
  eval "$(opencenter cluster env prod-cluster)"

  # Export with organization
  eval "$(opencenter cluster env myorg/prod-cluster)"

  # Override shell detection
  opencenter cluster env --shell fish | source
```

### Options

```
  -h, --help           help for env
      --shell string   Override shell detection (bash, zsh, fish, powershell)
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --dry-run             preview mutating operations without writing or acting
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter cluster](opencenter_cluster.md)	 - Manage cluster configurations
