---
last_updated: 2026-10-26
id: opencenter-cluster-generate
title: "Opencenter_Cluster_Generate"
sidebar_label: Opencenter_Cluster_Generate
description: Documentation for Opencenter_Cluster_Generate.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster generate

Generate the GitOps repository and rendered manifests

### Synopsis

Generate the customer GitOps repository and rendered manifests for a cluster.

This command creates or updates the repository structure, infrastructure templates,
Flux manifests, and application overlays based on the cluster configuration.

Use --render-only to render templates without running the full repository setup flow.
Use --gitops-auth to select the GitOps authentication method for the base repository
sources (ssh or token). Defaults to cluster_defaults.gitops_auth_method from settings.

base_domain provenance: a cluster whose opencenter.cluster.managed.base_domain is
"default" follows cluster_defaults.base_domain from settings automatically at every
load. A pinned (or hand-edited) domain is never rewritten; generate warns when the
settings domain changed since the last successful generation. Use --sync-base-domain
to adopt the settings domain into a pinned cluster config (rewrites the derived
cluster_fqdn).

```
opencenter cluster generate [name] [flags]
```

### Examples

```
  # Generate assets for the active cluster
  opencenter cluster generate

  # Generate assets for a specific cluster
  opencenter cluster generate my-cluster

  # Preview what would be generated
  opencenter cluster generate my-cluster --dry-run

  # Render templates only
  opencenter cluster generate my-cluster --render-only

  # Generate with explicit GitOps auth method
  opencenter cluster generate my-cluster --gitops-auth=token
  opencenter cluster generate my-cluster --gitops-auth=ssh

  # Adopt a changed cluster_defaults.base_domain into the cluster config
  opencenter cluster generate my-cluster --sync-base-domain
```

### Options

```
      --adopt-generated      claim differing planned files after creating backups
      --force                overwrite existing GitOps repository
      --gitops-auth string   GitOps authentication method for base repo sources (ssh, token); defaults to cluster_defaults.gitops_auth_method
  -h, --help                 help for generate
      --prune                remove stale generated files (use --prune=false to report but retain them) (default true)
      --render-only          render templates without running repository setup
      --skip-validation      skip configuration validation before generation
      --sync-base-domain     adopt cluster_defaults.base_domain from settings into the cluster config (rewrites the derived cluster_fqdn) before generating
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
