---
last_updated: 2026-11-02
id: code-structure
title: "Codebase Organization"
sidebar_label: Codebase Organization
description: Map of the openCenter-cli repository -- top-level layout, package boundaries, and where to find each subsystem.
doc_type: explanation
audience: "developers"
tags: [contributing, architecture]
---
# Codebase Organization

**Purpose:** For developers, explains how the openCenter-cli codebase is organized and where to find specific functionality.

## Repository Layout

```
openCenter-cli/
├── cmd/                    # CLI commands (Cobra), one file per command/subcommand
├── internal/                # All non-exported Go packages (the bulk of the codebase)
├── docs/                     # Documentation (Markdown with YAML frontmatter)
├── tests/features/           # BDD scenarios (Gherkin) and Godog step definitions
├── schema/                   # Generated JSON Schema (opencenter-v2.schema.json)
├── testdata/                  # Test fixtures (sample orgs/clusters, config fixtures)
├── hack/                      # Dev scripts: local Gitea, shell integration, doc audits
├── bin/                       # Compiled binaries (gitignored)
├── main.go                    # CLI entry point
├── go.mod                      # Go module definition (module github.com/opencenter-cloud/opencenter-cli)
└── .mise.toml                   # Tool versions and task definitions
```

## Command layer (`cmd/`)

Every CLI command is a `.go` file named `<noun>_<verb>.go`. There is no separate "cmd/cluster/" subpackage -- all command files live directly in `cmd/` as package `cmd`.

**Cluster commands** (`cluster_*.go`) -- the largest group, covering the full cluster lifecycle: `cluster_init.go`, `cluster_configure.go`, `cluster_validate.go`, `cluster_generate.go`, `cluster_render.go`, `cluster_deploy.go` (+ `cluster_deploy_plan.go` for `--dry-run` output), `cluster_destroy.go`, `cluster_list.go`, `cluster_use.go`, `cluster_active.go`, `cluster_describe.go`, `cluster_status.go` (+ `cluster_status_inventory.go`), `cluster_set.go`, `cluster_edit.go`, `cluster_normalize.go`, `cluster_export.go`, `cluster_import.go`, `cluster_template.go`, `cluster_service.go` (+ `cluster_service_storage.go`), `cluster_pool.go` (worker pools), `cluster_drift.go` (+ `cluster_drift_to_config.go`), `cluster_backup.go`, `cluster_lock.go`, `cluster_doctor.go`, `cluster_sync_status.go`, `cluster_migrate_layout.go`, `cluster_provider.go` (+ `cluster_provider_openstack.go`), `cluster_check_keys.go`, `cluster_rotate_keys.go`, `cluster_revoke_key.go`, `cluster_env.go`, `cluster_validate_manifests.go`.

**Secrets commands** (`secrets_*.go`): `secrets.go` (parent + CRUD), `secrets_keys.go` / `secrets_keys_ops.go` / `secrets_keys_reconcile.go` / `secrets_keys_set_primary.go`, `secrets_sync.go`, `secrets_validate.go`, `secrets_sops.go` (+ `secrets_sops_helpers.go`), `secrets_router.go`, `secrets_login.go`, `secrets_file_backend.go`, `secrets_helpers.go`.

**Settings commands** (`config_*.go`, Cobra `Use: "settings"`): `config.go`, `config_edit.go`, `config_ide.go` (schema + editor setup), `config_explain.go`, `config_helpers.go`.

**Utility commands**: `version.go`, `shell_init.go`, `plugins.go`, `root.go` (root command, global flags), `global_options.go`, `output_helpers.go`, `provider_availability.go`, `exit_codes.go`, `doc.go`.

Tests follow the source 1:1: `cluster_init_integration_test.go`, `cluster_deploy_integration_test.go`, `ga_command_surface_test.go` (asserts on the full command tree), `ga_readiness_property_test.go`, etc.

## Internal packages (`internal/`)

### Configuration (`internal/config/`)

