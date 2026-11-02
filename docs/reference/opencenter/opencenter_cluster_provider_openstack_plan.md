---
last_updated: 2026-10-27
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
settings when requested. The only remote resource it may create is an
application credential for a profile that lacks one (see below).

Selector flags (--image-name, --windows-image-name, --network-id, --subnet-id,
--external-network-id, --availability-zone) accept a resource name or ID; a name
that matches several discovered resources is rejected so nothing is picked
silently. With no --image-name, the name Ubuntu 24.04 is resolved against the
project inventory and reported as a warning; if the project does not have it, the
candidate list is printed instead. --windows-image-name is only required when the
cluster runs Windows workers.

The resolved Linux image propagates to every consumer of it: the provider
block (image_id), infrastructure.bastion.image when the bastion is enabled,
and each enabled Linux worker pool. Followers still holding an init-baked
(empty or placeholder) image are repointed silently; an image set deliberately
is preserved unless --replace.

The region discovered from the selected profile (clouds.yaml region_name, or
OS_REGION_NAME when the profile omits it) is authoritative: it repoints the
provider block, opencenter.meta.region, and the Barbican secret backend, so
plan and apply bind the whole cluster to the chosen cloud. A meta.region that
was set deliberately (not the init default) blocks the plan until --replace.

An application credential is minted and imported automatically when the
profile has no ID/secret pair: the pair is created through Keystone, written
back into clouds.yaml (atomically, 0600), and stored in the cluster config.
--create-application-credential forces a fresh credential (rotation) even when
the profile already has one.

Use 'apply' with identical flags to persist the same change set.

```
opencenter cluster provider openstack plan <cluster> [flags]
```

### Options

```
      --availability-zone string        availability zone name or ID
      --clouds-yaml string              path to clouds.yaml
      --create-application-credential   mint an application credential for the profile when it has none, persist it to clouds.yaml, and import it
      --create-internal-network         let OpenTofu create and manage the internal network and subnet (default; pass --create-internal-network=false to select an existing network instead) (default true)
      --external-network-id string      external network name or ID
  -h, --help                            help for plan
      --image-name string               Linux image name or ID
      --import-auth                     write application credential fields from the selected profile
      --import-tls                      persist selected profile TLS settings
      --network-id string               internal network name or ID
      --os-cloud string                 profile name inside clouds.yaml to use for discovery
      --replace                         allow replacing populated provider selections
      --subnet-id string                internal subnet name or ID
      --windows-image-name string       Windows image name or ID
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
