---
id: "opencenter-cluster-use"
title: "Opencenter_Cluster_Use"
sidebar_label: "Opencenter_Cluster_Use"
description: "Set the active cluster"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster use

**Purpose:** For operators, developers, documents `opencenter cluster use`: what it does, its flags, and its examples.

Set the active cluster

### Synopsis

Set the active cluster and display comprehensive information including:
- Cluster metadata (name, environment, region, status, organization)
- GitOps repository paths and structure
- Cluster-specific paths (SOPS keys, configuration files)
- Environment setup commands for shell configuration

By default, the active cluster is session-scoped (current terminal only) when shell
integration is enabled via: eval "$(opencenter shell-init)"

Use --persistent to set a global active cluster that affects all terminals.

If no cluster name is provided, an interactive selection menu is displayed.
For deployed clusters, environment setup commands are generated to configure
KUBECONFIG, ANSIBLE_INVENTORY, virtual environment, and PATH variables.

Use --clear to deactivate the current session cluster.
Use --clear-persistent to remove the persistent active cluster.

```
opencenter cluster use [name] [flags]
```

### Options

```
      --clear              Clear the active cluster (deactivate session)
      --clear-persistent   Clear the persistent active cluster
  -h, --help               help for use
      --persistent         Set persistent active cluster (affects all terminals)
      --shell string       Override shell detection (bash, zsh, fish, powershell)
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
