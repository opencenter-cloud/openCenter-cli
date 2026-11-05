---
id: "opencenter-cluster-drift-reconcile"
title: "Opencenter_Cluster_Drift_Reconcile"
sidebar_label: "Opencenter_Cluster_Drift_Reconcile"
description: "Reconcile detected infrastructure drift"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster drift reconcile

**Purpose:** For operators, developers, documents `opencenter cluster drift reconcile`: what it does, its flags, and its examples.

Reconcile detected infrastructure drift

### Synopsis

Reconcile differences between desired configuration and actual infrastructure state.

This command first detects drift, then applies changes to bring the actual infrastructure
state back in line with the desired configuration. Only reconcilable drift can be fixed
automatically. Non-reconcilable drift (e.g., deleted resources, manual resource creation)
requires manual intervention.

With --to-config, the direction reverses: observed deployed-cluster state is promoted
into the config file. --source selects the evidence (terraform: the applied OpenTofu
state, which recovers a network OpenTofu created; live: the running cluster; all: both),
and --field restricts the promoted config paths. Run 'opencenter cluster observe' first
to see the same report without writing.

Use --dry-run to see what changes would be made without applying them.

If no cluster name is provided, uses the currently active cluster.

```
opencenter cluster drift reconcile [cluster] [flags]
```

### Examples

```
  # Reconcile drift for active cluster
  opencenter cluster drift reconcile

  # Show what would be reconciled (dry-run)
  opencenter cluster drift reconcile my-cluster --dry-run

  # Apply reconciliation
  opencenter cluster drift reconcile my-cluster

  # Reconcile with confirmation prompt
  opencenter cluster drift reconcile my-cluster --confirm

  # Promote a network OpenTofu created back into the config
  opencenter cluster drift reconcile my-cluster --to-config --source terraform

  # Preview the config patch from every evidence source
  opencenter cluster drift reconcile my-cluster --to-config --source all --dry-run
```

### Options

```
      --confirm         Prompt for confirmation before applying changes
      --dry-run         Show what would be changed without applying
      --field strings   with --to-config: promote only these config paths
  -h, --help            help for reconcile
      --source string   with --to-config: evidence source: terraform, live, or all (default "live")
      --to-cluster      Reconcile infrastructure to match the current config (default)
      --to-config       Promote observed deployed-cluster state back into config
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

* [opencenter cluster drift](opencenter_cluster_drift.md)	 - Detect and reconcile infrastructure drift
