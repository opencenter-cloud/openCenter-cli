---
id: create-openstack-cluster
title: Create an OpenStack cluster
sidebar_label: Create an OpenStack cluster
description: Configure and prepare an OpenStack cluster through the openCenter command path.
doc_type: how-to
audience: openCenter operators
tags: [openstack, clusters]
last_updated: 2026-09-25
---
# Create an OpenStack cluster

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

The plan is read-only. Selectors include image, internal/external network,
subnet, and availability zone; each accepts a resource name or ID (`--image-name`,
`--windows-image-name`, `--network-id`, `--subnet-id`, `--external-network-id`,
`--availability-zone`) and persists the resolved ID. `--create-internal-network`
chooses the OpenTofu-managed internal-network mode and must be passed to both operations.
`--replace` allows replacement of populated values; `--import-auth` and
`--import-tls` import selected profile settings. Apply prompts in text mode;
global `--yes` is available for non-interactive use.

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
