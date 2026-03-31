---
id: services-rendering-parity-plan
title: "Service Rendering: Fixture Parity Plan"
sidebar_label: Rendering Parity Plan
description: Defines the contract, trust boundaries, dependencies, and gated work required for five distinct per-cluster config files to reproduce the RelayPoint overlay fixtures.
doc_type: explanation
audience: "developers, platform engineers"
tags: [services, rendering, overlays, fixtures, testing]
---

# Service Rendering: Fixture Parity Plan

**Purpose:** Define the contract, trust boundaries, missing decisions, and gated implementation work required for openCenter to render RelayPoint-style cluster overlays from five distinct per-cluster config files.

## Current Status

As of 2026-03-31, the descriptor-based parity implementation exists in the repository:

- Cluster-app rendering now plans output from dedicated overlay descriptors in `internal/services/descriptors/` and executes that plan through `internal/gitops/descriptor_renderer.go`.
- Shared overlay-unit types exist under `internal/config/overlay/` and are wired into both the active config model and `internal/config/v2/`.
- Checked-in RelayPoint fixture configs exist at `testdata/relaypoint-logistics-shared/.k8s-{dev,dr,prod,qa,uat}-config.yaml`.
- Missing embedded service templates for `harbor`, `kafka-cluster`, and `mimir` have been added.
- `RenderClusterApps`, `RenderClusterAppsAtomic`, and `RenderSingleService` use descriptor planning instead of the legacy convention-based filesystem walker.
- Descriptor coverage validation runs before rendering and fails when renderer-owned embedded files are missing descriptor ownership or have duplicate owners.
- The parity harness exists in `internal/gitops/relaypoint_parity_test.go` and uses the machine-readable canonicalization inventory at `testdata/relaypoint-logistics-shared/parity-canonicalization.yaml`.
- Fixture secret scanning exists in test coverage and rejects literal or base64-decoded private-key markers in the RelayPoint fixtures.

### Immediate Security Issue: Committed Secret Material

**Status: mitigated in fixtures; still relevant as a repository policy concern.**

The UAT customer-managed Secret fixture has been sanitized to obvious placeholder content. Repository tests now reject literal or decoded private-key markers in `testdata/relaypoint-logistics-shared/`.

Required standing policy:

1. Fixtures must contain only placeholders or clearly synthetic secret material.
2. Any real credential found in fixture history must still be treated as compromised and rotated out of band.
3. The fixture scanner must remain in CI to block regressions.

## Scope And Non-Goals

This document is in scope for:

- descriptor-driven rendering of overlay topology
- typed cluster-scoped config needed to reproduce the RelayPoint fixture
- parity fixtures and validation rules for renderer-owned output
- lifecycle boundaries between renderer-owned and bootstrap-owned files
- security, validation, and rollback requirements needed before cutover
- the rendering model inversion from negative-list to positive-list and its migration risks

This document is not the place to:

- replace typed Go config with a second full configuration system
- move complex validation or secret fallback logic out of Go
- define the implementation details of `flux bootstrap`
- approve committing live secret material into repository fixtures
- claim that the fixture is already a complete and canonical config-to-output contract

## Problem

The rendering architecture in the options document correctly moves the codebase toward typed config plus descriptor-driven rendering, but that document alone is not sufficient to guarantee exact parity with the RelayPoint fixture in `testdata/relaypoint-logistics-shared/applications/overlays/`.

The target contract must be explicit:

- one `.<cluster>-config.yaml` renders one `applications/overlays/<cluster>/` tree
- the RelayPoint fixture therefore requires five distinct config inputs: `.k8s-dev-config.yaml`, `.k8s-dr-config.yaml`, `.k8s-prod-config.yaml`, `.k8s-qa-config.yaml`, `.k8s-uat-config.yaml`
- rendering those five files independently should reproduce the renderer-owned portion of the five overlay trees, subject only to approved canonicalization rules

Today, the repository does not yet have those five config fixtures, and the proposed descriptor model still lacks capabilities and approval-relevant detail needed for parity, security review, and safe cutover.

## Assumptions And Required Decisions

The following architecture decisions are now treated as the implementation contract for parity work:

- The RelayPoint fixture remains the parity oracle, but only through the explicit canonicalization inventory in `testdata/relaypoint-logistics-shared/parity-canonicalization.yaml`.
- Rendering metadata lives in dedicated overlay descriptors. `ServicePluginManifest` is not the topology authority for cluster-app rendering.
- `config.Config` remains the active renderer input. v2 reuses the same overlay-unit contract in parallel.
- `flux-system/` remains bootstrap-owned and is excluded from parity comparisons.
- Cutover is fail-forward. There is no runtime fallback to the legacy renderer; regressions are handled by fix-forward or git revert.
- Customer-managed repository inputs and emitted secrets are trust-bearing fields and must pass typed validation before render.

### Ownership Assignment Requirement

Named owners must be assigned before Phase 1 can begin. This is a prerequisite to Phase 1, not part of it. The following roles require named individuals:

