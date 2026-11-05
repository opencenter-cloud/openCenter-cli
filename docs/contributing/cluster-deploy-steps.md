---
last_updated: 2026-11-05
id: cluster-deploy-steps
title: "cluster deploy — Step Matrix per Provider"
sidebar_label: cluster deploy — Step Matrix
description: Which bootstrap steps each provider (OpenStack, Kind, VMware, Baremetal) runs during cluster deploy, in order, with the gates that add or drop each step.
doc_type: reference
audience: "developers, contributors"
tags: [deploy, bootstrap, providers, openstack, kind, vmware, baremetal]
---
# cluster deploy — Step Matrix per Provider

**Purpose:** For developers and contributors, tabulates the exact ordered
bootstrap steps `opencenter cluster deploy` runs for each infrastructure
provider, and the configuration gates that add or drop each step. This is a
lookup table derived from the step builders; for the reasoning behind the
OpenStack sequence see
[cluster deploy — OpenStack Provider](cluster-deploy-openstack.md).

The step lists come from `internal/cluster/bootstrap_service.go`
(`buildBootstrapSteps` dispatch) and the two builders it selects:
`openstackBootstrapProvider.BuildSteps` in
`internal/cluster/bootstrap_provider_infra.go` (shared by OpenStack, VMware,
and Baremetal) and `kindBootstrapProvider.BuildSteps` in
`internal/cluster/kind_bootstrap_provider.go`.

## Dispatch

`buildBootstrapSteps` selects a builder from `infrastructure.provider`:

| `infrastructure.provider` | Step builder |
| --- | --- |
| `openstack` (also the empty default) | `openstackBootstrapProvider.BuildSteps` |
| `vmware` / `vsphere` / `baremetal` | the same `openstackBootstrapProvider.BuildSteps` |
| `kind` | `kindBootstrapProvider.BuildSteps` |
| `magnum` | `magnumBootstrapProvider.BuildSteps` (see [providers reference](../reference/providers.md)) |
| `aws` / `gcp` / `azure` | a three-step Terraform stub, but `checkProviderAvailability` in `cmd/provider_availability.go` rejects these before any step runs |
| anything else | `unsupported provider` error |

VMware and Baremetal reuse the OpenStack builder. Their generated templates
declare no `openstack-nova` module, so the two network-identity steps
(`bind-network-outputs`, `rerender-ccm-network-values`) never appear for them.

## Command-level gates (every provider, before any step)

These run in `cmd/cluster_deploy.go` and `BootstrapService.Bootstrap` before
the first numbered step, for all providers:

| Gate | Effect |
| --- | --- |
| provider-availability check | rejects `aws` / `gcp` / `azure` |
| `checkDeployPrerequisites` | requires `sops` in `PATH` and Helm **4** |
| deploy lock | acquires a 1-hour lock; prompts on conflict |
| `checkSecretsSynced` | every required secret manifest is materialized by `secrets sync` and wired into its service overlay `kustomization.yaml` |
| `rehomeGitURL` + origin verify/align | the configured `git_url` is reachable and the local `origin` remote matches it |
| `validateBootstrapConfig` | cluster name and provider are set; Kind additionally requires the `infrastructure.kind` block |

## Provision and deploy steps

`✔` = runs · `✔*` = runs only when its gate is on · `—` = not in this provider's
step list. The Kind column replaces rows 1–18 with the Kind-only sequence below.

