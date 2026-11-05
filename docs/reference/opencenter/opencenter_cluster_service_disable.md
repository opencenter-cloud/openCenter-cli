---
id: "opencenter-cluster-service-disable"
title: "Opencenter_Cluster_Service_Disable"
sidebar_label: "Opencenter_Cluster_Service_Disable"
description: "Disable a service in the cluster configuration"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster service disable

**Purpose:** For operators, developers, documents `opencenter cluster service disable`: what it does, its flags, and its examples.

Disable a service in the cluster configuration

### Synopsis

This command disables a service in the cluster configuration by setting its enabled flag to false.

Examples:
  # Disable the 'cert-manager' service
  opencenter cluster service disable cert-manager

  # Disable a managed service
  opencenter cluster service disable my-managed-service --managed

  # Disable a service and immediately update the rendered manifests
  opencenter cluster service disable loki --render

```
opencenter cluster service disable <service-name> [flags]
```

### Options

```
      --adopt-generated   Claim differing planned files after creating backups during --render
      --cluster string    Specify the cluster name
  -h, --help              help for disable
      --managed           Disable the service from the managed services list
      --prune             Remove stale generated files during --render (default true)
      --render            Render the cluster application manifests immediately after disabling
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