| Role | Responsibility | Status |
|---|---|---|
| Architecture owner | Approves renderer-owned vs. bootstrap-owned boundaries, descriptor schema, rendering model inversion | **TBD — must be assigned** |
| Security owner | Approves secret handling model, fixture secret policy, `.sops.yaml` generation, repository trust policy, redaction requirements | **TBD — must be assigned** |
| Parity oracle owner | Approves canonicalization inventory, fixture validity, test oracle authority | **TBD — must be assigned** |
| Cutover owner | Approves cutover readiness, rollback criteria, operational acceptance | **TBD — must be assigned** |

## Fixture Inventory

The fixture at `testdata/relaypoint-logistics-shared/applications/overlays/` contains five cluster overlay trees. Their structures are not identical. The differences are part of the parity problem and must be explained by per-cluster config plus explicit lifecycle rules.

### Per-cluster structural variance

| Feature | k8s-dev | k8s-dr | k8s-prod | k8s-qa | k8s-uat |
|---|---|---|---|---|---|
| `flux-system/` directory | absent | present | present | present | absent |
| `.sops.yaml` at overlay root | present | present | absent | absent | absent |
| `customer-managed/` directory | present | present | absent | present | present |
| `customer-managed/` Secret manifest | absent | absent | n/a | absent | present (contains private key material) |

All five root `kustomization.yaml` files reference `./flux-system` regardless of whether the directory exists. k8s-dev and k8s-uat are therefore invalid Kustomize overlays in their checked-in state. This is evidence of mixed lifecycle state in the fixture, not proof that openCenter may safely claim full standalone overlay validity before bootstrap.

### Embedded template vs. fixture service inventory

The embedded template filesystem at `internal/gitops/templates/cluster-apps-base/services/` and the fixture overlay trees do not contain the same service set. This gap must be closed before parity is achievable.

| Service | Embedded templates | k8s-dev fixture | k8s-prod fixture | k8s-qa fixture | Notes |
|---|---|---|---|---|---|
| calico | yes | yes | yes | yes | |
| cert-manager | yes | yes | yes | yes | |
| etcd-backup | yes | yes | no | no | dev-only in fixture |
| fluxcd | yes | yes | yes | yes | structural, not a service |
| gateway | yes | yes | yes | yes | |
| gateway-api | yes | yes | yes | yes | |
| harbor | **no** | yes | no | no | missing from embedded templates |
| headlamp | yes | yes | yes | yes | |
| kafka-cluster | **no** | yes | yes | yes | missing from embedded templates |
| keycloak | yes | yes | yes | yes | |
| kube-prometheus-stack | yes | yes | yes | yes | |
| kyverno | yes | yes | no | yes | absent from k8s-prod |
| loki | yes | yes | yes | yes | |
| longhorn | yes | no | no | yes | absent from k8s-dev, k8s-prod |
| metallb | yes | yes | yes | yes | |
| mimir | **no** | yes | yes | yes | missing from embedded templates |
| olm | yes | yes | yes | yes | |
| openstack-ccm | yes | no | no | no | provider-specific, not in fixture |
| openstack-csi | yes | no | no | no | provider-specific, not in fixture |
| opentelemetry-kube-stack | yes | yes | yes | yes | |
| postgres-operator | yes | yes | yes | yes | |
| sealed-secrets | yes | no | no | yes | qa-only in fixture |
| tempo | yes | yes | yes | yes | |
| velero | yes | yes | yes | yes | |
| vsphere-csi | yes | yes | yes | yes | |

Three services in the fixture (`harbor`, `kafka-cluster`, `mimir`) have no corresponding embedded templates. Parity is impossible for those services without adding templates or changing the rendering model to support non-embedded sources.

### Per-cluster service variance

Services present in the fixture vary by cluster. Notable differences include:

- `harbor` and `etcd-backup` appear only in `k8s-dev`
- `sealed-secrets` appears only in `k8s-qa`
- `kyverno` and `longhorn` appear in `k8s-dev`, `k8s-dr`, `k8s-qa`, `k8s-uat` but not `k8s-prod`
- `kafka-cluster`, `mimir`, `olm`, `opentelemetry-kube-stack`, `postgres-operator` appear across most clusters

### Customer-managed variance

The `customer-managed/` layer is present in four of five clusters and absent from `k8s-prod`. The four present clusters share the same logical repository reference and the same three Flux Kustomizations (`policies`, `infrastructure`, `apps`), but use cluster-specific paths.

Only `k8s-uat` includes a rendered Secret manifest in `customer-managed/sources/`. That Secret contains base64-encoded private key material (see Immediate Security Issue above). The other clusters reference the same GitRepository but do not emit a Secret. Parity requires a typed and policy-constrained way to express cluster-scoped repository credentials or approved references to them.

### Root kustomization template gap

The embedded root `kustomization.yaml` template at `internal/gitops/templates/cluster-apps-base/kustomization.yaml` references `./flux-system`, `./services/fluxcd`, and `./managed-services/fluxcd`. It does not reference `./customer-managed/fluxcd`. The k8s-dev fixture root `kustomization.yaml` references all four, including `./customer-managed/fluxcd`. This means the current renderer cannot produce the fixture root kustomization without modification.

