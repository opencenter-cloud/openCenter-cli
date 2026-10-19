---
last_updated: 2026-09-24
id: "opencenter-cluster-provider-openstack-plan"
title: "Opencenter_Cluster_Provider_Openstack_Plan"
sidebar_label: "Opencenter_Cluster_Provider_Openstack_Plan"
description: "plan OpenStack provider metadata and selections"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
---
## opencenter cluster provider openstack plan

plan OpenStack provider metadata and selections

### Synopsis

Preview the changes that would bind this cluster to the given OpenStack cloud.

Reads the profile named by --os-cloud from clouds.yaml (override its location with
--clouds-yaml or OS_CLIENT_CONFIG_FILE), discovers live images, networks, subnets, and
availability zones read-only, and prints the changes without writing anything: image IDs,
internal/external network and subnet IDs, availability zone, plus application credential and TLS
settings when requested. No remote resources are created or modified.

Selector flags (--image-name, --windows-image-name, --network-id, --subnet-id,
--external-network-id, --availability-zone) accept a resource name or ID; a name
that matches several discovered resources is rejected so nothing is picked silently.

Use 'apply' with identical flags to persist the same change set.

```
opencenter cluster provider openstack plan <cluster> [flags]
```

### Options

```
      --availability-zone string     availability zone name or ID
      --clouds-yaml string           path to clouds.yaml
      --create-internal-network      let OpenTofu create and manage the internal network and subnet
      --external-network-id string   external network name or ID
  -h, --help                         help for plan
      --image-name string            Linux image name or ID
      --import-auth                  write application credential fields from the selected profile
      --import-tls                   persist selected profile TLS settings
      --network-id string            internal network name or ID
      --os-cloud string              profile name inside clouds.yaml to use for discovery
      --replace                      allow replacing populated provider selections
      --subnet-id string             internal subnet name or ID
      --windows-image-name string    Windows image name or ID
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

* [opencenter cluster provider openstack](opencenter_cluster_provider_openstack.md)	 - Manage OpenStack provider configuration
