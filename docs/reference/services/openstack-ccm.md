---
last_updated: 2026-11-03
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

The catalog entry uses namespace stage `openstack-ccm`, override values, base path `applications/base/services/openstack-ccm`, and override dependency `openstack-ccm-namespace`. `SkipSourcesDependency: true` drops the `sources` dependency the other services carry: CCM is bootstrap-priority and must reconcile before the sources aggregate gate is ready, because DNS is unavailable until CCM clears the cloud-provider-uninitialized taint while sources needs DNS to fetch repositories. No explicit service descriptor is present.

### Upgrade timing

The CCM runs as a `hostNetwork` DaemonSet pinned to the control-plane nodes, and
the upstream chart renders `updateStrategy: {type: RollingUpdate}` with no
`maxUnavailable`, so Kubernetes defaults to `maxUnavailable: 1`. Every
control-plane node is rotated one at a time -- delete, create, wait for Ready --
and the Helm upgrade only succeeds if all N rotations finish inside
`HelmRelease.spec.timeout`.

The upstream base ships `timeout: 5m`, which is too small for that. On a
3 control-plane cluster a healthy rollout missed its own deadline by 25 seconds:
the upgrade deadline expired at `20:47:08` and the replacement pods were created
at `20:47:33`. The release was marked `failed`, and because the CCM deliberately
preserves evidence instead of rolling back, that one spurious failure became a
permanent `Stalled=True` that nothing clears without a human.

`openstackCCMBaseKustomizationPatchesRenderer` therefore patches two things
beyond remediation:

| Field | Value | Why |
|-------|-------|-----|
| `HelmRelease.spec.timeout` | `9m` | Derived as `openstackCCMBaseKustomizationTimeout - 1m`. The `<service>-base` Kustomization runs with `wait: true` and health-checks this HelmRelease, so the HelmRelease must be allowed strictly less time than the Kustomization is willing to wait, or the health check abandons a healthy in-flight upgrade and reports a second false failure. |
| `upgrade.remediation.retries` | `3` | With `retries: 0`, one slow upgrade stalls the release forever: the rendered manifest is already correct by then, so drift detection finds no diff and never re-attempts. Retries let a transient Keystone slowdown or a cold image pull recover unattended, while a genuinely bad config still ends in the preserved-evidence `Stalled` state. |

`remediateLastFailure: false` stays on both `install` and `upgrade` -- that part
of the contract is unchanged. `autoBaseKustomizationTimeout`
(`internal/gitops/auto_descriptor.go`) is the single source of truth for the
Kustomization side of the pair, and
`TestOpenstackCCMTimeoutsFitInsideBaseKustomization` fails if the two drift.

### Pod security labels

The catalog marks `openstack-ccm` with `PrivilegedNamespace: true`, the same
contract as `openstack-csi`, because the CCM pods use `hostNetwork` and
`hostPath` volumes -- both rejected outright by the `baseline` and `restricted`
Pod Security profiles. The generated namespace therefore carries:

```yaml
pod-security.kubernetes.io/enforce: privileged
pod-security.kubernetes.io/warn: baseline
pod-security.kubernetes.io/audit: baseline
```

`warn`/`audit` at `baseline` keep the security signal that `enforce: privileged`
erases: privileged pods still produce admission warnings and audit entries, so a
namespace quietly accumulating host-namespace workloads stays visible without
blocking bootstrap.

`pod-security.kubernetes.io/enforce-version` is deliberately never emitted.
Absent means "track the newest Kubernetes version the API server knows"; pinning
it would freeze PSA semantics. (`enforce-version: latest` is equivalent to
omitting it -- both mean "newest known version" -- but it is not CLI-authored,
and an unpinned version means the effective profile for a namespace can change
on a cluster upgrade without any manifest changing.) If that label appears on a
generated namespace, it came from outside this generator.

## Credentials (OCTR-750)

The external OpenStack cloud-controller-manager resolves its `external_openstack_*`
settings from environment lookups such as `OS_AUTH_URL`, `OS_APPLICATION_CREDENTIAL_ID`,
and `OS_APPLICATION_CREDENTIAL_SECRET`. GitOps is the single CCM owner. Kubespray only
configures kubelet with `cloud_provider: external`; its provider-specific
`external_cloud_provider` setting must remain empty so Kubespray does not install a
second OpenStack CCM.

During a cluster deploy, the CLI forwards every `OS_*` variable from the OpenTofu
environment into the Kubespray ansible run (OCTR-750), preserving the deployment's
credential flow without requiring username/password credentials.

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