## Architecture, Boundaries, And Constraints

### 1. Source-of-truth boundaries

- Typed Go config remains the source of truth for configuration shape, defaults, and complex validation.
- Overlay unit descriptors become the source of truth for rendered topology only.
- Parity fixtures are regression oracles only after the canonicalization inventory is approved. Until then, fixture drift is evidence to resolve, not silent permission to normalize output.

### 2. Renderer-owned versus bootstrap-owned paths

Under the current recommendation:

- openCenter renders renderer-owned paths such as root `kustomization.yaml`, `services/`, `managed-services/`, `customer-managed/`, `.sops.yaml`, and aggregate `kustomization.yaml` files within each branch.
- `flux-system/*` remains bootstrap-owned and is outside the descriptor renderer.

This boundary resolves template ownership only partially. It does not yet answer whether a root overlay that references `./flux-system` is considered valid before bootstrap. This document therefore distinguishes two lifecycle states:

- **Renderer parity state:** renderer-owned paths are present and compared by parity tests. `flux-system/` is excluded from comparison.
- **Bootstrap-complete state:** bootstrap-owned paths exist and the full overlay is expected to be consumable as deployed.

This plan defines renderer parity requirements. It does **not** approve a claim about bootstrap-complete validity for clusters that lack checked-in `flux-system/`. That remains an open issue requiring a decision from the architecture owner.

### 3. Rendering model inversion: negative-list to positive-list

The current renderer (`RenderClusterAppsAtomic`) walks the embedded filesystem `internal/gitops/templates/cluster-apps-base/` and copies everything except files matched by `shouldSkipFile`. This is a negative-list model: new files in the embedded FS are included automatically.

The proposed descriptor-driven renderer is a positive-list model: only files declared in a descriptor are rendered. New files added to the embedded FS but not declared in a descriptor will be silently excluded.

This inversion changes the failure mode:

- **Current (negative-list):** failure mode is "too much output" — undeclared files appear in rendered output.
- **Proposed (positive-list):** failure mode is "missing output" — undeclared files silently disappear.

The positive-list failure mode is more dangerous for security-relevant files (Kyverno policies, NetworkPolicies). A missing policy degrades cluster security silently.

**Required mitigation (must be part of Phase 1 contract):**

- The renderer must include a validation step that compares the embedded FS file list against declared descriptor files and fails on undeclared entries.
- The validation must run in CI and block merges that add embedded template files without corresponding descriptor entries.
- The Phase 1 contract must explicitly document this inversion and its failure modes.

### 4. Descriptor semantics must stay bounded

Descriptors are intended to answer rendering questions only. To keep them smaller than the code they replace:

- conditions are limited to simple checks against typed config values such as boolean, presence, and equality tests
- descriptors do not own secret fallback chains, external lookups, arbitrary evaluation, or complex validation
- if descriptor needs exceed those bounds, the design requires a separate review before scope expands

This constraint is intentional. It prevents the descriptor layer from becoming a hidden rules engine.

### 5. Security-sensitive inputs are trust-bearing

The following fields are not routine metadata:

- customer-managed repository URLs and branches
- SSH trust material such as `known_hosts`
- cluster-scoped credential inputs or references
- `.sops.yaml` recipients and path regexes (these control encryption scope and are security-sensitive, not just rendering concerns)

Any typed surface for those values must include validation, policy, and review expectations. This document does not treat those fields as ordinary fixture data.

### 6. ServicePluginManifest integration

`ServicePluginManifest` is not the rendering topology source for this work. Overlay descriptors explicitly supersede it for cluster-app ownership, aggregate planning, and conditional output selection.

## Known Gaps That Must Be Addressed

### 1. Static file membership is not enough

Several services render different file sets in different clusters. A descriptor with a single static `overlay.files` list cannot express that.

Examples from the fixture include:

- cert-manager rendering different issuer and secret files across clusters
- keycloak rendering `patch-subscription.yaml` only in some clusters
- kube-prometheus-stack rendering `alertmanager-routes.yaml` only in some clusters
- velero and vsphere-csi rendering extra files in specific clusters

### 2. Cluster-scoped assets are not fully modeled

The renderer must handle assets that are not naturally part of `services:` or `managed-service:`:

- root-level `kustomization.yaml` (currently missing `./customer-managed/fluxcd` reference in the embedded template)
- `.sops.yaml`
- `customer-managed/` sources and Flux units
- customer-managed Secret manifests

Those need typed inputs, ownership rules, and security constraints that are separate from service enablement.

### 3. The `flux-system/` lifecycle boundary is unresolved

All five root `kustomization.yaml` files reference `./flux-system`, but checked-in `flux-system/` files exist only for `k8s-dr`, `k8s-prod`, and `k8s-qa`. k8s-dev and k8s-uat are invalid Kustomize overlays in their checked-in state.

Before parity can be approved, the architecture owner must decide:

