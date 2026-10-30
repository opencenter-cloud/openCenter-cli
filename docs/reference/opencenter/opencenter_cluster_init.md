---
last_updated: 2026-09-24
id: opencenter-cluster-init
title: "Opencenter_Cluster_Init"
sidebar_label: Opencenter_Cluster_Init
description: Documentation for Opencenter_Cluster_Init.
doc_type: reference
audience: "platform engineers"
tags: [reference]
---
## opencenter cluster init

Initialize a new cluster configuration (non-interactive)

### Synopsis

Initialize a new cluster configuration with default values.

This command creates a new cluster configuration file with sensible defaults
based on the JSON schema. You can override any configuration value using
command-line flags with dot notation.

This command generates schema version "2.0" configuration only.

The configuration is created in an organization-based directory structure:
  ~/.config/opencenter/clusters/<organization>/<cluster>/

By default, the organization is set to "opencenter". Use --org to specify a different organization.

SOPS Age encryption keys and SSH key pairs are automatically generated if they
don't already exist, unless --no-keygen is specified.

With --os-cloud, initialization is followed immediately by the OpenStack
provider binding: the same discovery-and-apply pass as
'cluster provider openstack apply', run against the freshly created cluster
configuration. --os-cloud implies --type openstack. If the selected profile
has no application credential ID and secret, one is created through Keystone,
saved back into clouds.yaml, and written into the cluster configuration, so
the cluster is ready without a manual credential step. Internal networking
defaults to OpenTofu-managed (--create-internal-network is the default; pass
--create-internal-network=false to select an existing network). Provider
selector flags (--clouds-yaml, --image-name, --windows-image-name,
--network-id, --external-network-id, --subnet-id, --availability-zone,
--create-internal-network, --replace, --import-auth, --import-tls,
--create-application-credential) are only valid together with --os-cloud and
are forwarded to the provider apply pass.

--base-domain sets opencenter.cluster.base_domain (and the derived
cluster_fqdn) on the new cluster and tags it managed.base_domain: default, so
the cluster keeps following cluster_defaults.base_domain from the CLI settings
file: later 'opencenter settings set cluster_defaults.base_domain ...' changes
propagate through generate/validate/deploy at every config load. To freeze the
domain against settings changes later, use 'cluster set
<cluster> opencenter.cluster.base_domain=...'.

```
opencenter cluster init [name] [flags]
```

### Examples

```
  # Initialize with defaults
  opencenter cluster init my-cluster

  # Initialize from existing config file
  opencenter cluster init --config-file my-cluster-config.yaml

  # Initialize with organization
  opencenter cluster init my-cluster --org myorg

  # Initialize a VMware cluster
  opencenter cluster init my-cluster --org production --type vmware

  # Initialize with a specific region
  opencenter cluster init my-cluster --type openstack --region sjc3

  # Initialize a Kind cluster with the built-in CNI disabled
  opencenter cluster init my-cluster --org local --type kind --kind-disable-default-cni

  # Initialize without key generation
  opencenter cluster init my-cluster --no-keygen

  # Overwrite existing config
  opencenter cluster init my-cluster --force

  # Initialize and bind to an OpenStack cloud in one step (Tofu-managed
  # internal network and an auto-minted application credential included)
  opencenter cluster init my-cluster --org production --os-cloud prod-profile --yes

  # Initialize with an explicit base domain (still follows settings changes)
  opencenter cluster init my-cluster --base-domain k8s.example.com
```

### Options

```
      --base-domain string         base DNS domain for the cluster (sets opencenter.cluster.base_domain; the cluster keeps following cluster_defaults.base_domain from settings)
      --config-file string         load configuration from file
      --force                      overwrite existing config file
      --full-schema                generate configuration with all available fields
  -h, --help                       help for init
      --kind-disable-default-cni   disable Kind's default CNI so cluster networking is managed by openCenter
      --no-keygen                  do not auto-generate keys
      --no-sops-keygen             do not auto-generate SOPS keys
      --org string                 organization name (defaults to 'opencenter')
      --os-cloud string            after init, bind the cluster to this clouds.yaml profile (implies --type openstack)
      --regenerate-keys            regenerate keys even if they exist
      --region string              cluster region (e.g. dfw3, sjc3, iad3, ord1)
      --server-pool stringArray    additional server pool configuration
      --strict                     fail if required values are missing
      --type string                cluster type: openstack, baremetal, kind, vmware, magnum (default "openstack")
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