| # | Step ID | OpenStack | VMware | Baremetal |
| --- | --- | :---: | :---: | :---: |
| 1 | `preflight` | ✔ | ✔* | ✔* |
| 2 | `opentofu-init` (NeverSkip) | ✔ | ✔ | ✔ |
| 3 | `opentofu-apply` | ✔ | ✔ | ✔ |
| 4 | `bind-network-outputs` | ✔* | — | — |
| 5 | `kubespray-prepare` | ✔* | ✔* | ✔* |
| 6 | `kubespray-wait-cloudinit` | ✔* | ✔* | ✔* |
| 7 | `kubespray-os-hardening` | ✔* | ✔* | ✔* |
| 8 | `kubespray-deploy` | ✔* | ✔* | ✔* |
| 9 | `kubespray-export-kubeconfig` | ✔* | ✔* | ✔* |
| 10 | `openstack-normalize-kubeconfig` | ✔ | ✔ | ✔ |
| 11 | `openstack-install-network-plugin` | ✔ | ✔ | ✔ |
| 12 | `patch-coredns-toleration` | ✔ | ✔ | ✔ |
| 13 | `openstack-flux-bootstrap` | ✔* | ✔* | ✔* |
| 14 | `reconcile-sops-age-secret` | ✔* | ✔* | ✔* |
| 15 | `rerender-ccm-network-values` | ✔* | — | — |
| 16 | `openstack-gitops-push` | ✔* | ✔* | ✔* |
| 17 | `wait-for-cloud-controller-manager` | ✔* | ✔* | ✔* |
| 18 | `reconcile-grafana-admin-secret` | ✔* | ✔* | ✔* |

`bind-network-outputs` (row 4) is emitted inside the Kubespray group, between
`kubespray-wait-cloudinit` and `kubespray-os-hardening`, so on a full OpenStack
Kubespray deploy the real order is rows 1–3, then 5, 6, 4, 7, 8, 9, then 10–18.

### Kind-only sequence

The Kind provider builds a wholly different list; rows 1–18 above never appear.

| # | Step ID | Kind (Gitea) | Kind (external Git, e.g. GitHub) |
| --- | --- | :---: | :---: |
| K1 | `kind-create` | ✔ | ✔ |
| K2 | `kind-export-kubeconfig` | ✔ | ✔ |
| K3 | `kind-label-workers` | ✔ | ✔ |
| K4 | `kind-install-cni` | ✔* | ✔* |
| K5 | `gitea-attach-kind` (NeverSkip) | ✔ | — |
| K6 | `flux-bootstrap` | ✔ | ✔ |
| K7 | `reconcile-sops-age-secret` | ✔ | ✔ |
| K8 | `reconcile-grafana-admin-secret` | ✔ | ✔ |
| K9 | `gitea-source-rehome` (NeverSkip) | ✔ | ✔ |
| K10 | `gitea-rebase` | ✔ | — |
| K11 | `gitops-push` | ✔ | — |
| K12 | `flux-verify` | ✔ | ✔ |

### Readiness tail (every provider, after the steps)

| Step | Behavior |
| --- | --- |
| `waitForReady` | Kind: `WaitReady` on the Kind provider. OpenStack/VMware/Baremetal/Magnum: poll `kubectl cluster-info` until the API answers, then read the endpoint with `kubectl config view --minify`. Default timeout 30 minutes. |

## Gates that flip `✔*` to `✔`

- **Kubespray steps (rows 5–9):** present only when `deployment.method` is
  `kubespray` (the default). A non-Kubespray method drops rows 4–9, so the
  sequence goes `opentofu-apply` → `openstack-normalize-kubeconfig`.
- **`bind-network-outputs` (row 4) and `rerender-ccm-network-values` (row 15):**
  present only when `cfg.IsOpenStack()` is true (provider `openstack` or empty).
  VMware and Baremetal templates declare no `openstack-nova` module, so these
  are always absent for them.
- **`preflight` credentials (row 1):** the check differs per provider —
  OpenStack authenticates against Keystone live and asserts the region; VMware
  requires `secrets.vsphere_csi` (host, username, password) plus static nodes;
  Baremetal requires static master nodes and an SSH user. The bastion SSH
  sub-check runs only when a bastion is enabled with an address.
