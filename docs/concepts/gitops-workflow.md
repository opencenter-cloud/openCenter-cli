---
last_updated: 2026-09-25
id: gitops-workflow
title: "GitOps Workflow"
sidebar_label: GitOps Workflow
description: How openCenter uses Git as the source of truth with FluxCD reconciliation.
doc_type: explanation
audience: "platform teams, operators"
tags: [gitops, fluxcd, reconciliation, kustomize]
---
# GitOps Workflow

**Purpose:** For platform teams, explains the GitOps repository structure and reconciliation process, covering repository layout through drift detection.

Understanding the GitOps workflow helps you manage clusters effectively and troubleshoot reconciliation issues. This explanation covers how openCenter uses Git as the source of truth.

## GitOps Principles

openCenter follows these GitOps principles:

1. **Git as Single Source of Truth:** All cluster state defined in Git
2. **Declarative Configuration:** Describe desired state, not steps
3. **Automated Reconciliation:** FluxCD continuously syncs Git → Cluster
4. **Immutable Deployments:** Changes via Git commits, not kubectl

**Why GitOps:** Audit trail (Git history), rollback capability (Git revert), collaboration (pull requests), security (no direct cluster access needed).

## Repository Structure

openCenter generates a standardized GitOps repository:

```
<git_dir>/
├── .gitignore
├── .sops.yaml                     # SOPS encryption rules
├── README.md
│
├── applications/
│   └── overlays/<cluster>/
│       ├── .sops.yaml             # Cluster-specific encryption
│       ├── kustomization.yaml
│       │
│       ├── services/              # Platform services
│       │   ├── sources/           # GitRepository sources
│       │   │   ├── opencenter-cert-manager.yaml
│       │   │   ├── opencenter-kyverno.yaml
│       │   │   └── ...
│       │   │
│       │   ├── fluxcd/            # Kustomization resources
│       │   │   ├── cert-manager.yaml
│       │   │   ├── kyverno.yaml
│       │   │   └── ...
│       │   │
│       │   └── <service>/         # Service-specific overrides
│       │       ├── kustomization.yaml
│       │       └── override-values.yaml
│       │
│       └── managed-services/      # Customer applications
│           ├── sources/
│           ├── fluxcd/
│           └── <app>/
│
├── infrastructure/
    └── clusters/<cluster>/
        ├── main.tf                # Terraform/OpenTofu
        ├── provider.tf
        ├── variables.tf
        ├── inventory/             # Kubespray Ansible
        │   ├── inventory.yaml
        │   ├── group_vars/
        │   └── credentials/
        └── kubeconfig.yaml        # Generated after deployment

└── clusters/<cluster>/             # Flux bootstrap path and bridge
    ├── flux-system/                 # Written by flux bootstrap
    └── services.yaml                # openCenter-generated bridge
```

**Design Rationale:**

* **Separation:** Infrastructure (Terraform/OpenTofu) separate from applications (Kubernetes)
* **Overlays:** Cluster-specific configuration without duplicating base
* **Encryption:** SOPS configuration at multiple levels (root, cluster)
* **Sources:** GitRepository CRDs reference openCenter-gitops-base

Generated files are tracked by repository-relative ownership records in separate version-2 ledgers under `.opencenter/ownership/`. Each record contains a SHA-256 and permission mode:

```text
.opencenter/ownership/clusters/<cluster>.json
.opencenter/ownership/global.json
```

The cluster ledger covers only the active cluster's explicit recursive scopes: `applications/overlays/<cluster>/`, `infrastructure/clusters/<cluster>/`, and `clusters/<cluster>/`, excluding Flux bootstrap files below `clusters/<cluster>/flux-system/` but including the generated bridge files beside that directory. `global.json` is an exact-file allowlist for `.gitignore`, `README.md`, `applications/overlays/.gitkeep`, and `infrastructure/clusters/.gitkeep`; it is not authority over every repository file. Full generation may use all applicable scopes and the global allowlist, while applications-only and single-service promotion inspect and prune only their active scope. Sibling clusters and their ledger records remain isolated. A staged default may seed a missing file under a service's `custom/` directory, but existing custom files are not scanned as generator-owned, overwritten, or pruned; secret-sync-owned artifacts with matching hashes are also excluded from generator ownership.

