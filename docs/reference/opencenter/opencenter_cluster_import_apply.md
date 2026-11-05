---
id: "opencenter-cluster-import-apply"
title: "Opencenter_Cluster_Import_Apply"
sidebar_label: "Opencenter_Cluster_Import_Apply"
description: "Create or patch openCenter configs from the latest import artifact"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster import apply

**Purpose:** For operators, developers, documents `opencenter cluster import apply`: what it does, its flags, and its examples.

Create or patch openCenter configs from the latest import artifact

```
opencenter cluster import apply [flags]
```

### Options

```
  -h, --help          help for apply
      --repo string   Path to the customer GitOps repository
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

* [opencenter cluster import](opencenter_cluster_import.md)	 - Import running clusters into openCenter config
