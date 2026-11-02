---
last_updated: 2026-10-05
id: validation-rules
title: "Validation Rules"
sidebar_label: Validation Rules
description: Complete reference of configuration validation rules and constraints enforced by openCenter.
doc_type: reference
audience: "all users"
tags: [validation, rules, constraints, schema]
---
# Validation Rules

**Purpose:** For all users, documents what makes an `opencenter` cluster configuration invalid, and which subsystem enforces each rule.

`opencenter cluster validate` runs two independent layers on the loaded config. Both must pass for `cluster validate` to exit `0`; see [Exit Codes](exit-codes.md#general-command-contract) for how failures map to process exit codes.

## Layer 1 — schema and load-time checks

Every load (`internal/config/v2/loader.go`, `LoadFromBytes`) runs a fixed pipeline: **parse YAML → normalize → resolve `${ref:}`/`${env:}`/`${file:}` references → apply provider-region defaults → validate → freeze**. The "validate" stage here checks the decoded config against `schema/opencenter-v2.schema.json` (types, enums, required fields — see [Configuration Schema Reference](configuration-schema.md)) plus a handful of structural checks in `internal/config/v2/validator.go`, including detecting fields still left at their `cluster init` placeholder value (`v2.PlaceholderSecret = "CHANGEME"`, the SSH key placeholder, etc. — see [Default Values](default-values.md)).

## Layer 2 — deployment readiness (`v2.ValidateReadiness`)

`internal/cluster/validate_service.go` calls `v2.ValidateReadiness(cfg)` (`internal/config/v2/readiness.go`) for the actual cross-field, provider-aware business rules. This function is explicitly **offline** — it does not contact any cloud provider, Git remote, or Kubernetes API. It returns a `ReadinessReport{Valid, Issues[]}`; each `ValidationIssue` has a `Severity` (`error` or `warning` — only `error` flips `Valid` to `false`) and a `Category`:

| Category | Covers |
| --- | --- |
| `schema` | Structural rules that don't fit the JSON Schema (e.g. network plugin exclusivity — see below). |
| `provider` | Per-provider required fields and cross-checks. |
| `gitops` | GitOps repository/auth consistency. |
| `services` | Per-service secrets and scheduling-capacity checks. |
| `connectivity` | Reserved category; not populated by the current offline `ValidateReadiness` checks. |

`ValidateReadiness` runs seven checks, in this order: `validateProvider`, `validateNetworkPlugin`, `validateGitOps`, `validateServiceSchedulingCapacity`, `validateStorageProfile`, `validateServiceSecrets`, and `validateS3Buckets`.

### Provider rules

`validateProvider` dispatches on `opencenter.infrastructure.provider` (case-insensitively). Empty provider or an unrecognized value is an error ("Use one of: openstack, aws, gcp, azure, baremetal, vsphere, vmware, kind, magnum" — `vsphere` is accepted silently as a deprecated alias for `vmware`). Per-provider checks:

- **openstack** — `cloud.openstack` block required; `auth_url` required and must be an absolute URL (HTTP triggers a warning, not an error); `region`, `project_id`, `image_id` required and must not be placeholder values; `application_credential_id`/`application_credential_secret` must both be set or both be empty, and (for readiness purposes) both are actually required; `compute.flavor_master`/`flavor_worker`/`flavor_worker_windows`/`flavor_bastion` are required whenever the corresponding count is `> 0` or bastion is enabled; each `additional_server_pools_worker[]` entry with `count > 0` needs a flavor; having any of `cloud.aws`/`cloud.gcp`/`cloud.azure`/`cloud.vmware` populated alongside `cloud.openstack` is an error.
- **magnum** — `cloud.magnum` required; `auth_url` (HTTP(S) only, no embedded userinfo), `region`, `project_id`, `cluster_template` required; one of two authentication methods must be configured — either the application-credential pair (`application_credential_id`/`application_credential_secret`, both set together) or Keystone v3password (`username`/`password`, both set together). A config with neither method is rejected before any Keystone request. The v3password path exists for clouds that enforce Keystone trusts, which application credentials cannot mint. Other provider `cloud.*` sections must be absent. See `internal/cloud/magnum/provider.go` and [Adding New Infrastructure Providers](../contributing/adding-providers.md) — magnum is the newest provider and the closest current worked example of this rule shape.
- **baremetal**, **vmware**, **kind**, **aws**, **gcp**, **azure** — each has its own `validate<Provider>Provider` function in `internal/config/v2/readiness.go` with provider-specific required-field and cross-provider-section checks; read that file directly for the exact field list. AWS, GCP, and Azure readiness rules do not override the CLI availability gate that rejects those providers as planned.

### Network plugin rule

`validateNetworkPlugin` requires **exactly one** of `calico`/`cilium`/`kube-ovn` to have `enabled: true` — zero or more than one is an error (category `schema`). On the `openstack` provider only, the enabled plugin's `install_method` must be `helm` or `kustomize-helm`; `kubespray` (or anything else) is rejected. Other providers don't constrain `install_method`.

### GitOps rules

`validateGitOps` requires a non-empty, valid `https://` or `ssh://` `opencenter.gitops.repository.url`, and exactly one of `auth.ssh`/`auth.token` configured (both or neither is an error):

- **HTTPS repositories** require `auth.token` with a non-empty `token` or `token_file`, and `auth.token.provider` must match the repository host (`github.com` → `github`, `gitlab.com` → `gitlab`, anything else → `gitea`).
- **SSH repositories** require `auth.ssh.private_key` and `auth.ssh.public_key`, neither a placeholder.

The customer-managed overlay unit and its SOPS generation rules have their own separate validator — see [GitOps Configuration Reference](gitops-configuration.md#validation) for `overlay_units.customer_managed`/`overlay_units.sops` (enforced by `internal/gitops/overlay_units_validation.go`, not `ValidateReadiness`).

### Service rules

- `validateServiceSchedulingCapacity` — checks that enabling certain services is consistent with the cluster's node/compute counts (e.g. a service that needs scheduling capacity isn't enabled against a zero-worker cluster).
- `validateServiceSecrets` — checks per-service secret requirements when a service is enabled: `kube-prometheus-stack` webhook URL, `cert-manager`, `etcd-backup`, `loki`, `tempo`, `mimir`, and `harbor` secrets each have a dedicated check (`validateKubePrometheusStackWebhookURL`, `validateCertManagerSecrets`, `validateEtcdBackupSecrets`, `validateLokiSecrets`, `validateTempoSecrets`, `validateMimirSecrets`, `validateHarborSecrets`).

Full per-service field requirements are documented in [Platform Services](platform-services.md) and the [services reference](../reference/services/), not duplicated here.

## The generic validation engine — a separate, narrower framework

`internal/core/validation` (engine, registry, validators) is a reusable validator framework, but it is **not** the engine behind `cluster validate`'s business rules — `ValidateReadiness` above is a plain Go function with no dependency on it. The DI container (`internal/di/providers.go`) registers exactly four validators into the shared engine: `cluster-name`, `organization-name`, `config`, `file`, plus one always-on security validator (`security`) that every `Validate`/`ValidateAll`/`ValidateParallel` call runs first and cannot be bypassed. Concretely, `internal/cluster/init_service.go` calls `validationEngine.Validate(ctx, "cluster-name", name)` and `"organization-name"` during `cluster init`/rename flows to enforce Kubernetes-style naming (1–63 characters, lowercase alphanumeric and hyphens, must start/end alphanumeric).

Other validator types defined in `internal/core/validation/validators/` (`provider`, `gitops`, `network`, `opentofu`, `config-structure`) exist as reusable building blocks but are not registered into the DI container's engine as of this writing — don't assume they run during a normal `cluster validate`. Two other validators from that package **are** wired in elsewhere, outside the shared engine:

- `ServiceValidator` (`"service"`, parametrized per service name) is instantiated directly by `internal/services/registry.go` for each registered service, and by `internal/services/plugins/validators.go` for `cert-manager`/`keycloak`.
- `SOPSKeyValidator` (`"sops-key"`) is instantiated directly by `internal/sops/manager.go` to validate age key file existence/format (must start with `AGE-SECRET-KEY-`) before SOPS operations.

Validators run in priority order when multiple are requested together (`PriorityHigh = 50` first, then `PriorityNormal = 100`, then `PriorityLow = 200`); see `internal/core/validation/types.go`.

## Related

- [Configuration Schema Reference](configuration-schema.md) — the schema Layer 1 checks against.
- [Default Values Reference](default-values.md) — placeholder values Layer 1 flags as not-yet-configured.
- [GitOps Configuration Reference](gitops-configuration.md) — full `opencenter.gitops.*` field rules, including the overlay-unit validator.
- [Exit Codes](exit-codes.md) — how a validation failure surfaces as a process exit code.
- [Cluster Validate Execution Flow](../contributing/validation.md) — the contributor-facing walkthrough of this same code path.
