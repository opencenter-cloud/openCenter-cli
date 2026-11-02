---
last_updated: 2026-10-14
id: "opencenter-cluster-provider"
title: "Opencenter_Cluster_Provider"
sidebar_label: "Opencenter_Cluster_Provider"
description: "Manage provider configuration"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
---
## opencenter cluster provider

Manage provider configuration

### Synopsis

Configure how a cluster connects to its cloud provider.

Each subcommand works on an existing cluster whose infrastructure.provider
matches the subcommand's provider, and changes only that cluster's
configuration file -- it does not deploy or modify resources in the cloud.

```
opencenter cluster provider [flags]
```

### Options

```
  -h, --help   help for provider
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
* [opencenter cluster provider openstack](opencenter_cluster_provider_openstack.md)	 - Manage OpenStack provider configuration
