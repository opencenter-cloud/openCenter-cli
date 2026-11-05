---
id: "opencenter-cluster-reconcile"
title: "Opencenter_Cluster_Reconcile"
sidebar_label: "Opencenter_Cluster_Reconcile"
description: "Push GitOps manifests and reconcile Flux resources"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
last_updated: "2026-11-05"
---
## opencenter cluster reconcile

**Purpose:** For operators, developers, documents `opencenter cluster reconcile`: what it does, its flags, and its examples.

Push GitOps manifests and reconcile Flux resources

### Synopsis

Stage, validate, commit, and push cluster GitOps changes to remote repository,
and reconcile affected Flux resources in-cluster.

This command provides a generic Day-2 convergence workflow for GitOps updates:
  1. Optionally re-renders/re-encrypts target service overlay (--generate)
  2. Scopes and stages cluster changes, validating that no unencrypted secrets are committed
  3. Commits and pushes to the GitOps remote repository
  4. Triggers in-cluster Flux reconciliation for GitRepository source, Kustomizations, and HelmReleases

Examples:
  # Reconcile all uncommitted cluster changes for active cluster
  opencenter cluster reconcile

  # Reconcile specific cluster and service
  opencenter cluster reconcile opensnack/doritos --service openstack-ccm

  # Re-render service values and reconcile
  opencenter cluster reconcile opensnack/doritos -s openstack-ccm --generate

  # Preview execution plan without mutating git or cluster
  opencenter cluster reconcile opensnack/doritos --dry-run

  # Only commit and push without in-cluster Flux reconciliation
  opencenter cluster reconcile opensnack/doritos --push-only

  # Only trigger in-cluster Flux reconciliation without Git commit/push
  opencenter cluster reconcile opensnack/doritos --flux-only

```
opencenter cluster reconcile [name] [flags]
```

### Options

```
  -a, --all                 reconcile all enabled services for the cluster
      --flux-only           execute Flux in-cluster reconciliation only; skip Git commit & push
  -g, --generate            re-render and re-encrypt target service overlay before reconciling
  -h, --help                help for reconcile
      --kubeconfig string   path to kubeconfig (defaults to cluster-owned kubeconfig)
  -m, --message string      custom git commit message
      --push-only           execute GitOps commit & push only; skip Flux in-cluster reconciliation
      --remote string       git remote to push to (defaults to tracking remote, upstream, or origin)
  -s, --service strings     target service(s) to reconcile (repeatable or comma-separated)
      --skip-flux           skip Flux in-cluster reconciliation
      --timeout duration    timeout for Flux reconciliation and rollout (default 5m0s)
  -w, --wait                wait for Flux resources to become Ready (default true)
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