The pre-v1 transition is intentionally fail-fast: a repository-root or current overlay `.opencenter-generated.json` legacy manifest stops promotion before any mutation, with no automatic migration. Dry-run performs the same ownership preflight without writing; disabling prune reports candidates without deleting them; `--adopt-generated` is explicit and limited to an untracked planned collision, backing up the replaced file during a real apply. Modified tracked files and unknown files still block, and `--force` does not bypass ownership safety. See [Rendering Ownership and Secret Artifacts](../CODEMAPS/rendering-ownership-and-secret-artifacts.md).

**Evidence:** `internal/gitops/`

## FluxCD Components

### Source Controller

**Purpose:** Fetch and cache Git repositories, Helm charts, and OCI artifacts.

**Resources:**

* `GitRepository`: Git repository source
* `HelmRepository`: Helm chart repository
* `Bucket`: S3-compatible bucket

**Reconciliation:** Polls sources at configured interval (default 15m), detects changes, notifies dependent controllers.

**Example** (for a base-service source such as `opencenter-cert-manager`, rendered through `sourceAuthBlockForService`/`sourceAuthBlockAnonymous` in `internal/gitops/copy.go` and `internal/gitops/source_auth.go`):

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-cert-manager
  namespace: flux-system
spec:
  interval: 15m
  # --- token auth (active) ---
  url: https://github.com/opencenter-cloud/openCenter-gitops-base.git
  ref:
    tag: "2026.01"
  # --- ssh auth (alternative) ---
  # url: ssh://git@github.com/opencenter-cloud/openCenter-gitops-base.git
  # ref:
  #   tag: "2026.01"
```

`opencenter cluster generate <cluster> --gitops-auth=token|ssh` controls which block is active. With no flag, generation uses `cluster_defaults.gitops_auth_method`, then the built-in `token` default. The choice only affects that run; it does not modify cluster configuration.

**There is no `secretRef` on this source, in either mode.** `openCenter-gitops-base` is a public repository, so base-service sources render anonymously — attaching a credential Secret would make Flux authenticate to public GitHub with the wrong credential and fail with HTTP 401 (the exact scenario `internal/gitops/source_auth.go` guards against). No `opencenter-base` credential Secret is created anywhere in the bootstrap path (see the comments in `internal/cluster/bootstrap_provider_infra.go` and `internal/cluster/kind_bootstrap_provider.go`).

The one source that *does* carry a `secretRef` is `opencenter-keycloak-config`, because it points at the **customer's own** GitOps repository rather than the public base repo. It uses `secretRef: name: flux-system` — the credential Secret the `flux` CLI itself creates when `opencenter cluster deploy` runs `flux bootstrap` against the customer repository.

### Kustomize Controller

**Purpose:** Apply Kustomize manifests to cluster.

**Resources:**

* `Kustomization`: Kustomize build and apply

**Reconciliation:** Builds Kustomize overlay, applies to cluster, waits for health checks, prunes deleted resources.

**Example:**

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: cert-manager-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: opencenter-cert-manager
    namespace: flux-system
  path: applications/base/services/cert-manager
  targetNamespace: cert-manager
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: cert-manager
      namespace: cert-manager
```

