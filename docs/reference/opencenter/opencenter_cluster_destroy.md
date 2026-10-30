---
last_updated: 2026-10-02
id: opencenter-cluster-destroy
title: "Opencenter_Cluster_Destroy"
sidebar_label: Opencenter_Cluster_Destroy
description: Documentation for Opencenter_Cluster_Destroy.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster destroy

Destroy a cluster

### Synopsis

Destroy a cluster's infrastructure and optionally remove its configuration files.

This command first destroys the cloud infrastructure (via OpenTofu for supported providers),
then optionally removes this cluster's local configuration files and its
trees inside the GitOps worktree.

By default, local files are preserved after infrastructure destruction to allow for
inspection, debugging, or recovery. Use --remove-files to also delete local files.
The GitOps worktree (gitops.repository.local_dir) is shared by every cluster in
the organization: --remove-files deletes only the designated cluster's
directories and never the shared worktree or another cluster's files.

The cluster name can be specified as 'cluster' or 'organization/cluster'.
If no cluster name is provided, the active cluster will be destroyed.

If an existing lock is found, you will be prompted to break it. Use --break-lock
to automatically break any existing lock without prompting.

When the cluster API endpoint is unreachable (decommissioned cluster, VPN down),
use --skip-cluster-access to omit the kubectl CSI volume-capture step entirely,
so destroy never blocks on the dead endpoint. Orphaned Cinder volumes then need
manual cleanup.

```
opencenter cluster destroy [name] [flags]
```

### Examples

```
  # Destroy infrastructure only (keep local files for inspection)
  opencenter cluster destroy my-cluster --force

  # Destroy infrastructure AND remove all local files
  opencenter cluster destroy my-cluster --force --remove-files

  # Skip infrastructure destruction (just remove local files)
  opencenter cluster destroy my-cluster --force --skip-infrastructure --remove-files

  # Destroy with an unreachable cluster API endpoint (skip kubectl capture)
  opencenter cluster destroy my-cluster --force --skip-cluster-access --remove-files

  # Destroy cluster in specific organization
  opencenter cluster destroy myorg/my-cluster --force

  # Destroy and break any existing lock without prompting
  opencenter cluster destroy my-cluster --force --break-lock

  # Destroy active cluster
  opencenter cluster destroy --force
```

### Options

```
      --allow-cleanup-failures    Continue when pre-destroy cleanup steps (load balancer sweep) fail; leaked resources are reported as warnings
      --delete-volumes            Delete CSI-provisioned Cinder volumes left orphaned after infrastructure destruction (OpenStack only)
      --destroy-parallelism int   OpenTofu destroy -parallelism value (0 keeps the OpenTofu default)
      --destroy-retries int       Additional attempts for the OpenTofu destroy step after a transient failure (0 disables retries) (default 1)
      --force                     Skip confirmation prompt
  -h, --help                      help for destroy
      --log string                log file path (defaults to <state_dir>/logs/destroy/<org>/<name>/destroy-YYYYMMDDTHHMMSSZ.log)
      --remove-files              Remove this cluster's local configuration and GitOps files after infrastructure destruction (the shared organization GitOps worktree and other clusters' files are preserved)
      --require-cluster-access    Abort before destroying infrastructure if the cluster kubeconfig is missing or unreachable (CSI volume capture cannot be skipped)
      --skip-cluster-access       Never contact the cluster API during destroy (skips the kubectl CSI volume capture); use when the kube endpoint is unreachable (incompatible with --require-cluster-access)
      --skip-infrastructure       Skip infrastructure destruction (only remove local files when combined with --remove-files)
      --strict                    Exit with an error when post-destroy cleanup reported warnings (leaked load balancers, orphaned volumes)
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
