---
id: "opencenter-cluster-service"
title: "Opencenter_Cluster_Service"
sidebar_label: "Opencenter_Cluster_Service"
description: "Manage cluster services"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster service

**Purpose:** For operators, developers, documents `opencenter cluster service`: what it does, its flags, and its examples.

Manage cluster services

### Synopsis

The service command allows adding and removing services from a cluster's configuration.

Services can be either standard services or managed services. When adding a service,
it may require additional parameters or secrets. If these are not provided, the
command will fail and provide an example of the correct usage.

```
opencenter cluster service [flags]
```

### Options

```
  -h, --help   help for service
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
* [opencenter cluster service disable](opencenter_cluster_service_disable.md)	 - Disable a service in the cluster configuration
* [opencenter cluster service enable](opencenter_cluster_service_enable.md)	 - Enable a service in the cluster configuration
* [opencenter cluster service options](opencenter_cluster_service_options.md)	 - Display available configuration options for a service
* [opencenter cluster service reconcile](opencenter_cluster_service_reconcile.md)	 - Reconcile a specific service in GitOps and Flux
* [opencenter cluster service status](opencenter_cluster_service_status.md)	 - Display state of all services in the cluster configuration
* [opencenter cluster service storage](opencenter_cluster_service_storage.md)	 - Manage one service's OpenStack storage
