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
then optionally removes local configuration files and GitOps directories.

By default, local files are preserved after infrastructure destruction to allow for
inspection, debugging, or recovery. Use --remove-files to also delete local files.

The cluster name can be specified as 'cluster' or 'organization/cluster'.
If no cluster name is provided, the active cluster will be destroyed.

If an existing lock is found, you will be prompted to break it. Use --break-lock
to automatically break any existing lock without prompting.

**OpenStack CSI volume cleanup** — When an OpenStack cluster is destroyed, Cinder
volumes that were dynamically provisioned by the CSI driver (`cinder.csi.openstack.org`)
are not removed by OpenTofu because they were created by Kubernetes, not Terraform.
By default, `cluster destroy` reports any orphaned volumes so you can clean them up
manually. Use `--delete-volumes` to delete them automatically.

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

  # Destroy cluster in specific organization
  opencenter cluster destroy myorg/my-cluster --force

  # Destroy and break any existing lock without prompting
  opencenter cluster destroy my-cluster --force --break-lock

  # Destroy active cluster
  opencenter cluster destroy --force

  # Destroy and automatically delete orphaned CSI Cinder volumes (OpenStack only)
  opencenter cluster destroy my-cluster --force --delete-volumes

  # Destroy without deleting volumes — orphaned volumes are reported instead
  opencenter cluster destroy my-cluster --force
```

### Options

```
      --delete-volumes        Delete CSI-provisioned Cinder volumes left orphaned after infrastructure destruction (OpenStack only)
      --force                 Skip confirmation prompt
  -h, --help                  help for destroy
      --remove-files          Remove local configuration and GitOps files after infrastructure destruction
      --skip-infrastructure   Skip infrastructure destruction (only remove local files when combined with --remove-files)
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
* [Destroy an OpenStack cluster](../../operations/destroy-openstack-cluster.md) - Step-by-step teardown guide including CSI volume cleanup