- whether pre-bootstrap overlays are a supported output state
- how root `kustomization.yaml` is validated in that state
- what evidence proves a bootstrap-complete overlay is consumable

### 4. The fixture is not yet backed by complete config inputs

Some overlay content appears to exist without an obvious matching config declaration. The fixture is not currently a strict "config in, rendered output out" artifact.

Before parity can be approved, the repo needs five complete cluster config fixtures that include:

- every enabled service the renderer is expected to manage
- cluster-scoped overlay-unit inputs
- approved secret references or approved secure secret inputs (not live credentials)
- `.sops.yaml` generation inputs

### 5. Canonicalization is not defined tightly enough

Historical drift exists in the fixture. That does not justify open-ended "semantic parity."

Canonicalization must be:

- finite
- versioned
- path-specific
- justified with rationale
- approved by the parity oracle owner

Without that, parity tests can mask regressions.

### 6. The security model is not yet defined

This plan introduces security-sensitive config and secret surfaces, but the following are still undefined:

- secret source of truth (references, out-of-band injection, or another approved mechanism)
- fixture secret policy (fixtures must contain clearly synthetic placeholders, never real or real-looking credentials)
- repository allowlisting and transport policy for customer-managed Git sources
- host key validation requirements
- CI and logging redaction requirements (logs, parity failures, and diagnostics must not print secret values)
- auditability and rotation expectations

### 7. The operational model is not yet defined

Refactoring the renderer changes a control-plane path that produces GitOps output. The document still needs:

- a concrete rollback mechanism (see Rollback And Fallback section)
- diagnostic output format
- named operational ownership for cutover readiness

### 8. The hardcoded service list is incomplete

`inferServices` in `internal/template/embedded_registry.go` lists 13 services. The fixture contains at least 22 distinct service directories. The embedded template filesystem contains 22 service directories (excluding `sources/`), but three services in the fixture (`harbor`, `kafka-cluster`, `mimir`) have no embedded templates. Any descriptor-based approach must cover the full set and account for the template gap.

### 9. The v2 config model has no overlay unit types

The v2 config model (`internal/config/v2/config.go`) contains `GitOpsConfig` with basic Git URL/branch/release fields but has no `OverlayUnit`, `CustomerManaged`, `SOPSGenerationConfig`, or conditional rendering types. `ServiceMap` is `map[string]any`, which provides no type safety.

Phases 2, 3, 4, and 5 all depend on typed config surfaces that do not exist. The v2 config types needed by this plan must be designed and frozen (at least as interfaces) before Phase 2 or Phase 3 can start. This is a hard dependency.

**Decision required:** the v2 config model must have a stabilization commitment (timeline or interface freeze) for the types this plan depends on before Phase 2 begins.

## Proposed Direction

Keep the main recommendation from the options document:

- typed Go config remains the source of truth for config shape, defaults, and validation
- overlay unit descriptors become the source of truth for rendered topology

Then add the following parity-specific requirements.

### 1. Add conditional rendering with bounded semantics

Descriptor entries need conditions so a single logical unit can vary by cluster without forking the descriptor.

The model should support conditions on:

- overlay files
- generated source manifests
- generated Flux manifests
- aggregate inclusion

Conditions are intentionally limited to simple field-based checks.

**Phase 3 must produce a formal, testable schema for descriptor conditions.** The schema must fit on one page and define:

- allowed operators (equality, presence, boolean — no nesting, no logical combinators beyond these)
- valid field path syntax and resolution rules against typed config
- error handling for invalid conditions (fail-closed: invalid condition = render error, not silent skip)
- explicit extension review process: any operator or capability added beyond the initial set requires architecture owner approval

Illustrative example (not approved schema):

```yaml
overlay:
  files:
    - 10-operator/kustomization.yaml
    - name: 10-operator/patch-subscription.yaml.tpl
      when:
        field: services.keycloak.pin_operator_version
        is: true
```

The full condition space across all 22+ services has not been analyzed. Before Phase 3 is complete, the team must inventory the actual per-service file variance across all five clusters and confirm that the bounded condition model can express it. If it cannot, that is a design change requiring separate review.

### 2. Add typed cluster-scoped config for non-service units

`.<cluster>-config.yaml` needs a typed surface for overlay units that are not service-owned.

Illustrative shape only (not approved schema — exact field names belong in the typed config model):

```yaml
opencenter:
  gitops:
    overlay_units:
      customer_managed:
        enabled: true
        repository_name: customer-repository
        repository_url: ssh://<customer>@<git-host>/<org>/<repo>.git
        branch: main
        kustomizations:
          - name: policies
            path: /policies/qa
          - name: infrastructure
            path: /infrastructure/qa
          - name: apps
            path: /apps/qa
      sops:
        enabled: true
        age_recipients:
          - age1...
        path_regexes:
          - "^managed-services/.*/helm-values/.*\\.ya?ml$"
```

The illustrative schema must handle the variance between k8s-uat (emits a Secret manifest) and k8s-qa (no Secret manifest) for the same logical `customer_managed` block. This requires either a boolean flag controlling Secret emission or a separate typed secret surface. The exact mechanism is a decision for Phase 2.

