---
last_updated: 2026-10-27
id: "opencenter-cluster-provider-openstack"
title: "Opencenter_Cluster_Provider_Openstack"
sidebar_label: "Opencenter_Cluster_Provider_Openstack"
description: "Manage OpenStack provider configuration"
doc_type: reference
audience: "operators, developers"
tags: [cli, reference]
---
## opencenter cluster provider openstack

Manage OpenStack provider configuration

### Synopsis

Bind an existing cluster (infrastructure.provider: openstack) to a specific
OpenStack cloud. Reads a clouds.yaml profile (--os-cloud), discovers images,
networks, subnets, and availability zones read-only, then plans or applies the
cluster's OpenStack config values from them: image IDs, internal/external
network and subnet IDs, availability zone, and optionally application credentials and TLS settings.

If the profile has no application credential ID and secret, one is created
through Keystone, saved back into clouds.yaml (0600, atomically), and written
into the cluster configuration, so the cluster is bound with a usable identity
in one step. --create-application-credential forces a fresh credential even
when the profile already has one.

This command does not deploy the cluster or create remote resources beyond the
application credential above; it only writes the cluster's own configuration
file. Use 'cluster deploy' for that. With --create-internal-network it switches
internal networking to OpenTofu-managed mode instead of selecting an existing
network.

### Options

```
  -h, --help   help for openstack
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

* [opencenter cluster provider](opencenter_cluster_provider.md)	 - Manage provider configuration
* [opencenter cluster provider openstack apply](opencenter_cluster_provider_openstack_apply.md)	 - apply OpenStack provider metadata and selections
* [opencenter cluster provider openstack plan](opencenter_cluster_provider_openstack_plan.md)	 - plan OpenStack provider metadata and selections
