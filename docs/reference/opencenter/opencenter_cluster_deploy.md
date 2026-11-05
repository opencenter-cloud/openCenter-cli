---
id: "opencenter-cluster-deploy"
title: "Opencenter_Cluster_Deploy"
sidebar_label: "Opencenter_Cluster_Deploy"
description: "Deploy a cluster from its openCenter configuration"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster deploy

**Purpose:** For operators, developers, documents `opencenter cluster deploy`: what it does, its flags, and its examples.

Deploy a cluster from its openCenter configuration

### Synopsis

Deploy a cluster from its openCenter configuration.

This command provisions infrastructure and deploys Kubernetes based on the
cluster configuration. The process is resumable; if a step fails, fix the issue
and re-run deploy to continue from the saved state.

```
opencenter cluster deploy [name] [flags]
```

### Options

```
      --break-lock                 force removal of an existing operation lock before deploying
      --container-runtime string   container runtime for kind clusters (docker or podman)
      --debug                      print deploy step debug details before each step runs
      --from-step string           restart deploy from the specified step ID
  -h, --help                       help for deploy
      --kubeconfig string          path to kubeconfig used by deploy actions (defaults to the cluster-owned kubeconfig path)
      --log string                 log file path (defaults to <state_dir>/logs/bootstrap/<org>/<name>/bootstrap-YYYYMMDDTHHMMSSZ.log)
      --restart                    rerun all deploy steps and ignore saved state
      --step string                run a single deploy step by ID
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