### 3. Add typed handling for cluster-scoped secret inputs

Customer-managed GitRepository Secrets and similar assets need typed handling just like service-scoped secrets do.

Two constraints apply:

- this document does not approve committing live secret material into repository fixtures
- the plan must define whether the renderer consumes placeholders, secret references, or another approved secure input mechanism

Illustrative shape only:

```yaml
secrets:
  customer_managed:
    customer_repository:
      identity: "<provided out of band>"
      identity_pub: "<provided out of band>"
      known_hosts: "<approved host key entry>"
```

The exact secure input mechanism is a decision required from the security owner before Phase 2 is complete.

### 4. Treat `flux-system/` as a separate lifecycle concern

The current recommendation is:

- openCenter renders the root `kustomization.yaml`
- `flux-system/*` remains bootstrap-owned and outside the descriptor renderer
- parity tests compare renderer-owned paths directly
- bootstrap-owned paths are validated separately as part of bootstrap-complete state

This keeps template ownership clear, but it does **not** eliminate the need for a lifecycle decision. Approval remains blocked until the architecture owner defines how root overlays that reference `./flux-system` are validated in supported states.

### 5. Define canonical output rules before using them in tests

Parity must be exact for renderer-owned paths unless an approved canonicalization rule says otherwise.

Each canonicalization rule must state:

- affected path or file pattern
- exact normalization being applied
- why the existing fixture is not authoritative for that case
- who approved the rule (must be the parity oracle owner)

Open-ended "semantic parity" is not sufficient.

### 6. Add validation and diagnostics as part of the design

Parity tests alone are not enough. The design must also support:

- negative tests for invalid config and unsupported descriptor conditions
- diagnostics that identify which descriptor or field caused an inclusion or exclusion decision
- validation of single-service rendering without scanning unrelated files
- explicit treatment of lifecycle-state claims for clusters with and without checked-in `flux-system/`
- undeclared-file detection: any file in the embedded FS that has no descriptor must cause a validation failure

## Security Considerations

### Fixture secret policy

Repository fixtures must not contain live customer-managed credentials or material that is indistinguishable from real credentials. The k8s-uat fixture currently violates this (see Immediate Security Issue above). Fixtures must use clearly synthetic placeholders that cannot be mistaken for real keys.

### Secret handling model

The project must decide whether cluster-scoped secret data is supplied by references, out-of-band injection, or another approved mechanism before production use is approved. This decision belongs to the security owner and must be made in Phase 2.

### `.sops.yaml` generation is security-sensitive

`.sops.yaml` controls encryption scope. Generating it from config means the renderer influences what gets encrypted. This is a security-sensitive code path, not just a rendering concern. The security owner must review and approve the `.sops.yaml` generation design before it is implemented.

### Repository trust policy

Repository URL, branch, transport, and host verification are trust-bearing values and require policy. This plan does not yet define that policy. The security owner must define:

- allowed transport protocols (SSH only, or also HTTPS)
- repository URL allowlisting requirements
- host key validation requirements for SSH
- branch protection expectations

### Redaction requirements

Logs, parity failures, and diagnostics must not print secret values. The exact redaction mechanism is TBD but the requirement is not optional. CI pipelines must be audited for secret leakage paths.

### Audit trail

Any change to security-sensitive config fields (`.sops.yaml` recipients, repository URLs, secret references) must be auditable. The mechanism (Git history, structured logging, or external audit) is TBD.

## Rollback And Fallback

This refactor changes a control-plane path that produces GitOps output. A rendering bug in production could produce incorrect overlay output for a customer cluster. The rollback mechanism must be faster than "revert the PR and rebuild."

### Required rollback mechanism

**Decision required before Phase 4:** the fallback mechanism must be one of:

- **Runtime feature flag:** a config or environment variable that switches between the convention-based renderer and the descriptor-driven renderer at invocation time. This is the recommended approach because it allows per-cluster rollback without code changes.
- **Build tag:** a Go build tag that selects the renderer at compile time. Faster than a PR revert but requires a rebuild.
- **Config option:** a field in the cluster config that selects the rendering mode. Allows per-cluster control but adds config surface.

Whichever mechanism is chosen:

- it must be testable independently (both paths must have CI coverage)
- the convention-based renderer must remain functional and tested until Phase 7 cutover is approved
- the cutover owner must define the criteria for removing the fallback path

### Rollback criteria

The cutover owner must define explicit conditions under which the team reverts to the convention-based renderer. At minimum:

- any renderer-owned file differs from expected output in a way not covered by an approved canonicalization rule
- any security-relevant file (Kyverno policy, NetworkPolicy, `.sops.yaml`) is missing from rendered output
- any customer cluster reports a reconciliation failure attributable to rendered output

## Operational Considerations

### Observability and diagnostics

The renderer needs enough diagnostic output to explain:

- which descriptor was selected for each rendered unit
- why a conditional file or generated unit was included or excluded
- which lifecycle state is being validated
- which files in the embedded FS have no descriptor (undeclared-file detection)

