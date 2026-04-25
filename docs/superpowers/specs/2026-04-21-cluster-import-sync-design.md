# Cluster Import and Live-to-Config Reconciliation Design

Date: 2026-04-21

## Summary

This design adds a GitOps-first cluster import workflow and extends drift reconciliation so operators can explicitly promote running cluster state back into openCenter configuration.

The resulting model is:

- Config remains authoritative between reconciliations.
- Live state can be promoted into config only through an explicit review-and-apply flow.
- Git records every accepted promotion.
- Import is conservative: only high-confidence, non-conflicting, non-sensitive fields are written automatically.

## Goals

- Discover and import all clusters from a customer GitOps repository by default.
- Fall back to live cluster inspection through `kubeconfig` when GitOps artifacts are unavailable or incomplete.
- Infer cluster and service configuration from known GitOps and live-cluster sources.
- Support all known openCenter services in the inference pipeline.
- Restrict deep service inference to owned namespaces, with built-in defaults and CLI overrides.
- Detect divergence between desired config and live state and let the operator choose whether to reconcile toward config or promote live state into config.
- Require explicit patch review before updating an existing config file.

## Non-Goals

- Reverse-engineer every possible service field from arbitrary Kubernetes resources.
- Auto-import or auto-promote secrets, credentials, tokens, or generated key material.
- Scan unrelated namespaces or attempt cluster-wide heuristic discovery outside service ownership boundaries.
- Make `sync-status` responsible for rewriting desired config.

## Operating Model

There are two separate workflows:

1. Import/adoption
   - Discover clusters from GitOps.
   - Infer canonical openCenter config.
   - Create missing configs or propose patches for existing configs.

2. Drift/reconciliation
   - Compare desired config to running state.
   - Allow explicit promotion of selected live changes back into config.
   - Preserve the ability to reconcile config back to live state.

This keeps desired-state mutation explicit and reviewable.

## CLI Design

### New Commands

```text
opencenter cluster import scan
opencenter cluster import apply
opencenter cluster import report
```

### Existing Commands to Extend

```text
opencenter cluster drift detect
opencenter cluster drift reconcile --to-config
opencenter cluster drift reconcile --to-cluster
```

### Command Responsibilities

#### `cluster import scan`

- Discovers all clusters from a GitOps repo by default.
- Uses GitOps artifacts as the primary source of truth for inference.
- Falls back to `kubeconfig` inspection when GitOps artifacts are unavailable or incomplete.
- Produces a structured scan result containing:
  - discovered clusters
  - inferred config fragments
  - field provenance
  - confidence per field
  - conflicts
  - skipped fields
  - service inference results

#### `cluster import apply`

- Creates missing cluster config files.
- For existing config files, generates a proposed patch and requires confirmation before writing.
- Writes only high-confidence, non-conflicting, non-sensitive fields.
- Leaves conflicts and low-confidence fields unresolved and reports them.

#### `cluster import report`

- Renders the most recent scan or apply result as `text`, `json`, or `yaml`.
- Supports operator review and later CI integration.

#### `cluster drift detect`

- Continues to compare desired config to live cluster state.
- Expands drift reporting beyond infrastructure-only drift to include promotable config drift where supported, such as:
  - Kubernetes version
  - control-plane and worker counts
  - service enablement
  - selected service settings
  - networking and storage fields when they can be mapped safely

#### `cluster drift reconcile --to-config`

- Promotes selected live changes into config.
- Uses the same patch engine as import apply.
- Always shows a patch before writing.
- Never auto-promotes protected or low-confidence fields.

#### `cluster drift reconcile --to-cluster`

- Keeps the existing desired-to-live reconciliation behavior.
- Applies config-driven corrections where supported by providers or service workflows.

## Data Source Order

Each field is inferred using ordered sources:

1. GitOps artifacts
2. Live cluster resources through `kubeconfig`
3. openCenter defaults

GitOps is preferred because it reflects committed intent and aligns with the repo layout exercised by sanitized fixtures such as [testdata/example-inc](/Users/victor.palma/projects/openCenter-cloud/openCenter-cli/testdata/example-inc).

## Import Pipeline

### Phase A: Cluster Discovery

- Discover all clusters from the customer GitOps repository.
- Derive organization, cluster name, environment, region, and provider hints from known paths and files.
- Map clusters to source directories such as:
  - `infrastructure/clusters/<cluster>/`
  - `applications/overlays/<cluster>/`

### Phase B: Infrastructure Inference

- Infer cluster-level fields such as:
  - provider
  - region
  - Kubernetes version
  - control-plane count
  - worker count
  - networking hints
  - storage hints
  - GitOps repository metadata

- Use live cluster inspection only to fill gaps or confirm runtime state when GitOps evidence is missing or incomplete.

