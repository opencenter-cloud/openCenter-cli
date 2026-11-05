---
id: "opencenter-cluster-doctor"
title: "Opencenter_Cluster_Doctor"
sidebar_label: "Opencenter_Cluster_Doctor"
description: "Audit local executable prerequisites"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster doctor

**Purpose:** For operators, developers, documents `opencenter cluster doctor`: what it does, its flags, and its examples.

Audit local executable prerequisites

### Synopsis

Audit the local host's required executables without loading cluster configuration,
resolving an active cluster, contacting a provider, or changing state.

The audit checks a fixed all-provider catalog. tofu or terraform satisfies the
OpenTofu row, and podman or docker satisfies the container row. The openstack
CLI and external age executable are intentionally not checked.

```
opencenter cluster doctor [flags]
```

### Options

```
  -h, --help   help for doctor
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