The exact output format is TBD, but the requirement is not optional. Diagnostics must be structured (JSON or similar) to support automated analysis.

### Support model

Named operational ownership is currently missing. The cutover owner role (see Ownership Assignment Requirement) must be filled before Phase 4 begins.

### Backup and restore

This document does not change repository backup or restore posture. Recovery of generated output remains outside the scope of the renderer design.

## Risks

| Risk | Probability | Impact | Mitigation |
|---|---|---|---|
| Live secret material in the fixture is a real key | Unknown | High | Immediate triage and rotation. Fixture sanitization. Pre-commit hook. |
| Implementation begins before contracts are frozen, causing cascading rework | High | High | Assign owners. Enforce Phase 1 completion as a merge-blocking prerequisite. |
| Descriptor condition model grows beyond bounded semantics under real-world pressure | Medium | High | Formal condition schema with explicit extension review. Hard limit on complexity. |
| Parity tests pass against invalid fixture, creating false confidence | High | Medium | Canonicalization inventory must be approved before Phase 6. Invalid states must be documented. |
| Negative-to-positive-list rendering inversion silently drops security-relevant files | Medium | High | Undeclared-file validation in CI. Fail on any embedded FS file without a descriptor. |
| v2 config model changes under this plan, causing rework | High | Medium | v2 stabilization commitment before Phase 2. Interface freeze for overlay unit types. |
| Three services in fixture have no embedded templates, making parity impossible | Already realized | Medium | Add templates for `harbor`, `kafka-cluster`, `mimir` or change rendering model. |
| RelayPoint fixture is not representative of other customer patterns | Medium | Medium | Compare against at least two other customer repositories before Phase 5. |
| `ServicePluginManifest` and overlay descriptors become overlapping metadata sources | Medium | Medium | Architecture owner decides relationship in Phase 1. |

## Implementation Plan

### Prerequisites (must be completed before Phase 1)

1. **Triage the committed private key** in `testdata/relaypoint-logistics-shared/applications/overlays/k8s-uat/customer-managed/sources/customer-repository-rpl-apps-flux-k8s-secret.yaml`. Rotate if live. Replace with synthetic placeholders regardless.
2. **Assign named owners** for architecture, security, parity oracle, and cutover (see Ownership Assignment Requirement table).
3. **Add a pre-commit hook or CI check** that detects base64-encoded private key patterns in fixture files.

### Phase 1: Freeze contract and approval boundaries

Document and approve:

- renderer-owned versus bootstrap-owned paths (explicit path list, not just categories)
- supported lifecycle states and what claims may be made in each state
- the rendering model inversion (negative-list to positive-list), its failure modes, and the undeclared-file validation requirement
- the relationship between overlay unit descriptors and `ServicePluginManifest`
- canonicalization inventory format and approval authority
- security review inputs required before config and descriptor work proceeds
- the rollback mechanism (feature flag, build tag, or config option)

**Deliverable:** a standalone, reviewable contract document approved by the architecture owner.

No implementation work in later phases should be treated as stable before this phase is approved.

Status: not started.

### Phase 2: Freeze the security-sensitive config surface

Define the typed config and secret surface needed for:

- customer-managed repository definitions (including the k8s-uat Secret vs. k8s-qa no-Secret variance)
- `.sops.yaml` generation inputs (with security owner review of encryption scope implications)
- cluster-scoped secret handling for non-service units
- fixture secret policy (synthetic placeholders only)

**Hard dependency:** v2 config model must have a stabilization commitment for overlay unit types before this phase begins.

This phase must also produce approved decisions for:

- secret source of truth (references, out-of-band injection, or other)
- repository allowlisting and host verification policy
- whether fixture files contain placeholders, references, or another approved representation
- redaction requirements for CI and logging

**Deliverable:** typed Go interfaces for overlay unit config and secret config, approved by the security owner.

Status: not started.

### Phase 3: Freeze the descriptor schema with bounded semantics

Produce a formal, testable schema for overlay unit descriptors. The schema must define:

- conditional `overlay.files` with explicit operator set (equality, presence, boolean)
- conditional generated sources
- conditional generated Flux units
- cluster-scoped owners
- aggregate inclusion metadata
- field path syntax and resolution rules
- error handling (fail-closed on invalid conditions)
- extension review process

**Required before completion:** inventory the actual per-service file variance across all five clusters and confirm the bounded condition model can express it.

**Deliverable:** a one-page schema document approved by the architecture owner, plus a proof-of-concept descriptor for at least one complex service (keycloak or cert-manager) demonstrating that the schema handles real-world variance.

Status: not started.

### Phase 4: Refactor rendering behind a fallback path

Replace convention-based rendering logic in:

- `internal/gitops/copy.go` (`shouldSkipFile`, `RenderSingleService`, `RenderClusterAppsAtomic`)
- `internal/template/embedded_registry.go` (`inferServices` and its hardcoded service list)

The renderer should decide file membership from descriptors, not path guessing.

**Required:**

