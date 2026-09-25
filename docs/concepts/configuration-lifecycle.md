---
last_updated: 2026-09-25
id: configuration-lifecycle
title: "Configuration Lifecycle"
sidebar_label: Configuration Lifecycle
description: How cluster configuration evolves from initialization through validation, GitOps generation, deployment, and updates.
doc_type: explanation
audience: "platform teams, operators"
tags: [configuration, lifecycle, gitops, validation]
---
# Configuration Lifecycle

**Purpose:** For platform teams, explains how a cluster's configuration moves from `cluster init` through validation, GitOps generation, deployment, and ongoing updates.

## Configuration as Code

openCenter treats the cluster configuration YAML as the single declarative input for a cluster:

1. One YAML file per cluster describes the desired state.
2. The file is meant to be version-controlled (the CLI does not require this, but the layout supports it).
3. `internal/config/v2` loads, normalizes, and validates the file before any command uses it.
4. Generation and deployment are driven entirely by the validated model — there is no separate imperative script.

## Where Configuration Lives

`internal/core/paths` resolves cluster-scoped paths from a set of separate zone roots rather than one shared directory. This keeps secrets and local state out of the Git-tracked GitOps tree (see [Security Model](security-model.md)). The zones, resolved under `~/.config/opencenter/clusters/` by default:

| Zone | Root env var | Default root | Contains |
| --- | --- | --- | --- |
| Blueprints | `OPENCENTER_BLUEPRINTS_DIR` | `<clusters-dir>/blueprints` | `<organization>/<cluster>/<cluster>-config.yaml` — the cluster config input |
| GitOps | `OPENCENTER_GITOPS_DIR` | `<clusters-dir>/gitops` | `<organization>/` — the Git-tracked repository (`applications/overlays/<cluster>/`, `infrastructure/clusters/<cluster>/`) |
| Cluster state | `OPENCENTER_CLUSTER_STATE_DIR` | `<clusters-dir>/state` | `<organization>/<cluster>/` — kubeconfig, Ansible inventory, venv, `.bin/` |
| Secrets | `OPENCENTER_SECRETS_DIR` | `<clusters-dir>/secrets` | `<organization>/<cluster>/age/`, `<organization>/<cluster>/ssh/` |

Each root also has a matching `paths.*Dir` field in the CLI settings file (`opencenter settings set paths.blueprintsDir ...`). `internal/core/paths.ClusterPaths.Validate()` rejects any resolved secrets, state, or config path that is equal to or nested inside the GitOps zone — this is enforced on every path resolution, not just at `init` time.

**Evidence:** `internal/core/paths/types.go`, `internal/core/paths/secure.go`, `internal/core/paths/resolver.go`, `internal/config/cli_settings_helpers.go`

## Configuration Stages

### Stage 1: Initialization

**Command:**

```bash
opencenter cluster init my-cluster --org my-org --type openstack
```

**Process** (`internal/cluster.InitService.Initialize`):

1. Validate the cluster name and organization name.
2. Resolve cluster paths through `PathResolver` and create the blueprints, GitOps, state, and secrets directories with the modes described in [Security Model](security-model.md).
3. Build a default `v2.Config` from `internal/config/v2.NewV2Default`, which seeds provider-aware service defaults and identity settings.
4. Apply any `--param`/flag overrides supplied to `init`.
5. Generate Age and SSH key material into the secrets zone.
6. Save the config file (mode `0600`) into the blueprints zone as `<cluster>-config.yaml`.

**Why this design:** Defaults come from one typed Go source (`internal/config/v2/defaults.go`), so every new cluster starts from a validated, provider-aware baseline instead of a hand-copied template.

**Evidence:** `cmd/cluster_init.go`, `internal/cluster/init_service.go`, `internal/config/v2/defaults.go`

### Stage 2: Customization

**Methods:**

1. **Direct editing:** edit the config file with a text editor.
2. **Interactive mode:** `opencenter cluster edit <cluster>` (`cmd/cluster_edit.go`).
3. **CLI flags:** `opencenter cluster set <cluster> <path>=<value>...` (`cmd/cluster_set.go`).

`cluster set` and `init` share the same flag-merging pipeline in `internal/config/flags`, which merges configuration sources in a fixed precedence order — lowest to highest: built-in defaults, the existing file, template-derived values, then CLI-supplied values (`internal/config/flags/configuration_merger.go`). CLI flags always win over what is already on disk.

**Evidence:** `cmd/cluster_edit.go`, `cmd/cluster_set.go`, `internal/config/flags/configuration_merger.go`

### Stage 3: Validation

**Command:**

```bash
opencenter cluster validate my-cluster
```

`internal/cluster.ValidateService` loads the configuration through the same six-stage pipeline every other command uses (`internal/config/v2/loader.go`):

```
1. Parse YAML
2. Normalize (canonicalize provider names, resolve aliases)
3. Resolve references (${ref:}, ${env:}, ${file:})
4. Apply defaults (hydrate empty fields from the provider/region registry)
5. Validate (schema, business rules, provider, deployment, service checks)
6. Freeze (mark the result immutable)
```

A failure at any stage returns immediately with a stage-tagged error (for example `stage 5 (validate): ...`), so the reported error already indicates which phase produced it.

**Evidence:** `internal/config/v2/loader.go`, `internal/config/v2/validator.go`, `cmd/cluster_validate.go`

### Stage 4: Generate (GitOps Repository Rendering)

**Command:**

```bash
opencenter cluster generate my-cluster
```

`internal/cluster.SetupService.Setup` loads and validates configuration, then calls `internal/gitops` directly:

