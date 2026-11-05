---
id: "opencenter-cluster-edit"
title: "Opencenter_Cluster_Edit"
sidebar_label: "Opencenter_Cluster_Edit"
description: "Edit a cluster configuration in your preferred editor"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster edit

**Purpose:** For operators, developers, documents `opencenter cluster edit`: what it does, its flags, and its examples.

Edit a cluster configuration in your preferred editor

### Synopsis

Edit a cluster configuration file in your preferred editor.

If no cluster name is provided, the currently selected cluster is edited.
The editor is determined by the EDITOR or VISUAL environment variables,
falling back to 'vi' if neither is set.

Examples:
  # Edit the currently selected cluster
  opencenter cluster edit

  # Edit a specific cluster
  opencenter cluster edit my-cluster

  # Edit a cluster in a specific organization
  opencenter cluster edit myorg/my-cluster

```
opencenter cluster edit [name] [flags]
```

### Options

```
  -h, --help   help for edit
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
