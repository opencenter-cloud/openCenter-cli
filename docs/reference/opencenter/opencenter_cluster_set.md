---
last_updated: 2026-09-24
id: opencenter-cluster-set
title: "Opencenter_Cluster_Set"
sidebar_label: Opencenter_Cluster_Set
description: Documentation for Opencenter_Cluster_Set.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster set

Set fields in an existing cluster configuration

### Synopsis

Set one or more fields in an existing cluster configuration.

Fields use native v2 dot notation, for example:
  opencenter.meta.env=prod
  opencenter.gitops.repository.url=https://github.com/acme/platform.git

base_domain provenance: an explicit opencenter.cluster.base_domain assignment
pins the field (the cluster config owns it; cluster_defaults.base_domain from
settings never rewrites it). Pass --follow-settings to instead tag the field
managed.base_domain=default so settings changes propagate at every config load
(generate/validate/deploy). A cluster that has reached the bootstrap stage is
live: its DNS identity pins automatically and --follow-settings is rejected
there, because re-pointing a deployed cluster is a destructive rename.

```
opencenter cluster set [cluster] <path=value>... [flags]
```

### Examples

```
  # Freeze the cluster's own domain (pins against settings changes)
  opencenter cluster set my-cluster opencenter.cluster.base_domain=k8s.example.com

  # Seed a domain and keep following cluster_defaults.base_domain from settings
  opencenter cluster set my-cluster opencenter.cluster.base_domain=k8s.example.com --follow-settings
```

### Options

```
      --follow-settings   with an opencenter.cluster.base_domain assignment, tag the field managed.base_domain=default so cluster_defaults.base_domain from settings keeps propagating at every config load (default: pin the field to the cluster)
  -h, --help              help for set
      --strict            fail if the resulting configuration is not valid
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