(This is the actual generated `cert-manager-base` Kustomization, from `internal/gitops/templates/cluster-apps-base/services/fluxcd/cert-manager.yaml.tpl`; the paired `cert-manager-override` Kustomization additionally sets `decryption.provider: sops` with `secretRef: name: sops-age` and points its own `sourceRef` at the customer's `flux-system` GitRepository instead of the base repo. See [Service Templates](services-templates.md) for the full pair and the `force`/`suspend`/`commonMetadata` fields both carry.)

### Helm Controller

**Purpose:** Deploy and manage Helm releases.

**Resources:**

* `HelmRelease`: Helm chart deployment

**Reconciliation:** Fetches chart, merges values, installs/upgrades release, monitors health.

**Example:**

```yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: cert-manager
  namespace: cert-manager
spec:
  interval: 5m
  chart:
    spec:
      chart: cert-manager
      version: 1.18.2
      sourceRef:
        kind: HelmRepository
        name: jetstack
  values:
    installCRDs: true
    global:
      leaderElection:
        namespace: cert-manager
```

### Notification Controller

**Purpose:** Send notifications about reconciliation events.

**Resources:**

* `Alert`: Alert configuration
* `Provider`: Notification provider (Slack, Discord, etc.)

**Reconciliation:** Watches events, filters by severity, sends notifications.

## Reconciliation Process

### Initial Bootstrap

```
1. User: opencenter cluster deploy
    ↓
2. FluxCD: Install controllers (source, kustomize, helm, notification)
    ↓
3. FluxCD: Create gotk-sync Kustomization
    ↓
4. Kustomize Controller: Apply applications/overlays/<cluster>/
    ↓
5. Source Controller: Fetch GitRepository sources
    ↓
6. Kustomize Controller: Apply service Kustomizations
    ↓
7. Helm Controller: Deploy HelmReleases
    ↓
8. Services: Running in cluster
```

### Continuous Reconciliation

```
Every 15 minutes (GitRepository interval):
    Source Controller: Poll Git repository
    If changes detected:
        Source Controller: Fetch new commits
        Source Controller: Notify Kustomize Controller

Every 5 minutes (Kustomization interval):
    Kustomize Controller: Check source revision
    If source changed OR interval elapsed:
        Kustomize Controller: Build Kustomize overlay
        Kustomize Controller: Apply to cluster
        Kustomize Controller: Wait for health checks
        Kustomize Controller: Prune deleted resources

Every 5 minutes (HelmRelease interval):
    Helm Controller: Check chart version
    If chart changed OR values changed:
        Helm Controller: Fetch chart
        Helm Controller: Merge values
        Helm Controller: Upgrade release
        Helm Controller: Monitor health
```

**Why these intervals:** 15m for Git polling reduces load on Git server. 5m for Kustomization provides fast reconciliation. Intervals are configurable per resource.

These are FluxCD's own default `interval` values (15m for `GitRepository`, 5m or 15m for `Kustomization` depending on the service; see the templates cited above); intervals are configurable per resource.

## Kustomize Overlay Pattern

### Base + Overlay Composition

**Pattern:** Base manifests in openCenter-gitops-base, cluster-specific overrides in customer repository.

**Example:**

Base (openCenter-gitops-base):

```yaml
# applications/base/services/cert-manager/helmrelease.yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: cert-manager
  namespace: cert-manager
spec:
  chart:
    spec:
      chart: cert-manager
      version: 1.18.2
  values:
    installCRDs: true
```

Overlay (customer repository):

```yaml
# applications/overlays/my-cluster/services/cert-manager/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../../../base/services/cert-manager

# applications/overlays/my-cluster/services/cert-manager/override-values.yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: cert-manager
  namespace: cert-manager
spec:
  values:
    global:
      leaderElection:
        namespace: cert-manager
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
```

**Benefits:**

* Base provides security-hardened defaults
* Overlay provides cluster-specific configuration
* No duplication of base manifests
* Easy to update base (change tag in GitRepository)

**Trade-offs:** Requires understanding Kustomize. Debugging can be harder (need to build overlay to see final manifest).

## SOPS Integration

### Encryption at Rest (Git)

**Pattern:** Secrets encrypted with SOPS Age before commit.

**Configuration:**

```yaml
# .sops.yaml, rendered from internal/gitops/templates/cluster-apps-base/.sops.yaml.tpl
creation_rules:
{{- range .OpenCenter.GitOps.OverlayUnits.SOPS.Rules }}
  - path_regex: {{ .PathRegex }}
    age: {{ join "," .AgeRecipients }}
    encrypted_regex: {{ .EncryptedRegex }}
{{- end }}
```

The template renders one `creation_rules` entry per configured SOPS overlay rule (`opencenter.gitops.overlay_units.sops.rules`, `internal/config/overlay/types.go`) rather than one fixed rule. Separately, the top-level `secrets.sops.encrypted_regex` (`internal/config/v2/config.go` `SOPSConfig`) defaults to `^(data|stringData|secret)$` (`internal/config/v2/defaults.go`) and governs SOPS's own field-matching behavior for provider/GitOps overlay values encrypted directly by `internal/sops` (see [Secrets Management](../CODEMAPS/secrets-management.md)) — a related but distinct mechanism from the per-rule overlay-unit encryption shown above.

**Workflow:**

```
1. User: Edit secret in plaintext
2. User: opencenter secrets encrypt
3. SOPS: Encrypt with Age key
4. User: git commit (encrypted secret)
5. User: git push
```

### Decryption in Cluster

**Pattern:** FluxCD decrypts secrets during reconciliation.

**Setup:**

```bash
# Create Age key secret in cluster
kubectl create secret generic sops-age \
  --from-file=age.agekey=$SOPS_AGE_KEY_FILE \
  -n flux-system
```

**Kustomization with decryption:**

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: my-service
  namespace: flux-system
spec:
  decryption:
    provider: sops
    secretRef:
      name: sops-age
```

**Reconciliation:**

```
1. Kustomize Controller: Fetch encrypted manifest from Git
2. Kustomize Controller: Decrypt with Age key from sops-age secret
3. Kustomize Controller: Apply decrypted manifest to cluster
4. Kubernetes: Store secret in etcd (encrypted at rest)
```

**Why this design:** Secrets safe in Git (encrypted), FluxCD handles decryption automatically, no manual decryption needed.

**Evidence:** `internal/sops/manager.go`

## Dependency Management

### dependsOn Chains

**Pattern:** Explicit dependencies between Kustomizations.

**Example:**

Real chains have more than one link and more than one named stage per service. Keycloak's actual chain (`internal/gitops/templates/cluster-apps-base/services/fluxcd/keycloak.yaml.tpl`) is, in order: `sources` → `keycloak-namespace` → (with `postgres-operator-base` also required) a Postgres cluster stage → (with `olm-base` also required) the Keycloak operator stage → the Keycloak override stage, e.g.:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: keycloak-operator
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: olm-base
      namespace: flux-system
    - name: keycloak-postgres
      namespace: flux-system
```

**Reconciliation:**

```
1. FluxCD: apply sources, wait Ready
2. FluxCD: apply keycloak-namespace, wait Ready
3. FluxCD: apply the Postgres-cluster stage (also depends on postgres-operator-base), wait Ready
4. FluxCD: apply keycloak-operator (also depends on olm-base), wait Ready
5. FluxCD: apply the Keycloak override stage (depends on keycloak-postgres and keycloak-operator)
```

**Why this design:** Ensures services deploy in correct order. Prevents failures due to missing dependencies.

**Trade-offs:** Slower initial deployment (sequential). But more reliable (no race conditions).

## Drift Detection

### Automatic Drift Detection

**Pattern:** FluxCD detects drift on every reconciliation interval.

**Process:**

```
1. Kustomize Controller: Build desired state from Git
2. Kustomize Controller: Fetch actual state from cluster
3. Kustomize Controller: Compare desired vs actual
4. If drift detected:
    Kustomize Controller: Apply desired state
    Kustomize Controller: Log drift correction
```

**Example Drift:**

```
Desired (Git): replicas: 3
Actual (Cluster): replicas: 5 (manually scaled)

FluxCD: Detects drift, scales back to 3
```

**Why this design:** Self-healing (cluster always matches Git), prevents configuration drift, enforces GitOps discipline.

**Trade-offs:** Manual changes are reverted. But this is intentional (Git is source of truth).

### Manual Drift Detection

**Command:** `opencenter cluster drift detect <cluster>` (also `drift reconcile`, `drift schedule`).

**Purpose:** Compare configuration file vs actual infrastructure state through `internal/operations.DriftDetector` and a provider's `internal/cloud.CloudProvider` implementation.

**Use Case:** Detect infrastructure drift (VMs deleted, networks changed) before reconciliation. This is a separate mechanism from FluxCD's own drift correction above — see [Infrastructure Drift Detection](drift-detection.md) for supported providers and reconciliation limits.

## Update Strategies

### Rolling Updates

**Pattern:** Update Git, FluxCD reconciles automatically.

**Workflow:**

```
1. User: Update configuration file
2. User: opencenter cluster generate
3. User: git commit -m "Update service configuration"
4. User: git push
5. FluxCD: Detects change (within 15m)
6. FluxCD: Reconciles new state
7. Services: Updated in cluster
```

**Rollback:**

```
1. User: git revert <commit>
2. User: git push
3. FluxCD: Reconciles previous state
4. Services: Rolled back
```

### Blue-Green Deployments

**Pattern:** Deploy new version alongside old, switch traffic.

**Workflow:**

```
1. User: Deploy green version
2. User: Test green version
3. User: Switch traffic to green
4. User: Decommission blue version
```

**Implementation:** Requires manual orchestration or external tools.

## Troubleshooting Reconciliation

### GitRepository Not Syncing

**Symptom:** `kubectl get gitrepositories -n flux-system` shows authentication error.

**Diagnosis:**

```bash
kubectl describe gitrepository <name> -n flux-system
```

**Common Causes:**

* SSH key not found or incorrect
* Git URL incorrect
* Branch/tag doesn’t exist

**Solution:** Recreate SSH key secret, verify Git URL, check branch exists.

### Kustomization Failing

**Symptom:** `kubectl get kustomizations -n flux-system` shows reconciliation error.

**Diagnosis:**

```bash
kubectl describe kustomization <name> -n flux-system
kubectl logs -n flux-system deployment/kustomize-controller
```

**Common Causes:**

* Path not found in Git repository
* SOPS decryption failed
* Invalid manifest syntax
* Health check failed

**Solution:** Verify path, check SOPS key, validate manifests, check pod status.

### HelmRelease Failing

**Symptom:** `kubectl get helmreleases -A` shows failed status.

**Diagnosis:**

```bash
kubectl describe helmrelease <name> -n <namespace>
kubectl logs -n flux-system deployment/helm-controller
```

**Common Causes:**

* Chart not found
* Values error
* Dependency not ready

**Solution:** Verify HelmRepository, check values, wait for dependencies.

## Best Practices

### 1. Use Tags for Stability

**Practice:** Reference openCenter-gitops-base by tag, not branch.

**Example:**

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-cert-manager
spec:
  ref:
    tag: v1.0.0  # Not branch: main
```

**Rationale:** Tags are immutable, branches can change. Tags ensure reproducible deployments.

### 2. Test Changes in Dev First

**Practice:** Apply changes to dev cluster before production.

**Workflow:**

```
1. Update dev cluster configuration
2. Deploy to dev
3. Test thoroughly
4. Apply same changes to prod
```

**Rationale:** Catch errors in dev, not prod. Validate changes before production.

### 3. Use Small Commits

**Practice:** One logical change per commit.

**Example:**

```
Good: "Update cert-manager to v1.18.2"
Bad: "Update cert-manager, add loki, fix keycloak"
```

**Rationale:** Easier to review, easier to rollback, clearer history.

### 4. Monitor Reconciliation

**Practice:** Watch FluxCD status regularly.

**Commands:**

```bash
# Check all Kustomizations
kubectl get kustomizations -A

# Check specific service
kubectl describe kustomization cert-manager-base -n flux-system

# Watch reconciliation
flux logs --follow
```

**Rationale:** Detect issues early, understand reconciliation status.

### 5. Document Cluster-Specific Decisions

**Practice:** Update cluster README.md with decisions.

**Example:**

```markdown
# my-cluster

## Configuration Decisions

- Using OVN load balancer (no Octavia quota)
- Disabled Loki (cost optimization)
- Custom cert-manager email (team@example.com)
```

**Rationale:** Context for future maintainers, audit trail for decisions.

## Common Misconceptions

### "FluxCD applies changes immediately"

**Reality:** FluxCD polls Git at configured interval (default 15m). Changes take 5-15 minutes to apply. Use `flux reconcile` to force immediate reconciliation.

### "Manual kubectl changes are permanent"

**Reality:** FluxCD reverts manual changes on next reconciliation. All changes must go through Git.

### "Kustomize overlays replace base manifests"

**Reality:** Kustomize overlays merge with base manifests. Use strategic merge patches or JSON patches for precise control.

### "SOPS decryption happens in Git"

**Reality:** Secrets stay encrypted in Git. FluxCD decrypts in-memory during reconciliation. Decrypted secrets never touch disk.

### "GitOps means no manual intervention"

**Reality:** Manual intervention is sometimes necessary (debugging, emergencies). But changes should be committed to Git afterward.

## Further Reading

* [Architecture](architecture.md) - System design and components
* [Security Model](security-model.md) - Security architecture
* [Manage Secrets](../operations/manage-secrets.md) - SOPS and secrets management
* [Troubleshoot Deployment](../operations/troubleshoot-deployment.md) - Fix reconciliation issues

---

## Evidence

This explanation is based on:

* Repository/rendering structure: `internal/gitops/` (`copy.go`, `source_auth.go`, `ownership.go`, `render_catalog.go`), `internal/gitops/templates/cluster-apps-base/`
* GitOps auth flag and defaults: `cmd/cluster_generate.go`, `internal/config/v2/defaults.go`
* Bootstrap/Flux bootstrap: `internal/cluster/bootstrap_provider_infra.go`, `internal/cluster/openstack_flux_bootstrap.go`, `internal/cluster/kind_bootstrap_provider.go`
* SOPS integration: `internal/sops/`, `internal/config/v2/config.go` (`SOPSConfig`)
* Drift detection: `internal/operations/`, [Infrastructure Drift Detection](drift-detection.md)