* Top level: `cli_settings.go` / `cli_settings_helpers.go` (user preferences, cluster defaults, plugin checksums, and zone-root resolution such as `ResolveClustersDir`), `manager.go` (load/cache orchestration), `persistence.go` (config/state directory resolution), `status.go` (per-cluster stage/status tracking).
* `defaults/` -- provider/region default registries.
* `flags/` -- CLI flag parsing and struct mutation via reflection (used for dotted override flags like `opencenter.infrastructure.compute.worker_count=5`).
* `overlay/` -- shared overlay-unit types (`UnitsConfig`, `CustomerManagedConfig`, `SOPSGenerationConfig`, `Secrets`) used by both the active config model and v2.
* `registry/` -- provider/service registry glue.
* `services/` -- one file per platform service (`keycloak.go`, `cert_manager.go`, `prometheus_stack.go`, ...), plus `dependency_validator.go`, `provider_registry.go`, `provider_validator.go`, `secrets_validator.go`, `deprecations.go`, `default_services.go`.
* `v2/` -- the authoritative v2 configuration model: struct definitions, loader, validator, defaults, cache, error types.
* `v2schema/` -- the JSON Schema generator that produces `schema/opencenter-v2.schema.json` from the v2 Go structs.
* `cache/` -- config load caching.

### GitOps rendering (`internal/gitops/`)

* `embed.go` -- `//go:embed all:gitops-base-dir all:templates`, exposing the embedded template filesystem as `Files`.
* `copy.go` -- convention-based rendering helpers (`shouldSkipFile`, `RenderSingleService`, `RenderClusterAppsAtomic`); `shouldSkipFile` is deprecated in favor of descriptor-driven planning (see [Renderer Contract](rendering-contract.md)).
* `descriptor_renderer.go` -- descriptor-driven rendering: `planClusterAppActions`, `validateDescriptorCoverage` (fails the build if an embedded template file has no descriptor owner).
* `render_diagnostics.go` -- structured `RenderDiagnostics` / `DescriptorDecision` / `ActionDiagnostic` output.
* `gitops-base-dir/` -- the embedded base repository skeleton (bootstrap-owned pieces such as `flux-system/`).
* `templates/cluster-apps-base/` -- embedded per-service templates (`services/`, `managed-services/`, `customer-managed/`) plus the root `kustomization.yaml` template.
* `templates/infrastructure-cluster-template/` -- OpenTofu template files (`main-default.tf.tpl`, `main-vmware.tf.tpl`, `main-baremetal.tf.tpl`, `variables.tf.tpl`, `Makefile.tpl`, `inventory/`).
* `templates/cluster-flux-bridge/`, `templates/kind-config.yaml.tpl` -- Flux bridge manifest and Kind cluster config templates.
* `stages/` -- per-stage rendering helpers.

### Service plugin registry (`internal/services/`)

* `plugin.go` -- `ServicePluginManifest` type (service metadata: name, dependencies, template refs, validation rules).
* `registry.go` -- plugin registration and dependency-ordered lookup.
* `descriptors/` -- YAML overlay descriptors (one per service) that own rendering topology; `descriptors/data/` holds the descriptor files themselves. See [Renderer Contract](rendering-contract.md) and [Descriptor Condition Schema](descriptor-condition-schema.md).
* `plugins/` -- per-service plugin implementations.

### Secrets and encryption (`internal/sops/`, `internal/secrets/`, `internal/secretartifacts/`, `internal/barbican/`)

* `internal/sops/` -- SOPS/Age operations: `manager.go`, `keys.go` (Age key generation/storage), `encrypt.go`, `git.go` (encrypted-file Git integration), `key_manager.go`, `overlay_files.go`.
* `internal/secrets/` -- multi-cluster secret lifecycle: `manager.go`, `multi_cluster.go`, `registry.go`, `revocation.go`, `reconcile.go`, `hooks.go`.
* `internal/secretartifacts/` -- planning and state tracking for generated secret manifests (`planner.go`, `state.go`).
* `internal/barbican/` -- OpenStack Key Manager (Barbican) client (`client.go`, `auth.go`, `token.go`).

### Providers (`internal/cloud/`, `internal/provision/`, `internal/tofu/`)

* `internal/cloud/openstack/` -- OpenStack drift detection, discovery, and preflight checks.
* `internal/cloud/vmware/` -- VMware/vSphere drift detection.
* `internal/cloud/kind/` -- Kind cluster lifecycle; it is not a `CloudProvider` drift implementation.
* `internal/cloud/magnum/` -- OpenStack Magnum managed-Kubernetes API client; its lifecycle wiring lives in `internal/cluster/magnum_*` (see [Adding New Infrastructure Providers](adding-providers.md)).
* `internal/cluster/bootstrap_provider_infra.go` -- shared OpenTofu bootstrap for OpenStack, VMware, and baremetal; `kind_bootstrap_provider.go` and `magnum_bootstrap_provider.go` own their dedicated lifecycle paths.
* `internal/provision/` -- embedded OpenTofu/Terraform provisioning templates (`embed.go`).
* `internal/tofu/` -- OpenTofu execution wrapper (falls back to `terraform` binary if `tofu` is unavailable).

