---
id: "opencenter-cluster-import"
title: "Opencenter_Cluster_Import"
sidebar_label: "Opencenter_Cluster_Import"
description: "Import running clusters into openCenter config"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster import

**Purpose:** For operators, developers, documents `opencenter cluster import`: what it does, its flags, and its examples.

Import running clusters into openCenter config

### Synopsis

Discover cluster metadata from GitOps sources with kubeconfig fallback,
persist the discovery artifact, and create or patch openCenter cluster configs.

```
opencenter cluster import [flags]
```

### Options

```
  -h, --help   help for import
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
* [opencenter cluster import apply](opencenter_cluster_import_apply.md)	 - Create or patch openCenter configs from the latest import artifact
* [opencenter cluster import report](opencenter_cluster_import_report.md)	 - Render the latest cluster import artifact for a repo
* [opencenter cluster import scan](opencenter_cluster_import_scan.md)	 - Scan a customer GitOps repo and persist a cluster import artifact
