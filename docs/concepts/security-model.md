---
last_updated: 2026-09-26
id: security-model
title: "Security Model"
sidebar_label: Security Model
description: Defense-in-depth security controls across the CLI, generated infrastructure, and the deployed platform.
doc_type: explanation
audience: "security engineers, architects"
tags: [security, sops, encryption, rbac, kyverno]
---
# Security Model

**Purpose:** For security engineers, explains the security controls openCenter applies at the CLI layer, in generated infrastructure/cluster manifests, and in the secrets lifecycle.

Two of these layers are directly implemented in this repository and independently verifiable: the CLI's own input/command/audit controls (`internal/security`) and the secrets/key lifecycle (`internal/secrets`, `internal/sops`). The cluster-level and platform-service layers (Pod Security Admission, Kyverno, NetworkPolicies, RBAC) are generated into the deployed cluster from templates and the separate `openCenter-gitops-base` repository; this document describes what this CLI's templates and defaults configure, not the full policy content of that external repository.

## Layered Model

```
CLI layer            InputValidator, CommandSanitizer, CredentialMasker, AuditLogger
                          (internal/security)
Secrets layer         SOPS/Age encryption in Git, key registry, rotation, revocation
                          (internal/sops, internal/secrets)
Cluster layer         Pod Security Admission, audit logging, encryption at rest
                          (generated via internal/gitops infrastructure templates)
Platform layer        Kyverno policies, NetworkPolicies, RBAC Manager, Keycloak OIDC
                          (deployed from openCenter-gitops-base, configured by this CLI)
Network layer         Gateway API and CNI-configured NetworkPolicies
```

**Why this design:** each layer has an independent control and independent failure mode. A defect in one (a missing NetworkPolicy, a stale audit key) does not by itself defeat the others.

## CLI-Layer Controls

**Entry point: CLI arguments and configuration files.**

* `internal/security.InputValidator` validates user-controlled identifiers and values before they reach filesystem or provider calls.
* `internal/security.CommandSanitizer` builds external commands (git, provider CLIs) through an allowlist-based sanitizer rather than raw shell interpolation; it also validates editor invocations (`ValidateEditor`) and sanitizes Git argument lists (`SanitizeGitArgs`).
* `internal/security.CredentialMasker` strips sensitive values from logs, error output, and diagnostics.
* Configuration itself passes through the six-stage `internal/config/v2` pipeline (schema, then business-rule, provider, and deployment validation) before any command acts on it — see [Configuration Lifecycle](configuration-lifecycle.md).

**Evidence:** `internal/security/input_validator.go`, `internal/security/command_sanitizer.go`, `internal/security/credential_masker.go`, `internal/config/v2/loader.go`

### CLI Audit Logging

`internal/security.AuditLogger` signs every audit event with HMAC-SHA256 using a 32-byte random key generated on first use and stored at `~/.config/opencenter/audit/audit.key` (`internal/security/audit_logger.go`). The signature covers the event's timestamp, type, actor, resource, action, and result, so a post-hoc edit of a log entry invalidates its signature. Logs rotate at 100 MB and are retained for 30 days by default (`MaxLogSize`, `LogRetentionDays` in `internal/security/audit_logger.go`). See [Audit Signing Key](../reference/audit-key.md) for the key format and rotation guidance.

Logged operations include cluster initialization, configuration changes, secret operations (generation, rotation, revocation, decryption), deployment actions, drift detection, and input-validation failures.

## Secrets Layer

### SOPS Age Encryption

Secrets are encrypted with SOPS/Age before they are written into the GitOps tree; FluxCD decrypts them during reconciliation using an Age key stored as a Kubernetes Secret in the cluster. See [GitOps Workflow](gitops-workflow.md) for the reconciliation side and [Manage Secrets](../operations/manage-secrets.md) for operational commands.

**Evidence:** `internal/sops/manager.go`, `internal/sops/overlay_files.go`

### Key Lifecycle

`internal/secrets.DefaultKeyRegistry` enforces uniqueness per `(cluster, key type, fingerprint)` across every status, so an archived or revoked fingerprint cannot be silently reinstated (`internal/secrets/registry.go`). A cluster may have any number of active Age recipients; SOPS encrypts to all of them. At most one active entry per cluster and key type is the **primary** — the key that rotation replaces — selected explicitly via `SetPrimaryKey` (also exposed as `opencenter secrets keys set-primary`). Registration never auto-promotes a key to primary.