- The rollback mechanism (decided in Phase 1) must be implemented. Both rendering paths must have CI coverage.
- The convention-based renderer must remain functional and tested until Phase 7 cutover.
- Undeclared-file validation must be implemented: any file in the embedded FS without a descriptor causes a build failure.
- Add embedded templates for `harbor`, `kafka-cluster`, and `mimir` (or document why they are excluded from parity).

Status: not started.

### Phase 5: Create the five config fixtures

Author:

- `.k8s-dev-config.yaml`
- `.k8s-dr-config.yaml`
- `.k8s-prod-config.yaml`
- `.k8s-qa-config.yaml`
- `.k8s-uat-config.yaml`

These files must include every renderer-owned service and cluster-scoped unit that appears in expected output. They must not introduce live secret material or material indistinguishable from real credentials.

**Required before completion:** compare RelayPoint overlay structure against at least two other customer repositories to confirm the fixture is representative.

Status: not started.

### Phase 6: Add the validation suite

For each cluster:

1. Load one config fixture.
2. Render renderer-owned output for one overlay tree.
3. Compare renderer-owned output against `testdata/relaypoint-logistics-shared/applications/overlays/<cluster>/`, excluding bootstrap-owned paths (`flux-system/`).
4. Apply only approved canonicalization rules before failing the test.
5. Validate any lifecycle-state claims made by the renderer contract.

This phase must also include:

- negative validation for unsupported conditions, invalid config, and security-sensitive inputs that violate approved policy
- undeclared-file detection tests
- single-service rendering tests
- validation that the sanitized fixture (no live secret material) is the test oracle

**Prerequisite:** the canonicalization inventory must be approved by the parity oracle owner before this phase begins.

Status: not started.

### Phase 7: Cutover approval and legacy cleanup

Only after validation and approval gates are met:

- cutover owner approves cutover from the convention-based renderer
- remove historical output drift that is covered by approved canonicalization rules
- clean up redundant or inconsistent files that are no longer part of the intended contract
- remove the convention-based renderer code (only after the cutover owner confirms rollback is no longer needed)

Legacy cleanup must not become an unbounded rewrite of the fixture.

Status: not started.

## Dependencies And Sequencing

This work is intentionally gated. The dependency graph includes explicit prerequisites that must be completed before Phase 1 can begin.

```
Prerequisites (secret triage, owner assignment, pre-commit hook)
                          │
              Phase 1 (contract + boundaries + rollback mechanism)
                          │
              ┌───────────┴───────────┐
              │                       │
    Phase 2 (security config)   Phase 3 (descriptor schema)
    [requires v2 stabilization] [requires per-service variance inventory]
              │                       │
              └───────────┬───────────┘
                          │
              ┌───────────┴───────────┐
              │                       │
    Phase 4 (renderer refactor)  Phase 5 (config fixtures)
    [requires rollback mechanism] [requires customer comparison]
              │                       │
              └───────────┬───────────┘
                          │
              Phase 6 (validation suite)
              [requires canonicalization inventory]
                          │
              Phase 7 (cutover approval + cleanup)
              [requires cutover owner approval]
```

- Prerequisites block Phase 1.
- Phase 1 blocks all implementation phases.
- Phase 2 requires a v2 config stabilization commitment.
- Phase 3 requires a per-service file variance inventory.
- Phase 2 and Phase 3 may start only after Phase 1 freezes boundaries and required decisions.
- Phase 4 depends on Phase 2, Phase 3, and the rollback mechanism from Phase 1.
- Phase 5 depends on Phase 2 and Phase 3 enough to encode fixtures without immediate rework.
- Phase 6 depends on Phase 4, Phase 5, and an approved canonicalization inventory.
- Phase 7 depends on Phase 6 and cutover owner approval.

## Validation And Approval Gates

Before this plan is approved for cutover, all of the following must be true:

- the committed private key has been triaged and the fixture sanitized
- named owners are assigned for all four roles
- there is an approved contract for renderer-owned files, bootstrap-owned files, and supported lifecycle states
- the rendering model inversion is documented with undeclared-file validation
- there is an approved security design for cluster-scoped secrets, repository trust inputs, and `.sops.yaml` generation inputs
- there is a finite, versioned canonicalization inventory approved by the parity oracle owner
- descriptor semantics and the typed config surface are frozen enough that Phase 4 does not code against a moving interface
- the rollback mechanism is implemented and both rendering paths have CI coverage
- validation covers renderer-owned parity, negative cases, single-service rendering behavior, undeclared-file detection, and lifecycle-state claims
- the RelayPoint fixture has been compared against at least two other customer repositories for representativeness
- the three missing embedded templates (`harbor`, `kafka-cluster`, `mimir`) have been addressed

## Acceptance Criteria

This work is complete when all of the following are true:

