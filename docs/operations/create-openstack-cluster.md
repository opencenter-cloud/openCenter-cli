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

`cluster init --os-cloud PROFILE` combines both steps: it creates the
blueprint and then runs the apply pass against it (selector flags such as
`--create-internal-network` and `--import-auth` are accepted and forwarded).

The plan is read-only. Selectors include image, internal/external network,
subnet, and availability zone; each accepts a resource name or ID (`--image-name`,
`--windows-image-name`, `--network-id`, `--subnet-id`, `--external-network-id`,
`--availability-zone`) and persists the resolved ID. When no `--image-name` is
given, the default `Ubuntu 24.04` is resolved case-insensitively; if several
active images match, the plan still resolves it when exactly one candidate's
name matches byte-for-byte, or when exactly one of the same-name candidates is
owned by your project (community imports lose). Any remaining ambiguity blocks
with the candidate list. An explicit `--image-name` is never tie-broken: an
ambiguous value always errors. `--create-internal-network`
chooses the OpenTofu-managed internal-network mode and must be passed to both operations.
The profile's region (`region_name`, or `OS_REGION_NAME` when the profile omits
it) is authoritative: plan and apply repoint `infrastructure.cloud.openstack.region`,
`meta.region`, and the Barbican region to it, correcting the region baked by
`cluster init`. A deliberately-set `meta.region` blocks the plan until
`--replace`. `--replace` allows replacement of populated values; `--import-auth`
and `--import-tls` import selected profile settings. Apply prompts in text mode;
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
