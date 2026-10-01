---
last_updated: 2026-09-28
id: service-openstack-ccm
title: "OpenStack Cloud Controller Manager"
sidebar_label: OpenStack CCM
description: OpenStack CCM service configuration and catalog ownership.
doc_type: reference
audience: "platform engineers, operators"
tags: [openstack, services]
---

> **Evidence:** `internal/config/services/default_services.go`, `internal/config/v2/defaults.go`, and `internal/gitops/render_catalog.go`.

## Configuration

`openstack-ccm` uses `DefaultServiceConfig`. The generated default is enabled in `openstack-ccm`.

```yaml
opencenter:
  services:
    openstack-ccm:
      enabled: true
      namespace: openstack-ccm
      adoption_mode: managed
      address_pool:
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | `true` | `NewDefaultServiceConfig` |
| `namespace` | `openstack-ccm` | `NewDefaultServiceConfig` |
| common fields | — | `BaseConfig` |

## Rendering

The catalog entry uses namespace stage `openstack-ccm`, override values, base path `applications/base/services/openstack-ccm`, and override dependencies `sources` and `openstack-ccm-namespace`. No explicit service descriptor is present.

## Credentials (OCTR-750)

The external OpenStack cloud-controller-manager resolves its `external_openstack_*`
settings from environment lookups such as `OS_AUTH_URL`, `OS_APPLICATION_CREDENTIAL_ID`,
and `OS_APPLICATION_CREDENTIAL_SECRET`. During a cluster deploy, the CLI forwards every
`OS_*` variable from the OpenTofu environment into the Kubespray ansible run so the CCM
role can configure itself without the operator re-exporting each variable in the deploy
shell.

If the CCM cannot read these credentials it fails with
`external_openstack_auth_url is missing`, nodes keep the
`node.cloudprovider.kubernetes.io/uninitialized` taint, and `.spec.providerID` stays
empty. When the credentials are forwarded correctly, every node reports
`providerID: openstack:///<instance-uuid>` and the uninitialized taint clears.

> **Evidence:** `internal/cluster/kubespray_lifecycle.go` (`deploy`) forwards `OS_*`
> keys from `openTofuEnv` into the ansible deploy environment.

## Commands

```bash
opencenter cluster service enable openstack-ccm
opencenter cluster service disable openstack-ccm
opencenter cluster service options openstack-ccm
```