- there are five distinct per-cluster config fixtures containing only synthetic secret placeholders
- each fixture renders the renderer-owned portion of exactly one overlay tree
- rendered output matches the expected fixture output except for approved canonicalization rules
- cluster-scoped assets such as `.sops.yaml` and `customer-managed/` are driven from typed config and approved secret handling
- per-cluster structural variance is config-driven, not hardcoded
- `flux-system/` ownership and lifecycle treatment are explicit in docs and validation
- single-service rendering still works without scanning unrelated files
- the hardcoded `inferServices` list in `internal/template/embedded_registry.go` is replaced by descriptor-driven discovery for covered services
- undeclared-file validation prevents silent file drops
- the rollback mechanism is functional and tested
- the convention-based renderer is removed only after cutover owner approval

## Open Issues

- **Open issue (architecture):** decide whether pre-bootstrap overlays are a supported output state and how they are validated. Owner: architecture owner (TBD).
- **Open issue (security):** decide the secure source of truth and delivery mechanism for cluster-scoped secret material. Owner: security owner (TBD).
- **Open issue (security):** define repository policy for customer-managed Git sources, including transport and host verification expectations. Owner: security owner (TBD).
- **Open issue (security):** triage the committed private key in the k8s-uat fixture. Owner: security owner (TBD). **This is urgent and independent of the plan.**
- **Open issue (delivery):** assign named owners for architecture, security, parity oracle, and cutover. **This blocks Phase 1.**
- **Open issue (architecture):** freeze the v2 config types needed by this plan so renderer work does not chase an unstable interface. Owner: architecture owner (TBD). **This blocks Phase 2.**
- **Open issue (architecture):** decide the relationship between overlay unit descriptors and `ServicePluginManifest`. Owner: architecture owner (TBD).
- **Open issue (delivery):** add embedded templates for `harbor`, `kafka-cluster`, and `mimir`, or document why they are excluded from parity. **This blocks Phase 4.**
- **Open issue (delivery):** inventory per-service file variance across all five clusters to confirm the bounded condition model is sufficient. **This blocks Phase 3 completion.**
- **Open issue (delivery):** compare RelayPoint overlay structure against at least two other customer repositories. **This blocks Phase 5 completion.**

## Recommendation

Proceed with the descriptor-driven rendering design only as a gated implementation track. Do not treat fixture parity, cluster-scoped secret handling, or bootstrap-state claims as approved until the boundaries, security decisions, and validation requirements in this document are resolved.

Before any implementation work begins:

1. Triage the committed private key immediately.
2. Assign named owners for the four decision domains.
3. Produce the Phase 1 contract as a standalone, reviewable artifact.

## Evidence

Code paths referenced in this document:

- `cmd/cluster_service.go` — `getServiceOptions` (8 services + default), `getServiceSecrets`, `validateService` switch statements
- `internal/config/v2/config.go` — `GitOpsConfig` (no overlay unit fields), `ServiceMap` (`map[string]any`)
- `internal/gitops/copy.go` — `shouldSkipFile` (negative-list filter), `RenderSingleService`, `RenderClusterAppsAtomic` (walks embedded FS)
- `internal/gitops/embed.go` — `//go:embed all:gitops-base-dir all:templates` defining `Files` embedded FS
- `internal/gitops/templates/cluster-apps-base/` — embedded template filesystem (22 service directories)
- `internal/gitops/templates/cluster-apps-base/kustomization.yaml` — root template (references `./flux-system`, `./services/fluxcd`, `./managed-services/fluxcd` but NOT `./customer-managed/fluxcd`)
- `internal/services/plugin.go` — `ServicePluginManifest`, `TemplateRef` (with `Condition`), `ValidationRule` (not wired into rendering)
- `internal/template/embedded_registry.go` — `inferServices` with hardcoded 13-service list

Fixture paths verified:

- `testdata/relaypoint-logistics-shared/applications/overlays/k8s-dev/` — has `.sops.yaml`, `customer-managed/`, no `flux-system/`, includes `harbor` and `etcd-backup`, 21 service directories
- `testdata/relaypoint-logistics-shared/applications/overlays/k8s-dr/` — has `.sops.yaml`, `customer-managed/`, `flux-system/`
- `testdata/relaypoint-logistics-shared/applications/overlays/k8s-prod/` — no `.sops.yaml`, no `customer-managed/`, has `flux-system/`, 18 service directories, missing `kyverno` and `longhorn`
- `testdata/relaypoint-logistics-shared/applications/overlays/k8s-qa/` — no `.sops.yaml`, has `customer-managed/`, `flux-system/`, includes `sealed-secrets` and `longhorn`, 21 service directories
- `testdata/relaypoint-logistics-shared/applications/overlays/k8s-uat/` — no `.sops.yaml`, has `customer-managed/` with Secret manifest containing base64-encoded private key, no `flux-system/`
- `testdata/relaypoint-logistics-shared/applications/overlays/k8s-dev/kustomization.yaml` — references `./flux-system`, `./services/fluxcd`, `./managed-services/fluxcd`, `./customer-managed/fluxcd`

Embedded template vs. fixture gaps verified:

- `harbor`, `kafka-cluster`, `mimir` — present in fixture, absent from embedded templates
- `openstack-ccm`, `openstack-csi` — present in embedded templates, absent from fixture (provider-specific)
- Root `kustomization.yaml` template — missing `./customer-managed/fluxcd` reference present in fixture