### Phase C: Service Inference

- Iterate all known openCenter services.
- For each service, inspect only its owned namespaces.
- Infer:
  - enabled or disabled
  - namespace
  - adoption mode candidate
  - selected high-signal service fields
  - observed runtime status

### Phase D: Conflict Analysis

- GitOps and live agree: high confidence.
- GitOps present, live missing: usually high confidence.
- Live present, GitOps missing: medium confidence unless mapping is exact and safe.
- GitOps and live disagree: conflict.
- Evidence is partial or ambiguous: low confidence.

### Phase E: Patch Generation

- Build a canonical `v2.Config` proposal.
- Compare against any existing config file.
- Generate a patch for operator review.
- Write only approved, high-confidence, non-conflicting fields.

## Namespace-Scoped Service Detection

Each service gets a default namespace ownership map in the CLI. Operators can override it through flags.

Example:

```bash
opencenter cluster import scan \
  --service-namespace keycloak=keycloak \
  --service-namespace cert-manager=cert-manager \
  --service-namespace velero=velero
```

Rules:

- Built-in defaults are used unless overridden.
- Overrides replace or extend the owned namespace list for a service.
- The importer does not inspect arbitrary namespaces for a service.
- Resources found outside owned namespaces are ignored unless the operator explicitly includes those namespaces.

This keeps deep inference predictable and avoids accidental adoption of unrelated workloads.

## Confidence and Provenance Model

Every inferred field must include:

- field path
- inferred value
- confidence
- origin
- evidence

### Confidence Levels

- `high`
  - direct mapping from trusted GitOps artifact
  - unambiguous mapping from owned live resources
- `medium`
  - strong but indirect inference
- `low`
  - ambiguous or partial evidence
- `conflict`
  - GitOps and live disagree

### Write Policy

- Only `high` confidence fields are auto-writable.
- `medium` and `low` are report-only in v1.
- `conflict` is never written automatically.

## Config Mutation Rules

### Create Path

If no config exists for a discovered cluster:

- create a new canonical config file
- include only high-confidence, non-conflicting, non-sensitive fields
- report skipped and unresolved items

### Update Path

If a config already exists:

- generate a patch against the current file
- show the patch before writing
- require confirmation before writing
- write only approved, high-confidence, non-conflicting changes

### Protected Fields

Never auto-write:

- secrets
- credentials
- tokens
- generated key material
- other security-sensitive values that cannot be safely inferred

These may appear in evidence, but must be masked in reports and excluded from automatic config mutation.

## Conflict Policy

When GitOps and live state disagree:

- continue scanning all clusters
- write conflict-free results only
- include detailed conflict records in the report
- do not stop the full batch unless a fatal repo-level error occurs

Conflict records should include:

- cluster
- field path
- GitOps value
- live value
- evidence source
- recommended operator action

This policy allows import-all-by-default without turning one inconsistent cluster into a global blocker.

## Drift Promotion Model

`cluster drift reconcile --to-config` promotes selected live changes into config.

Flow:

1. Load desired config.
2. Query live state.
3. Build a structured drift report.
4. Classify drift items as:
   - promotable to config
   - reconcilable to cluster
   - manual or conflict-only
5. Generate a config patch for promotable items.
6. Show the patch and require confirmation.
7. Write the updated config and refresh metadata timestamps.

This preserves the design principle that config remains authoritative until the operator explicitly accepts live state as new desired state.

## Relationship to `sync-status`

`sync-status` remains narrow and safe.

It may continue updating observed metadata such as:

- service status
- last-seen runtime state
- health indicators

It must not become a general desired-state mutation command. Desired-state mutation belongs only in:

- `cluster import apply`
- `cluster drift reconcile --to-config`

## Internal Architecture

Add thin command files under `cmd/`:

- `cmd/cluster_import.go`
- `cmd/cluster_import_scan.go`
- `cmd/cluster_import_apply.go`
- `cmd/cluster_import_report.go`

Extend:

- [cmd/cluster.go](/Users/victor.palma/projects/openCenter-cloud/openCenter-cli/cmd/cluster.go)
- [cmd/cluster_drift.go](/Users/victor.palma/projects/openCenter-cloud/openCenter-cli/cmd/cluster_drift.go)

Add a new internal package:

- `internal/importer`

Recommended files:

- `repo_discovery.go`
- `source_inventory.go`
- `inference_engine.go`
- `field_inference.go`
- `service_inference.go`
- `namespace_registry.go`
- `confidence.go`
- `report.go`
- `patch.go`
- `apply.go`

The command layer should orchestrate input, output, and confirmation only. Inference, conflict handling, and patch generation should live in reusable internal packages.

## Core Types

Recommended result model:

