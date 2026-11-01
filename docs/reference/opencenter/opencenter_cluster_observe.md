---
id: "opencenter-cluster-observe"
title: "Opencenter_Cluster_Observe"
sidebar_label: "Opencenter_Cluster_Observe"
description: "Report differences between the deployed cluster and its config"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-10-31"
---
## opencenter cluster observe

Report differences between the deployed cluster and its config

### Synopsis

Report what the deployed cluster knows that the cluster configuration does not.

observe is read-only. It never writes the configuration file; use
'opencenter cluster drift reconcile --to-config' to write the reported values.

Two evidence sources are merged into one report:

  terraform  the applied OpenTofu state, i.e. resources the CLI created. Needs
             the infrastructure tree rendered by lifecycle contract 3, which is
             what exposes opencenter_network_id / opencenter_subnet_id.
  live       the running Kubernetes cluster, read through kubectl and Flux.

The terraform source is what recovers a network OpenTofu created. 'cluster
provider openstack' clears network_id/subnet_id so OpenTofu can own them, and
until contract 3 nothing wrote the created IDs back, so the CCM rendered its
[LoadBalancer] subnet-id blank on a tofu-managed cluster.

A field the config already holds with a different value is reported as a
conflict, never as a promotable value: a populated subnet_id may be an operator
selection made with --create-internal-network=false.

Exit code 1 when findings exist, so observe can gate a pipeline.

```
opencenter cluster observe [cluster] [flags]
```

### Examples

```
  # Report everything observable about the active cluster
  opencenter cluster observe

  # Only what OpenTofu created
  opencenter cluster observe acme/prod --source terraform

  # Machine-readable
  opencenter cluster observe acme/prod --output json
```

### Options

```
  -h, --help            help for observe
      --source string   evidence source: terraform, live, or all (default "all")
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
