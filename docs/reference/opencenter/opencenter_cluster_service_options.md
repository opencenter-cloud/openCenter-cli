---
id: "opencenter-cluster-service-options"
title: "Opencenter_Cluster_Service_Options"
sidebar_label: "Opencenter_Cluster_Service_Options"
description: "Display available configuration options for a service"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster service options

**Purpose:** For operators, developers, documents `opencenter cluster service options`: what it does, its flags, and its examples.

Display available configuration options for a service

### Synopsis

This command displays all available configuration parameters and secrets for a service.
It shows the field names, types, descriptions, and whether they are required.

Examples:
  # Show options for cert-manager
  opencenter cluster service options cert-manager

  # Show options for loki
  opencenter cluster service options loki

  # Show options for a managed service
  opencenter cluster service options alert-proxy --managed

```
opencenter cluster service options <service-name> [flags]
```

### Options

```
  -h, --help      help for options
      --managed   Show options for a managed service
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

* [opencenter cluster service](opencenter_cluster_service.md)	 - Manage cluster services
