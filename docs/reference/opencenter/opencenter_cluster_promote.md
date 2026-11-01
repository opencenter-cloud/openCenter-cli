---
id: "opencenter-cluster-promote"
title: "Opencenter_Cluster_Promote"
sidebar_label: "Opencenter_Cluster_Promote"
description: "Write observed deployed-cluster values into the cluster config"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-10-31"
---
## opencenter cluster promote

Write observed deployed-cluster values into the cluster config

### Synopsis

Write values observed from the deployed cluster into the cluster configuration.

promote is the write half of 'opencenter cluster observe'. It re-observes the
cluster, then patches only the reported paths through the same write plan the
import flow uses, which means:

  - the config file is backed up before it is written
  - protected fields (credentials, tokens, keys) are never written
  - conflicting fields are skipped and reported, never overwritten
  - only high-confidence observations are written

Pass --dry-run to see the patch without writing it. Use --field to promote a
subset of the reported paths.

After promoting infrastructure identity, regenerate the GitOps tree so the
rendered manifests pick the values up:

  opencenter cluster promote acme/prod
  opencenter cluster generate acme/prod

The CCM [LoadBalancer] subnet-id in particular is rendered from
networking.subnet_id at generate time, so a promoted subnet does not reach the
cluster until the overlay is re-rendered and pushed.

```
opencenter cluster promote [cluster] [flags]
```

### Examples

```
  # Preview the patch
  opencenter cluster promote acme/prod --dry-run

  # Write every promotable value
  opencenter cluster promote acme/prod

  # Write only the network identity
  opencenter cluster promote acme/prod \
    --field opencenter.infrastructure.cloud.openstack.networking.subnet_id
```

### Options

```
      --dry-run         show the patch without writing it
      --field strings   promote only these config paths
  -h, --help            help for promote
      --source string   evidence source: terraform, live, or all (default "all")
```

### Options inherited from parent commands

```
      --config-dir string   configuration directory (defaults to ~/.config/opencenter on Linux/macOS)
      --log-level string    set log level explicitly (debug, info, warn, error) (default "warn")
      --output string       output format for supported commands: text, json, yaml (default "text")
      --quiet               suppress nonessential human output
      --yes                 answer yes to confirmation prompts
```

### SEE ALSO

* [opencenter cluster](opencenter_cluster.md)	 - Manage cluster configurations