### Cluster lifecycle orchestration (`internal/cluster/`)

Business logic behind the `cmd/cluster_*.go` commands: `init_service.go`, `configure_service.go` (+ provider orchestrators such as `openstack_configure_orchestrator.go`, `magnum_configure_orchestrator.go`), `setup_service.go` (backs `cluster generate`), `validate_service.go` (+ `validation_formatter.go`, `validation_report.go`), `bootstrap_service.go` / `bootstrap_provider.go` / `bootstrap_plan.go` / `bootstrap_runtime.go` (backs `cluster deploy`; provider-specific steps in `bootstrap_provider_infra.go`), `destroy_service.go` / `destroy_provider.go`, `configure_storage.go`, `configure_dns.go`, `configure_git_auth.go`, `admin_secrets.go`, `sops_age_secret.go`, `tofu_binary.go`.

### Validation (`internal/core/validation/`)

Engine + registry + typed validators. See [Validation Rules](../reference/validation-rules.md) for the full validator inventory and [Cluster Validate Execution Flow](validation.md) for how `cluster validate` drives it.

### Security (`internal/security/` and `internal/util/security/`)

Two distinct packages -- do not confuse them:

* `internal/security/` -- CLI-facing security controls: `audit_logger.go` (structured audit log of CLI operations), `command_runner.go` (safe external-command execution), `command_sanitizer.go` (argument sanitization), `credential_masker.go` (secret redaction in output/logs), `input_validator.go` (path traversal / injection checks on user input).
* `internal/util/security/` -- lower-level security utilities shared across packages: `credential_masker.go` (a separate, more general masker implementation), `secure_temp_file.go`, `interfaces.go`.

See [Audit Signing Key](../reference/audit-key.md) for what the audit logger actually records and signs.

### Shared utilities (`internal/util/`)

* `crypto/` -- Age/SSH key generation and validation (`key_generator.go`, `ssh_key_generator.go`, `key_validator.go`, `key_manager.go`).
* `errors/` -- error aggregation, wrapping, and structured-error middleware.
* `files/` -- atomic file writes (`atomic_writer.go`).
* `fs/` -- filesystem abstraction wrapper (for testability).
* `metrics/` -- lightweight in-process metrics.
* `security/` -- see above.
* `reflection.go` -- reflection helpers used by the dotted-flag override mechanism.

### Other packages

