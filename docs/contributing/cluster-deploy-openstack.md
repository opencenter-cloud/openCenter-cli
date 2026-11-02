---
last_updated: 2026-11-02
id: cluster-deploy-openstack
title: "cluster deploy — OpenStack Provider"
sidebar_label: cluster deploy — OpenStack
description: How cluster deploy works end-to-end for the OpenStack provider, including every bootstrap step and why the Kubespray steps are currently disabled.
doc_type: explanation
audience: "developers"
tags: [openstack, deploy, bootstrap, kubespray, opentofu]
---
# cluster deploy — OpenStack Provider

**Purpose:** For developers, explains how `opencenter cluster deploy` works end-to-end for the OpenStack provider: the CLI pre-flight, the bootstrap step sequence, and the current (not the originally-designed) way Kubespray gets run.

The entry point is `cmd/cluster_deploy.go`. Orchestration lives in `internal/cluster/bootstrap_service.go`. The OpenStack-specific step sequence is built by `openstackBootstrapProvider.BuildSteps` in `internal/cluster/bootstrap_provider_infra.go` — **not** a separate `openstack_bootstrap_provider.go` file (that filename does not exist in the current tree; don't trust older references to it).

## Overview

1. **CLI pre-flight** (`cmd/cluster_deploy.go`) — resolve the cluster name, load and validate the config, check provider availability, acquire a deploy lock, verify the GitOps working tree is clean, verify the Git remote matches config, mark cluster status `running`.
2. **Bootstrap orchestration** (`internal/cluster/bootstrap_service.go`) — resolve cluster paths, open the bootstrap log, load any saved resume state, then hand off to the provider's `BuildSteps`.
3. **Step execution** (`internal/cluster/bootstrap_provider_infra.go`) — the OpenStack step sequence below.
4. **Readiness polling** — wait for the Kubernetes API to respond before declaring success.

## The current OpenStack step sequence

`openstackBootstrapProvider.BuildSteps` builds these steps, in order:

1. **`preflight`** — validates OpenStack credentials via `internal/credentials.Extractor.ExtractOpenStack()` plus `openstackprovider.PreflightOpenStack(authURL)`. Rejects empty credentials, and rejects `application_credential_id`/`application_credential_secret` still set to the `CHANGEME` placeholder.
2. **`opentofu-init`** — runs `opentofu init` in the cluster's infrastructure directory (`<gitDir>/infrastructure/clusters/<cluster>`), with `OS_*` credential env vars merged in via `credentials.OpenStackCredentials.ToEnvMap()` (`OS_AUTH_URL`, `OS_REGION_NAME`, `OS_APPLICATION_CREDENTIAL_ID`, `OS_APPLICATION_CREDENTIAL_SECRET`, `OS_USERNAME`/`OS_PASSWORD` as a fallback, plus fixed `OS_INTERFACE=public` and `OS_IDENTITY_API_VERSION=3`) and `KUBECONFIG` set to the resolved cluster kubeconfig path.
3. **`opentofu-apply`** — runs `opentofu apply -auto-approve` with the same environment. For `deployment.method: kubespray` this is also where the cluster nodes and tenant network are created (see the Kubespray note below).
4. **`bind-network-outputs`** — reads `opencenter_network_id`/`opencenter_subnet_id` from the applied OpenTofu state and fills the corresponding empty `opencenter.infrastructure.cloud.openstack` fields, persisting the config. **OpenStack only.** It lands as the last action on the infrastructure and the first before the cluster is built on top of it: `opentofu-apply` has just created the network, and hardening plus the Kubespray run both consume the inventory derived from this config. See [the network-identity section](#the-network-identity-loop) below.
5. **Kubespray steps** — added only for `deployment.method: kubespray`, running in the cluster state zone rather than the GitOps worktree so the inventory Ansible executes is the only copy:
   - `kubespray-prepare` — checks out Kubespray, builds the venv, stages the hardening role.
   - `kubespray-wait-cloudinit` — blocks until cloud-init settles on every host.
   - `kubespray-os-hardening` — conditional on `infrastructure.networking.security.os_hardening`.
   - `kubespray-deploy` — the Ansible/Kubespray cluster run.
   - `kubespray-export-kubeconfig` — fetches the admin kubeconfig into the secrets zone.

   `bind-network-outputs` (step 4) is emitted *inside* this group, between `kubespray-wait-cloudinit` and `kubespray-os-hardening`, so on a kubespray deploy the real order is `opentofu-apply` → `kubespray-prepare` → `kubespray-wait-cloudinit` → `bind-network-outputs` → `kubespray-os-hardening` → `kubespray-deploy` → `kubespray-export-kubeconfig`.
6. **`openstack-normalize-kubeconfig`** — finds whichever kubeconfig the tooling actually wrote (`opts.KubeconfigPath`, then `kubeconfig.yaml`, `kubeconfig`, or `kube_config_cluster.yml` inside the cluster directory, in that order), rewrites any `127.0.0.1`/`localhost`/`[::1]` server host to the cluster's API endpoint IP (`infrastructure.k8s_api_ip` if set, else the VRRP VIP when `vrrp_enabled: true`), and writes the result to `opts.KubeconfigPath` with mode `0600`.
7. **Network plugin install step** (`buildNetworkPluginInstallStep`) — installs the enabled CNI (Calico/Cilium/Kube-OVN) so the cluster is network-ready.
8. **`patch-coredns-toleration`** — adds a toleration for the `node.cloudprovider.kubernetes.io/uninitialized` taint to CoreDNS. Without it CoreDNS cannot schedule on tainted nodes, `source-controller` cannot resolve external endpoints, the `GitRepository` never becomes Ready, and CCM installation is blocked indefinitely.
9. **Flux bootstrap step** (`openstack-flux-bootstrap`) — added *only* when `gitops.auth.token` is configured with a non-empty provider and a real (non-placeholder) repository URL. Runs after the CNI so FluxCD's source-controller has a working network when it starts reconciling. No credential Secret is created for the shared `openCenter-gitops-base` repository itself — it's public, so its `GitRepository` source is rendered anonymously.
10. **`reconcile-sops-age-secret`** — puts the cluster's SOPS age key into the cluster as the `sops-age` Secret. It must precede anything that reads a SOPS-encrypted overlay: without the Secret the CCM override fails with `secrets "sops-age" not found`, `openstack-ccm-base` never runs, no CCM is ever installed, and the gate below polls a taint nothing can ever clear. Observed on kit-kat, where deploy blocked here while cert-manager, CSI, headlamp, metrics-server and the postgres stack sat unschedulable behind the same missing key.
11. **`rerender-ccm-network-values`** — re-renders and promotes only `services/openstack-ccm` so the CCM `override-values.yaml` carries the real `[LoadBalancer]` subnet-id. Runs only when step 4 promoted a subnet and `use_octavia` is false. See [the network-identity section](#the-network-identity-loop).
12. **`wait-for-cloud-controller-manager`** — polls until the CCM clears the uninitialized taint. The CCM is installed by Flux, so its effect can only be observed after bootstrap; this gate is what turns "cluster is up" into "cluster can actually run workloads".
13. **`reconcile-grafana-admin-secret`** — no-op unless `secrets.grafana.admin_password` is set.

There is deliberately **no automatic git push** — neither `cluster deploy`, `cluster generate`, nor `secrets sync` ever commits or pushes. The operator publishes the generated GitOps repository by hand once the deploy finishes.

### Publishing the generated repository

After a deploy that promoted network IDs (steps 4 and 11), the config holds values the rendered tree does not yet reflect. Re-render and push:

```bash
opencenter cluster generate <org>/<cluster> --force
opencenter secrets sync <org>/<cluster> --force
cd <gitops-dir>          # opencenter.gitops.repository.local_dir
git add -A
git commit -m "update networks"
git push origin
```

- `cluster generate --force` re-renders the whole tree from the current config, overwriting the existing repository contents. It accepts the `org/cluster` form; the organization prefix is stripped for path resolution and the organization itself is read back from the config.
- `secrets sync --force` rewrites the SOPS-encrypted manifests from the config even where no drift is detected — needed when a key rotated or a manifest was hand-edited.
- `git push origin` publishes the branch; Flux in the cluster then reconciles it.

### The network-identity loop

`cluster provider openstack` deliberately clears `network_id`/`subnet_id` so OpenTofu can create the tenant network (`--create-internal-network`, the default). Until lifecycle contract 3 nothing wrote the created IDs back, so the CCM rendered its `[LoadBalancer]` `subnet-id` blank on every tofu-managed cluster and allocated VIPs from the wrong scope.

Contract 3 closes it. The `openstack-nova` module exports the *effective* `network_id`/`subnet_id` (the created resource ID when it made one, otherwise the operator-supplied variable), the CLI's `outputs.tf.tpl` surfaces them as `opencenter_network_id`/`opencenter_subnet_id`, and steps 4 and 11 above promote them into config and into the rendered CCM overlay.

Two properties keep this safe to add to an existing deploy:

- **Inert by default.** `bindNetworkOutputs` reads the outputs through `internal/importer`, which returns nothing for a missing infrastructure directory or a module that predates contract 3. Kind, Magnum, VMware, baremetal and every pre-contract-3 cluster are untouched — their templates declare no `openstack-nova` module, so the outputs never appear.
- **Never overwrites.** A field the config already holds is left alone: a populated `subnet_id` means the operator chose the network with `--create-internal-network=false`, and repointing it would change a running cluster's addressing. `cluster observe` reports those as conflicts instead.

The re-render uses the scoped single-service render rather than a full `GenerateClusterTree` on purpose: a full render would also re-render `main.tf` from the mutated config, changing the `network_id`/`subnet_id` inputs OpenTofu already applied and producing a spurious `tofu plan` diff mid-deploy.

### Where the Kubespray run lives

For `deployment.method: kubespray` the Ansible run is owned by the dedicated
`kubespray-deploy` step, not by OpenTofu. An earlier design embedded a
`null_resource.run_kubespray` `local-exec` provisioner in the OpenTofu module,
which made Ansible run a second time against an already-provisioned cluster and
wasted roughly an hour of deploy time; both that provisioner and the
`buildKubespraySteps` helper that once competed with it have since been removed.
`kubespray-deploy` is the only place the playbook runs.

## State and resume

Bootstrap step results are persisted to `state.json` after each step runs (`internal/cluster/bootstrap_service.go` / `bootstrap_runtime.go`):

```
<state dir>/bootstrap/<org>/<cluster>/<generation_id>/state.json          — resume state
<state dir>/logs/bootstrap/<org>/<cluster>/bootstrap-<timestamp>.log      — full command output
```

The resume state is scoped by `metadata.generation_id`, so a destroyed-and-reinitialized cluster cannot resume from its predecessor's completed steps. The older pre-generation path `<state dir>/bootstrap/<org>/<cluster>/state.json` is never a resume source and `cluster deploy` deletes it on sight; a config with no generation identifier yet falls back to that directory until one is stamped.

Re-running `cluster deploy` reads saved state and skips steps already marked `success`. Relevant flags: `--restart` (ignore saved state, rerun everything), `--step <id>` (run exactly one step), `--from-step <id>` (run from a given step onward).

## Files involved

| File | Role |
| --- | --- |
| `cmd/cluster_deploy.go` | Entry point: flags, lock acquisition, git hygiene checks, status updates. |
| `internal/cluster/bootstrap_service.go` | Orchestrator: path resolution, config load, step execution, resume-state handling, readiness polling. |
| `internal/cluster/bootstrap_provider.go` | `lifecycleBootstrapProvider` interface and the `bootstrapStep` type every provider builds. |
| `internal/cluster/bootstrap_provider_infra.go` | The OpenStack (and shared infra-provider) step sequence described above. |
| `internal/cluster/bootstrap_plan.go` | `BootstrapPlan` types and the `--dry-run` plan renderer. |
| `internal/cluster/bootstrap_runtime.go` | Log-file and state-file path resolution. |
| `internal/credentials/extractor.go`, `internal/credentials/openstack.go` | Pull OpenStack credentials out of `v2.Config` and turn them into `OS_*` env vars. |
| `internal/cluster/network_output_binding.go` | Contract v3 network/subnet promotion into config and the scoped CCM overlay re-render. |
| `internal/importer/terraform.go` | Reads `opencenter_network_id`/`opencenter_subnet_id` from the applied OpenTofu state; shared with `cluster observe`. |
| `internal/cloud/openstack/preflight.go` | CLI availability + `auth_url` sanity checks. |

## Related

- [Cluster Init Details](cluster-init-details.md) — how the config bootstrap reads from is created.
- [Renderer Contract](rendering-contract.md) — how the GitOps repository bootstrap deploys into is generated.
- `internal/cluster/magnum_bootstrap_provider.go` — contrast with a provider that has no OpenTofu/Kubespray step at all (see [Adding New Infrastructure Providers](adding-providers.md)).