Keys move from `active` to `archived` when replaced by ordinary rotation, or to `revoked` when explicitly distrusted (`KeyStatus` in `internal/secrets/interfaces.go`); only `active` keys are SOPS recipients.

**Default expiration periods** (`internal/secrets/registry.go`):

| Key type | Default expiration |
| --- | --- |
| Age | 90 days |
| SSH | 180 days |

**Rotation** (`internal/secrets/rotation.go`):

```bash
opencenter secrets keys generate
opencenter secrets keys rotate --cluster my-cluster --type age
opencenter secrets keys revoke --cluster my-cluster --key <fingerprint>
```

Age rotation runs in dual-key mode (`DualKeyActive: true`): the new key is added to `.sops.yaml` alongside the old one so existing ciphertext stays decryptable, then `CompleteRotation` archives the predecessor once re-encryption is done. SSH rotation is immediate — no dual-key period. Rotation state is derived from an active successor explicitly naming an active predecessor, not merely from the count of active keys.

**Evidence:** `internal/secrets/registry.go`, `internal/secrets/rotation.go`, `internal/secrets/interfaces.go`, `internal/secrets/reconcile.go`

## Cluster and Platform Layers

These controls are generated by this CLI's infrastructure templates (`internal/gitops/templates/`) or deployed from the separate `openCenter-gitops-base` repository, which is outside this codebase; specific counts (for example, the exact number of default Kyverno `ClusterPolicy` resources) live in that repository and are not verified here.

* **Pod Security Admission:** the generated Terraform/Ansible inputs set `kube_pod_security_exemptions_namespaces`, defaulting to `["trivy-temp"]` when `opencenter.cluster.kubernetes.security.pod_security_exemptions` is not set (`internal/gitops/templates/infrastructure-cluster-template/main-default.tf.tpl` and the VMware/bare-metal variants).
* **Kyverno:** deployed as a platform service; policy content comes from `openCenter-gitops-base`, not this repository.
* **RBAC Manager / Keycloak OIDC:** Keycloak is configured through `opencenter.identity` and `services.keycloak`; RBAC Manager converts Keycloak groups into `RBACDefinition` resources reconciled by Flux.
* **Network:** Gateway API is the configured ingress model, while Calico,
  Cilium, or Kube-OVN provide the CNI-level NetworkPolicy setting. This
  repository does not provide an Istio installation path, so an Istio mTLS
  deployment must not be inferred from this CLI.

## Supply Chain

* Go module dependencies are pinned via `go.mod`/`go.sum` (current toolchain: see `go.mod`).
* Release builds generate an SPDX SBOM with Syft (`.github/workflows/release.yml`, "Generate SBOM" step) — this is automated in CI, not a manual or planned step.

## Common Misconceptions

### "SOPS encryption is optional"

**Reality:** the generated GitOps repository encrypts secret manifests with SOPS by default; there is no supported path that writes plaintext secrets into the Git-tracked tree.

### "Pod Security Admission replaces Kyverno"

**Reality:** they are independent layers with different enforcement points — Pod Security Admission is namespace-level and cluster-native; Kyverno is resource-level and policy-engine-based.

### "Secrets are decrypted in Git"

**Reality:** secrets stay encrypted in Git. FluxCD decrypts in-memory during reconciliation.

## Further Reading

* [Architecture](../architecture.md) — system design and components
* [GitOps Workflow](gitops-workflow.md) — repository structure and reconciliation
* [Secret and Config Separation](security-update-design.md) — how secrets, state, and the GitOps tree are kept in separate filesystem zones
* [Manage Secrets](../operations/manage-secrets.md) — SOPS and secrets management
* [Audit Signing Key](../reference/audit-key.md) — audit log signature format

## Evidence

* CLI-layer controls: `internal/security/input_validator.go`, `command_sanitizer.go`, `credential_masker.go`, `audit_logger.go`
* Secrets layer: `internal/secrets/registry.go`, `rotation.go`, `interfaces.go`, `reconcile.go`, `internal/sops/manager.go`
* Generated cluster/platform layer: `internal/gitops/templates/infrastructure-cluster-template/`
* Supply chain: `go.mod`, `.github/workflows/release.yml`