```go
type ImportScanResult struct {
    RepoPath string
    Clusters []ClusterImportResult
    Summary  ImportSummary
}

type ClusterImportResult struct {
    ClusterName    string
    Organization   string
    Sources        ClusterSources
    ProposedConfig *v2.Config
    FieldResults   []FieldInferenceResult
    ServiceResults []ServiceInferenceResult
    Conflicts      []FieldConflict
    SkippedFields  []SkippedField
    ExistingConfig string
    ProposedPatch  string
}

type FieldInferenceResult struct {
    Path       string
    Value      any
    Confidence ConfidenceLevel
    Origin     FieldOrigin
    Evidence   []EvidenceRef
}

type ServiceInferenceResult struct {
    ServiceName  string
    Namespaces   []string
    Enabled      *bool
    AdoptionMode *config.AdoptionMode
    Fields       []FieldInferenceResult
}
```

Every inferred field needs provenance so the operator can understand why the tool proposed a change.

## Service Detector Registry

Support all known openCenter services through a registry-driven model instead of a single large `switch`.

Recommended interface:

```go
type ServiceDetector interface {
    ServiceName() string
    DefaultNamespaces() []string
    Detect(ctx context.Context, src ClusterSources, opts DetectOptions) ServiceInferenceResult
}
```

Each detector is responsible for:

- owned namespaces
- supported GitOps and live resource lookups
- field mapping
- confidence and evidence production

This fits the existing registry-oriented style already present in the codebase and keeps service-specific heuristics isolated.

## Scan Result Persistence

`cluster import scan` should persist the scan result as a local artifact so `report` and `apply` can operate without forcing an immediate re-scan.

The cache should be stored in the CLI state or cache directory and keyed by repo path plus scan timestamp.

This improves:

- operator review flow
- repeatability
- debugging
- testability

## Error Handling

### Fatal Repo-Level Errors

- repo path unreadable
- no recognizable cluster structure
- cached scan artifact corrupt and no rescan requested

### Non-Fatal Cluster-Level Errors

- missing kubeconfig
- unreadable Terraform or OpenTofu file
- cluster API unreachable
- service detector failure for one service

### Non-Fatal Field-Level Errors

- ambiguous mappings
- partial resources
- unsupported resource shapes

Behavior:

- continue at the widest safe scope
- collect errors and warnings in the result
- never silently drop failed inference without explanation

## Edge Cases

- GitOps-only cluster
  - create best-effort config from GitOps evidence
- Live-only cluster
  - infer safe fields from live state and report missing repo-backed values
- Existing config with local customizations
  - patch against the existing file rather than replacing it wholesale
- Service resources outside owned namespaces
  - ignore unless operator overrides namespace ownership
- Multiple candidate resources for one field
  - do not guess; mark low confidence or conflict
- Sensitive values visible in resources
  - mask in reports and never auto-write

## Exit Codes

Recommended default behavior:

- `0` for successful scan or apply, even when conflicts or skipped fields exist
- `1` for fatal failures that prevent usable output

Optional later flags may add stricter behavior:

- `--fail-on-conflict`
- `--fail-on-skipped`
- `--strict`

## Testing Strategy

### Unit Tests

- confidence classification
- namespace override logic
- protected-field filtering
- field conflict detection
- patch generation

### Service Detector Tests

- GitOps-only evidence
- live-only evidence
- conflicting evidence
- ambiguous evidence

### Fixture-Based Integration Tests

Use [testdata/example-inc](/Users/victor.palma/projects/openCenter-cloud/openCenter-cli/testdata/example-inc) to verify:

- import-all scan across all discovered clusters
- inferred cluster values
- service inference behavior
- conflict reporting
- skipped field reporting
- report rendering
- apply behavior for new and existing configs

### Drift Promotion Tests

- desired vs live diff generation
- `--to-config` patch generation
- exclusion of protected and conflicting fields
- confirmation-gated write behavior

## Implementation Boundaries for V1

V1 should include:

- import-all from GitOps repo by default
- kubeconfig fallback
- service detector coverage for all known openCenter services
- high-signal field inference per service
- patch-based writes for existing configs
- explicit live-to-config promotion through drift reconciliation

V1 should not include:

- low-confidence auto-apply
- implicit secret import
- arbitrary namespace scanning
- whole-file regeneration when targeted patching is sufficient

## Final Recommendation

Implement a new `cluster import` workflow backed by a GitOps-first inference engine and a patch-driven apply path. Extend `cluster drift` so operators can explicitly promote accepted live changes back into config via `--to-config`.

This gives openCenter a complete loop:

- import existing clusters into canonical config
- detect divergence between desired and actual state
- reconcile either toward the cluster or toward the config
- keep Git as the durable audit trail for accepted changes
