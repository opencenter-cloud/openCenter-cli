---
id: "opencenter-cluster-service-storage-plan"
title: "Opencenter_Cluster_Service_Storage_Plan"
sidebar_label: "Opencenter_Cluster_Service_Storage_Plan"
description: "plan one service's OpenStack storage"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster service storage plan

**Purpose:** For operators, developers, documents `opencenter cluster service storage plan`: what it does, its flags, and its examples.

plan one service's OpenStack storage

```
opencenter cluster service storage plan <service> [flags]
```

### Options

```
      --backend string       OpenStack storage backend: swift or s3
      --clouds-yaml string   path to clouds.yaml
      --cluster string       cluster identifier
      --container string     container or bucket name
  -h, --help                 help for plan
      --os-cloud string      clouds.yaml profile to use
      --rotate-credentials   replace an existing credential pair
      --s3-endpoint string   distinct S3-compatible endpoint URL
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

* [opencenter cluster service storage](opencenter_cluster_service_storage.md)	 - Manage one service's OpenStack storage