* `internal/ansible/` and `internal/observability/` referenced in older documentation **do not exist on this branch** -- do not link to them. The shared bootstrap invokes OpenTofu from `internal/cluster`; the generated OpenTofu module owns the Kubespray `local-exec` invocation. Structured logging lives in `internal/logging/`.
* `internal/core/paths/` -- cluster path-layout resolution (org-based directory strategy).
* `internal/credentials/` -- cloud credential extraction from `v2.Config` (`extractor.go`, `openstack.go`, `aws.go`).
* `internal/di/` -- dependency-injection container (`app.go`, `container.go`, `providers.go`) that wires services (like `BootstrapService`) with their dependencies.
* `internal/importer/` -- live-cluster scan/import (`scanner.go`, `detectors.go`, `apply.go`, `write_plan.go`).
* `internal/localdev/` -- local development environment (Kind + Gitea) lifecycle (`cluster.go`, `exec.go`, `layout.go`).
* `internal/logging/` -- global structured logger (`logging.go`).
* `internal/operations/` -- drift detection (`drift_detector.go`) and backup/restore (`backup_manager.go`).
* `internal/plugins/` -- external CLI plugin discovery and checksum verification (`loader.go`).
* `internal/resilience/` -- retry (`retry.go`), circuit breaker (`circuit_breaker.go`), distributed/local lock manager (`lock_manager.go`, with a Redis-backed implementation).
* `internal/template/` -- general-purpose template engine with caching, sandboxing, and composition (`engine.go`, `sandbox.go`, `cache.go`, `composition.go`, `embedded_registry.go` -- the latter's `inferServices` lists every service name the renderer can discover from the embedded template filesystem).
* `internal/testenv/` -- isolated CLI config/state directories for tests (`cli_dirs.go`, `loopback.go`).
* `internal/testing/` -- shared test helpers/mocks/generators.
* `internal/ui/` -- prompts, guided-flow prompter, error formatting.

## Testing (`tests/`)

BDD scenarios live in `tests/features/*.feature` (Gherkin), executed via [Godog](https://github.com/cucumber/godog): `cli_config.feature`, `cluster_generate_deploy.feature`, `cluster_init.feature`, `cluster_selection.feature`, `config_template_rendering.feature`, `secrets.feature`, `validation.feature`, `workflow.feature`. Step definitions live in `tests/features/steps/` (`helpers.go`, `steps_test.go`). Scenarios are tagged (`@wip`, `@init`, `@deploy`, `@keycloak`, `@cert-manager`, and dozens of feature-specific tags); `@wip` scenarios are excluded from the default `mise run godog` run. See [Testing Guide](testing-guide.md).

## Configuration storage on disk

```
~/.config/opencenter/clusters/
├── blueprints/<organization>/<cluster>/<cluster>-config.yaml
├── gitops/<organization>/
│   ├── infrastructure/clusters/<cluster>/
│   │   ├── inventory/                # Kubespray inventory (gitignored)
│   │   ├── venv/                     # Python virtualenv (gitignored)
│   │   ├── kubespray/                # Kubespray checkout (gitignored)
│   │   ├── .bin/                     # local binaries (gitignored)
│   │   └── kubespray-outputs.json    # Tofu outputs (gitignored)
│   ├── applications/overlays/<cluster>/
│   └── .sops.yaml
└── secrets/<organization>/<cluster>/
    ├── age/keys/<cluster>-key.txt
    ├── ssh/<cluster> (+ .pub)
    └── kubernetes/kubeconfig.yaml
```

See [File Locations](../reference/file-locations.md) for the full, verified path list and the environment variables that override each root.

## Naming and organization conventions

* Commands: `<noun>_<verb>.go` (e.g. `cluster_init.go`, `secrets_keys_ops.go`).
* Tests: `<name>_test.go` (unit), `<name>_property_test.go` (property-based, via `gopter`), `<name>_integration_test.go` (integration).
* Package documentation: `doc.go` per package.
* Errors: wrapped with `fmt.Errorf("...: %w", err)` for context; aggregated failures use `internal/util/errors`.
* Embedded resources: `//go:embed` directives in `embed.go` files (`internal/gitops/embed.go`, `internal/provision/embed.go`).

## Finding functionality: quick pointers

**Add a command:** create `cmd/cluster_<action>.go`, implement a `newCluster<Action>Cmd() *cobra.Command`, register it in `cmd/cluster.go`.

**Change configuration shape:** edit `internal/config/v2/config.go` (structs), `internal/config/v2/` validator and defaults files, then regenerate the schema with `mise run schema-v2`.

**Add a provider:** see [Adding New Infrastructure Providers](adding-providers.md) -- `internal/cloud/magnum/` is the current worked example.

**Add a service:** see [Adding New Platform Services](adding-services.md) -- add the typed config in `internal/config/services/`, a plugin manifest in `internal/services/plugins/`, embedded templates under `internal/gitops/templates/cluster-apps-base/services/<service>/`, and a descriptor in `internal/services/descriptors/data/`.

**Add a validator:** create it under `internal/core/validation/validators/` and register it with the engine; see [Validation Rules](../reference/validation-rules.md).

## Dead-code and duplication cleanup history

A conservative first cleanup pass removed the unreferenced `internal/util/template` package (no importers -- active template rendering lives in `internal/gitops` and `internal/template`), trimmed `internal/util/files` to the atomic-write helper still used by `internal/sops` and `internal/util/crypto`, and collapsed duplicated `ConfigurationManager.Load` / `LoadWithoutValidation` logic into a shared private helper. `cmd/` dead-code findings were deliberately deferred because command registration affects public CLI behavior and generated reference docs; treat unused-looking exported functions in `cmd/` and in `internal/gitops` (e.g. the deprecated `shouldSkipFile` path) as intentionally retained until the descriptor-driven renderer cutover is formally approved (see [Renderer Contract](rendering-contract.md)), not as candidates for casual removal.