- **The Flux tail (rows 13–18):** one block gated on `gitops.auth.token` having
  a non-empty provider **and** `ConfiguredGitURL()` returning a real
  (non-`CHANGEME`) repository URL. A freshly initialized VMware or Baremetal
  config still carries the placeholder URL, so the entire block is absent until
  a repository URL is bound.
- **`kind-install-cni` (K4):** runs only when `infrastructure.kind.disable_default_cni`
  is true and Calico is the enabled plugin; otherwise Kindnet provides the CNI
  and the step is a no-op.
- **`gitea-attach-kind` / `gitea-rebase` / `gitops-push` (K5, K10, K11):** dropped
  when the GitOps provider is external (GitHub/GitLab), because
  `flux bootstrap github` commits straight to the remote — there is no local
  Gitea to attach to or push back to.

## Verified orderings

The following orders are asserted by `internal/cluster/bootstrap_service_test.go`
and `internal/cluster/bootstrap_provider_infra_test.go`.

- **OpenStack (full):** `preflight` → `opentofu-init` → `opentofu-apply` →
  `kubespray-prepare` → `kubespray-wait-cloudinit` → `bind-network-outputs` →
  `kubespray-os-hardening` → `kubespray-deploy` → `kubespray-export-kubeconfig`
  → `openstack-normalize-kubeconfig` → `openstack-install-network-plugin` →
  `patch-coredns-toleration` → `openstack-flux-bootstrap` →
  `reconcile-sops-age-secret` → `rerender-ccm-network-values` →
  `openstack-gitops-push` → `wait-for-cloud-controller-manager` →
  `reconcile-grafana-admin-secret`
- **VMware / Baremetal (repository URL unbound):** `preflight` →
  `opentofu-init` → `opentofu-apply` → `kubespray-prepare` →
  `kubespray-wait-cloudinit` → `kubespray-os-hardening` → `kubespray-deploy` →
  `kubespray-export-kubeconfig` → `openstack-normalize-kubeconfig` →
  `openstack-install-network-plugin` → `patch-coredns-toleration`
- **Kind (Gitea):** `kind-create` → `kind-export-kubeconfig` →
  `kind-label-workers` → `kind-install-cni` → `gitea-attach-kind` →
  `flux-bootstrap` → `reconcile-sops-age-secret` →
  `reconcile-grafana-admin-secret` → `gitea-source-rehome` → `gitea-rebase` →
  `gitops-push` → `flux-verify`
- **Kind (GitHub):** the Kind (Gitea) order minus `gitea-attach-kind`,
  `gitea-rebase`, and `gitops-push`.

The invariants these tests lock: `opentofu-apply` before `bind-network-outputs`
before `kubespray-deploy`; and `openstack-flux-bootstrap` before
`reconcile-sops-age-secret` before `rerender-ccm-network-values` before
`openstack-gitops-push` before `wait-for-cloud-controller-manager`. The last
chain exists because the CCM is installed by Flux from a SOPS-encrypted
overlay, so the gate must run last or it polls a taint nothing can clear.

## Step filtering and resume

`cluster deploy` flags reshape the selected list without changing its order:
`--step <id>` runs exactly one step, `--from-step <id>` runs from a step onward,
and `--restart` ignores saved resume state and reruns everything. Completed
steps are skipped from `state.json` unless a step is marked `NeverSkip`
(`opentofu-init`, `gitea-attach-kind`, `gitea-source-rehome`). See
[cluster deploy — OpenStack Provider](cluster-deploy-openstack.md#state-and-resume)
for the state layout.

## Related

- [cluster deploy — OpenStack Provider](cluster-deploy-openstack.md) — the
  reasoning behind the OpenStack sequence and the network-identity loop.
- [Cluster Init Details](cluster-init-details.md) — how the config deploy reads
  is created.
- [Renderer Contract](rendering-contract.md) — how the GitOps tree deploy
  publishes is generated.
- [Infrastructure Providers Reference](../reference/providers.md) — provider
  lifecycle and drift boundaries.