```
1. gitops.CopyBase               — copy the base repository structure
2. gitops.RenderClusterAppsWithEncryption
                                  — render application descriptors/catalog actions,
                                    encrypt overlay values before promotion
3. gitops.RenderInfrastructureCluster
                                  — render provider-selected infrastructure assets
4. gitops.RenderClusterFluxBridge — render the per-cluster Flux bridge
5. tofu.Provision                — non-Kind providers only
```

Generated files are tracked by repository-relative SHA-256 and mode records in the version-2 ledger `.opencenter/ownership/clusters/<cluster>.json`; exact repository-wide generated files are recorded separately in version-2 `.opencenter/ownership/global.json`. The cluster ledger covers only the active cluster's explicit scopes — `applications/overlays/<cluster>/`, `infrastructure/clusters/<cluster>/`, and `clusters/<cluster>/` — with `clusters/<cluster>/flux-system/` reserved for Flux bootstrap and excluded. The generated bridge files beside that Flux directory remain in the cluster scope. The global ledger is an exact allowlist (`.gitignore`, `README.md`, and the two repository `.gitkeep` files), not a recursive root scope. Applications-only and single-service renders update and prune only their active scope, preserving sibling cluster records and unrelated state; they do not rewrite global ownership. Missing staged defaults may seed `custom/` files, but existing custom content and hash-verified secret-sync artifacts are outside generator ownership.

The pre-v1 ownership transition is fail-fast: a repository-root or current overlay `.opencenter-generated.json` legacy manifest is rejected before mutation rather than migrated implicitly. Dry-run performs the same preflight without writing; prune-disabled runs report candidates and retain them; `--adopt-generated` is an explicit opt-in that can back up and claim only an untracked planned collision. Modified tracked files and unknown user-authored files remain protected, and `--force` never overrides ownership safety. Put hand-authored manifests in a service's `custom/` directory before regenerating.

**Evidence:** `internal/cluster/setup_service.go`, `internal/gitops/ownership.go`, `cmd/cluster_migrate_layout.go`. See [GitOps Workflow](gitops-workflow.md) for the full rendering contract.

### Stage 5: Deployment

**Command:**

```bash
opencenter cluster deploy my-cluster
```

`internal/cluster.BootstrapService` selects a provider-specific step sequence and persists progress in `bootstrap-state.json`, so a failed run can resume with `--step` or `--from-step` instead of starting over. OpenStack, VMware, and bare metal share an infrastructure bootstrap implementation (OpenTofu provisioning, then Kubernetes/Ansible-driven cluster bring-up and Flux install); Kind and Magnum have their own dedicated bootstrap providers. See [Cluster Lifecycle](../CODEMAPS/cluster-lifecycle.md) for the provider table.

**Evidence:** `cmd/cluster_deploy.go`, `internal/cluster/bootstrap_service.go`, `internal/cluster/bootstrap_provider_infra.go`

### Stage 6: Operation

Once deployed, day-2 commands read or mutate the same validated configuration:

* `cluster status` — cluster and service health.
* `cluster service enable/disable/status` — toggle and inspect platform services.
* `cluster pool add/scale/remove` — manage worker pools.
* `cluster drift detect/reconcile` — compare configuration against live provider state (OpenStack, VMware only — see [Drift Detection](drift-detection.md)).
* `cluster backup create/restore/list` — archive and restore cluster/config state.

### Stage 7: Updates

```
1. Edit the configuration file (directly, `cluster edit`, or `cluster set`)
2. opencenter cluster validate my-cluster
3. opencenter cluster generate my-cluster
4. git commit && git push   (inside the GitOps zone)
5. FluxCD detects the change and reconciles
```

Most service and topology changes are in-place: regenerate, commit, and let Flux reconcile. Changing the infrastructure provider is not an in-place update — it requires a new cluster.

### Stage 8: Decommission

**Command:**

```bash
opencenter cluster destroy my-cluster
```

`internal/cluster.DestroyService` runs a provider-specific `lifecycleDestroyProvider` (an OpenTofu destroy path for OpenStack/VMware/bare metal, direct Kind teardown for Kind, Magnum cluster deletion for Magnum). Destroy does not take an automatic backup — run `cluster backup create` first if you need one; see [Backup and Restore](../operations/backup-and-restore.md).

**Evidence:** `cmd/cluster_destroy.go`, `internal/cluster/destroy_service.go`

## Configuration Drift

Drift detection compares the validated configuration against a provider's live API state. It is a distinct concept from FluxCD's own continuous reconciliation of the GitOps repository against the cluster — see [Drift Detection](drift-detection.md) for what is actually compared and which providers are supported, and [GitOps Workflow](gitops-workflow.md) for how FluxCD reconciles Kustomizations and HelmReleases.

## Further Reading

* [Architecture](../architecture.md) — system design and components
* [GitOps Workflow](gitops-workflow.md) — repository structure and reconciliation
* [Drift Detection](drift-detection.md) — supported providers and comparison scope
* [Security Model](security-model.md) — path zone separation and key lifecycle
* [Validate Configuration](../operations/validate-configuration.md) — validation procedures
* [Configuration Schema](../reference/configuration-schema.md) — complete field reference

## Evidence

* Loading pipeline: `internal/config/v2/loader.go`, `internal/config/v2/validator.go`
* Path zones: `internal/core/paths/types.go`, `internal/core/paths/secure.go`, `internal/core/paths/resolver.go`
* Lifecycle services: `internal/cluster/init_service.go`, `configure_service.go`, `validate_service.go`, `setup_service.go`, `bootstrap_service.go`, `destroy_service.go`
* Generated-file ownership: `internal/gitops/ownership.go`
* Configuration precedence: `internal/config/flags/configuration_merger.go`
