---
id: create-openstack-cluster
title: Create an OpenStack cluster
sidebar_label: Create an OpenStack cluster
description: Configure and prepare an OpenStack cluster through the openCenter command path.
doc_type: how-to
audience: openCenter operators
tags: [openstack, clusters]
last_updated: 2026-10-27
---
# Create an OpenStack cluster

**Purpose:** For openCenter operators, shows how to configure and prepare an OpenStack cluster through the openCenter command path.

Use this page when the target provider is OpenStack. Values such as image IDs,
network IDs, credentials, and Git URLs are environment inputs; the examples use
placeholders and are not credentials.

## Configure

Use the guided configuration workflow when interactive provider selection is
appropriate. The file-based path is:

For a non-interactive starting file:

```bash
opencenter cluster init demo --org my-org --type openstack
opencenter cluster edit my-org/demo
```

The readiness validator requires an OpenStack cloud block with `auth_url`,
`region`, `project_id`, and `image_id`. Application credential ID and secret
must be supplied together when that credential mode is used.

## Plan and apply provider metadata

```bash
opencenter cluster provider openstack plan my-org/demo --os-cloud PROFILE
opencenter cluster provider openstack apply my-org/demo --os-cloud PROFILE
```

`cluster init --os-cloud PROFILE` combines both steps: it creates the
blueprint and then runs the apply pass against it (selector flags such as
`--create-internal-network` and `--import-auth` are accepted and forwarded).

The plan is read-only. Internal networking is OpenTofu-managed by default:
`--create-internal-network` is on unless you pass `--create-internal-network=false`,
which opts into selecting an existing network. Selectors include image,
internal/external network, subnet, and availability zone; each accepts a
resource name or ID (`--image-name`, `--windows-image-name`, `--network-id`,
`--subnet-id`, `--external-network-id`, `--availability-zone`) and persists the
resolved ID. The mode flag must be passed identically to plan and apply.
The profile's region (`region_name`, or `OS_REGION_NAME` when the profile omits
it) is authoritative: plan and apply repoint `infrastructure.cloud.openstack.region`,
`meta.region`, and the Barbican region to it, correcting the region baked by
`cluster init`. A deliberately-set `meta.region` blocks the plan until
`--replace`. `--replace` allows replacement of populated values; `--import-auth`
and `--import-tls` import selected profile settings. Apply prompts in text mode;
global `--yes` is available for non-interactive use.

Two values propagate beyond the provider block. The resolved Linux image is
written to every consumer of it — `infrastructure.bastion.image` (when the
bastion is enabled) and each enabled Linux worker pool — unless the follower
holds a deliberately-set image, which `--replace` repoints. An application
credential is minted and imported automatically when the profile has no
ID/secret pair: the pair is created through Keystone, persisted back into
`clouds.yaml` (atomically, mode 0600), and stored in the cluster config.
`--create-application-credential` forces a fresh credential (rotation) even
when the profile already has one.

## Validate, render, and deploy

```bash
opencenter cluster validate my-org/demo
opencenter cluster generate my-org/demo
opencenter cluster deploy my-org/demo
```

Use `--validation online` for remote checks, `--render-only` for template-only
rendering, and `--dry-run` for previews. Deployment state and logs are reported
by the command and can be controlled with `--step`, `--from-step`, `--restart`,
and `--log`.

## Evidence

- OpenStack provider commands: `cmd/cluster_provider_openstack.go`
- Lifecycle commands: `cmd/cluster_validate.go`, `cmd/cluster_generate.go`,
  `cmd/cluster_deploy.go`
- OpenStack readiness: `internal/config/v2/readiness.go`
- OpenStack bootstrap steps: `internal/cluster/bootstrap_provider_infra.go`
