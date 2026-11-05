---
id: "opencenter"
title: "Opencenter"
sidebar_label: "Opencenter"
description: "opencenter CLI manages cluster configurations and GitOps scaffolding"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter

**Purpose:** For operators, developers, documents `opencenter`: what it does, its flags, and its examples.

opencenter CLI manages cluster configurations and GitOps scaffolding

### Synopsis

opencenter is a command-line tool for managing Kubernetes cluster configurations
and GitOps repositories. It provides a declarative approach to cluster lifecycle
management with built-in validation, secrets management, and multi-provider support.

Key Features:
  • Declarative YAML-based cluster configuration
  • Automatic GitOps repository scaffolding
  • SOPS integration for secrets management
  • Multi-cloud provider support (OpenStack, VMware, Kind, Baremetal)
  • Comprehensive validation and doctor checks
  • Organization-based multi-tenancy support

Documentation: https://github.com/opencenter-cloud/opencenter-cli/blob/main/docs
Support: https://github.com/opencenter-cloud/opencenter-cli/issues

```
opencenter [flags]
```

### Examples

```
  # Initialize a new cluster configuration
  opencenter cluster init my-cluster

  # Validate cluster configuration
  opencenter cluster validate my-cluster

  # List all clusters
  opencenter cluster list

  # Generate GitOps assets
  opencenter cluster generate my-cluster

  # Deploy a cluster
  opencenter cluster deploy my-cluster

  # Show the active cluster
  opencenter cluster active
```

### Options

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --dry-run             preview mutating operations without writing or acting
  -h, --help                help for opencenter
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter cluster](opencenter_cluster.md)	 - Manage cluster configurations
* [opencenter completion](opencenter_completion.md)	 - Generate the autocompletion script for the specified shell
* [opencenter plugins](opencenter_plugins.md)	 - Manage opencenter plugins
* [opencenter secrets](opencenter_secrets.md)	 - Manage secrets across backends
* [opencenter settings](opencenter_settings.md)	 - Manage CLI settings
* [opencenter shell-init](opencenter_shell-init.md)	 - Output shell integration script for session-scoped active clusters
* [opencenter version](opencenter_version.md)	 - Display version and build information
