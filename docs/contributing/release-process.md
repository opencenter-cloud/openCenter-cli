---
last_updated: 2026-11-02
id: release-process
title: "Release Process and Codebase Touchpoints Guide"
sidebar_label: Release Process
description: End-to-end guide and checklist for releasing openCenter-cli, synchronizing upstream gitops-base releases, and updating lifecycle contracts across all codebase touchpoints.
doc_type: how-to
audience: "maintainers"
tags: [contributing, release]
---

# Release Process and Codebase Touchpoints Guide

**Purpose:** For maintainers, provides the end-to-end process and complete codebase touchpoint checklist for cutting openCenter-cli releases and release candidates, including upstream gitops-base synchronization and lifecycle contract progression.

`.github/workflows/release.yml` is the authoritative publishing engine for openCenter-cli. Whenever a `v*` tag is pushed to GitHub, this workflow compiles multi-architecture binaries for the CLI and local plugin, computes SHA-256 checksums, signs release assets with Sigstore Cosign, generates a software bill of materials (SBOM) with Anchore Syft, and publishes the release on GitHub.

However, preparing a release—whether a release candidate (RC) or a general availability (GA) release—requires synchronizing upstream dependencies, lifecycle contracts, configuration defaults, templates, schemas, and test assertions across the repository. This document catalogs every location that must be updated.

---

## 1. Release Architecture and Types

OpenCenter deployments depend on two coordinated layers:

1. **`openCenter-gitops-base`**: The upstream repository housing Infrastructure-as-Code (IaC) OpenTofu/Terraform modules, Helm charts, Flux CD Kustomizations, and platform catalog definitions. Pinned using calendar tags (such as `2026.03-rc05` or `2026.03`).
2. **`openCenter-cli`**: The operator tooling that renders cluster blueprints, manages secrets, validates configurations, drives OpenTofu provisioning, and deploys Kubernetes clusters. Versioned using semantic tags (such as `v1.0.0-rc06` or `v1.0.0`).

### Release Types

| Release Type | Git Tag Format | Purpose | Upstream Base Requirement |
| :--- | :--- | :--- | :--- |
| **Main / GA Release** | `vX.Y.Z` (e.g. `v1.0.0`) | Production-ready stable release. | Must pin a stable, immutable GA tag in `openCenter-gitops-base` (e.g. `2026.03`). |
| **Release Candidate (RC)** | `vX.Y.Z-rcNN` (e.g. `v1.0.0-rc07`) | Pre-release build for multi-platform integration testing and staging validation. | Pins the corresponding RC tag in `openCenter-gitops-base` (e.g. `2026.03-rc05`). |
| **Hotfix Release** | `vX.Y.Z` (e.g. `v1.0.1`) | Urgent bug or security fix branched from the prior stable release tag. | Keeps current stable base pin unless the fix requires an upstream patch release. |

### Lifecycle Contract Versions

The CLI and upstream Kubespray/OpenStack modules communicate through lifecycle contracts:

* **Contract v1 (Legacy)**: Rendered inventory directly into the shared GitOps worktree. Deprecated.
* **Contract v2**: Added `inventory_output_dir` to the Kubespray module, outputting inventory into the state directory (`.opencenter-state`) rather than dirtying the GitOps repository. Added CoreDNS rewrite block injection for authentication endpoints.
* **Contract v3**: Added `opencenter_network_id` and `opencenter_subnet_id` outputs to the `openstack-nova` module. Allows `opencenter cluster deploy` and `opencenter cluster observe` to discover OpenTofu-created tenant networks/subnets and bind them into the configuration so the Cloud Controller Manager (CCM) can populate the Octavia LoadBalancer subnet ID without manual entry.

---

## 2. Complete Codebase Touchpoints Checklist

When cutting any release or bumping the upstream `openCenter-gitops-base` dependency, update the following files in `openCenter-cli`:

### Summary Table of Touchpoints

| Category | File Path | Symbols / Keys to Update | Description |
| :--- | :--- | :--- | :--- |
| **Defaults** | `internal/config/v2/defaults.go` | `defaultGitBaseRepoRelease`, `kubesprayModuleTag`, `DefaultKubesprayModuleSourceURL`, OpenStack Nova module default | Pins default base repository release tag and default OpenTofu module sources. |
| **Contracts** | `internal/config/v2/kubespray_contract2.go` | `kubesprayModuleTagContract3`, `kubesprayModuleSourceNeedsContract3Upgrade`, `RequiresKubesprayModuleContract3Upgrade` | Defines the target contract tag and upgrade predicates. |
| **Loader** | `internal/config/v2/loader.go` | `normalizeConfig()` backfill call | Upgrades stale module pins in older configs at load time. |
| **Validation** | `internal/config/v2/generation_validation.go` | `ReportsKubesprayModuleContract3Gap`, `validateClusterGenerationConfig()` | Emits advisory warnings if an existing cluster has not yet been upgraded to the contract. |
| **IaC Templates** | `internal/gitops/templates/infrastructure-cluster-template/main-default.tf.tpl` | `openstack-nova`, `calico`, `cilium`, `kube-ovn` module source defaults | Default OpenTofu module references for OpenStack clusters. |
| **IaC Templates** | `internal/gitops/templates/infrastructure-cluster-template/main-baremetal.tf.tpl` | `calico`, `cilium`, `kube-ovn` module source defaults | Default OpenTofu module references for Baremetal clusters. |
| **IaC Templates** | `internal/gitops/templates/infrastructure-cluster-template/main-vmware.tf.tpl` | `calico`, `cilium`, `kube-ovn` module source defaults | Default OpenTofu module references for VMware clusters. |
| **IaC Templates** | `internal/gitops/templates/infrastructure-cluster-template/outputs.tf.tpl` | `opencenter_kubespray_lifecycle_contract_version` precondition, contract outputs | Asserts supported contract version range (`contains(["2", "3"])`) and defines output blocks. |
| **Tests** | `internal/gitops/terraform_calico_source_test.go` | `defaultGitBaseRepoTag`, `kubesprayModuleTag` | Verifies rendered Terraform template module source URLs. |
| **Tests** | `internal/gitops/kubespray_contract2_source_test.go` | `expected` source URL assertion | Verifies Kubespray source rendering for contract compliance. |
| **Tests** | `internal/config/v2/kubespray_contract2_backfill_test.go` | Stale pin test fixtures, `TestValidateGenerationWarnsOnContract3Gap` | Verifies backfill and gap detection. |
| **CLI Version** | `VERSION` | Version string (e.g. `1.0.0`) | Repository-level version file read by release automation. |
| **JSON Schemas** | `schema/opencenter-v2.schema.json` | Generated schema files | Run `mise run schema-v2` if any config structs changed. |
| **CLI Docs** | `docs/reference/opencenter/` | Auto-generated Cobra documentation | Run `mise run docs-gen` to sync CLI flags and command descriptions. |
| **Workflow Docs**| `docs/concepts/gitops-workflow.md`, `docs/operations/openstack-production-deployment.md` | Base repo tag and contract descriptions in markdown snippets | Guides and examples referencing default tags. |

---

## 3. Detailed Touchpoint Guidance

### A. Configuration Defaults (`internal/config/v2/defaults.go`)

This file defines the fallbacks used by `opencenter cluster init` and runtime config completion:

```go
// Base repo (gitops-base) is pinned to the immutable release tag so
// generated clusters resolve to a reproducible commit.
defaultGitBaseRepoRelease = "2026.03-rc05"

// kubesprayModuleTag pins the gitops-base tag carrying the lifecycle contract.
kubesprayModuleTag = "2026.03-rc05"
DefaultKubesprayModuleSourceURL = "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/provider/kubespray?ref=" + kubesprayModuleTag
```

Also verify the OpenStack Nova module default in `applyProviderCloudDefaults`:

```go
Source: "github.com/opencenter-cloud/openCenter-gitops-base.git//iac/cloud/openstack/openstack-nova?ref=2026.03-rc05",
```

### B. Lifecycle Contract Logic (`internal/config/v2/kubespray_contract2.go` & `loader.go`)

When advancing contracts:

1. Update the contract tag constant:
   ```go
   const kubesprayModuleTagContract3 = "2026.03-rc05"
   ```
2. Update the predicate functions to identify when an older ref requires promotion:
   ```go
   func kubesprayModuleSourceNeedsContract3Upgrade(source string) bool {
       // Returns true if source does not contain kubesprayModuleTagContract3
   }
   ```
3. Maintain backward-compatible aliases if earlier migration tests rely on contract-2 predicates.
4. Verify `internal/config/v2/loader.go` calls the latest upgrade check during configuration normalization:
   ```go
   if kubesprayModuleSourceNeedsContract3Upgrade(cfg.Deployment.Kubespray.KubesprayCluster.Source) {
       cfg.Deployment.Kubespray.KubesprayCluster.Source = DefaultKubesprayModuleSourceURL
   }
   ```
5. Ensure `internal/config/v2/generation_validation.go` issues an advisory warning if an existing cluster has an un-promoted module pin:
   ```go
   if ReportsKubesprayModuleContract3Gap(cfg) {
       reportWarning(..., "cluster set %s deployment.kubespray.kubespray_cluster.source=%s", ...)
   }
   ```

### C. OpenTofu Templates (`internal/gitops/templates/infrastructure-cluster-template/`)

Update the default `?ref=<tag>` parameters across all provider templates:

* **`main-default.tf.tpl`**:
  * `module "openstack-nova"`
  * `module "calico"`
  * `module "cilium"`
  * `module "kube-ovn"`
* **`main-baremetal.tf.tpl`**:
  * `module "calico"`
  * `module "cilium"`
  * `module "kube-ovn"`
* **`main-vmware.tf.tpl`**:
  * `module "calico"`
  * `module "cilium"`
  * `module "kube-ovn"`
* **`outputs.tf.tpl`**:
  * Ensure the contract version precondition permits all active versions:
    ```terraform
    condition = var.opencenter_lifecycle_mode == "legacy" || contains(
      ["2", "3"],
      try(tostring(module.kubespray-cluster.lifecycle_contract_version), ""),
    )
    ```

### D. Regression and Unit Tests

Keep test assertions in sync with the new release defaults:

* **`internal/gitops/terraform_calico_source_test.go`**:
  Update `defaultGitBaseRepoTag` and `kubesprayModuleTag` constants.
* **`internal/gitops/kubespray_contract2_source_test.go`**:
  Update `require.Equal(t, "...?ref=<tag>", m[1])`.
* **`internal/config/v2/kubespray_contract2_backfill_test.go`**:
  Update backfill assertions to check for the new tag. Ensure previous-version test fixtures (e.g. testing `2026.03-rc4` gap warnings) remain pinned to older versions to assert migration behavior.

### E. Documentation and Examples

Search the documentation tree for old release candidate tags:

```bash
git grep "2026.03-rc" docs/
```

Update references in:
* `docs/concepts/gitops-workflow.md`
* `docs/operations/openstack-production-deployment.md`

Regenerate reference docs if CLI command signatures or flags were modified:

```bash
mise run docs-gen
```

---

## 4. Pre-Release Verification Protocol

Run the full local test suite before tagging. Every step must be green:

```bash
# 1. Run unit tests, race detector, property tests, and vulnerability audit
mise run verify

# 2. Run BDD integration test scenarios
mise run godog

# 3. Verify JSON schema consistency
go test -v ./internal/config/v2schema/...

# 4. Verify documentation links, frontmatter, and mermaid diagrams
mise run test-docs

# 5. Check git working tree for unstaged whitespace or formatting issues
mise run test-diff
```

---

## 5. Local Preflight Build

Test the multi-architecture compilation locally without publishing:

```bash
mise run release v1.0.0-rc07
```

This compiles:
* `bin/release/opencenter-1.0.0-rc07-linux-amd64`
* `bin/release/opencenter-1.0.0-rc07-linux-arm64`
* `bin/release/opencenter-1.0.0-rc07-darwin-amd64`
* `bin/release/opencenter-1.0.0-rc07-darwin-arm64`
* Generates draft release notes in `bin/release/RELEASE_NOTES_1.0.0-rc07.md`.

---

## 6. Release Publishing Workflow

### Step 1: Commit the Changes

```bash
git checkout main
git pull origin main
git add -A
git commit -m "chore(release): prepare 1.0.0-rc07 and bump gitops-base to 2026.03-rc05"
git push origin main
```

### Step 2: Create and Push Annotated Tag

The release workflow is triggered exclusively by pushing a tag matching `v*`:

```bash
# Create annotated tag
git tag -a v1.0.0-rc07 -m "Release 1.0.0-rc07"

# Push tag to GitHub
git push origin v1.0.0-rc07
```

### Step 3: Monitor GitHub Actions

Watch the execution of `.github/workflows/release.yml`:

```bash
gh run list --workflow release.yml
gh run watch
```

### Step 4: What the Workflow Executes

1. **`build-cli`**: Compiles four target binaries (`linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`) with Git commit, branch, tag, and build timestamp injected via `-ldflags`.
2. **`build-plugin`**: Compiles `opencenter-local` binaries across the same four targets.
3. **`release`**:
   * Collects all eight binaries.
   * Computes SHA-256 checksums into `checksums.txt`.
   * Signs binaries and `checksums.txt` using Sigstore Cosign keyless signing (`.bundle` files).
   * Generates a Software Bill of Materials (SBOM) using Syft into `opencenter.spdx.json`.
   * Publishes the GitHub Release with automated release notes (`gh release create "${GITHUB_REF_NAME}" dist/* --generate-notes`).

---

## 7. Post-Release Verification

After the GitHub Actions workflow completes:

### 1. Verify GitHub Release Assets

Ensure the release page includes all 18 expected assets:
* 4 `opencenter-<version>-<os>-<arch>` CLI binaries
* 4 `opencenter-<version>-<os>-<arch>.bundle` Cosign signature bundles
* 4 `opencenter-local-<version>-<os>-<arch>` plugin binaries
* 4 `opencenter-local-<version>-<os>-<arch>.bundle` Cosign signature bundles
* 1 `checksums.txt`
* 1 `checksums.txt.bundle`
* 1 `opencenter.spdx.json`

### 2. Verify Release Candidate Flag

For release candidates, verify or set the pre-release status:

```bash
gh release edit v1.0.0-rc07 --prerelease
```

### 3. Spot-Check Binary Execution

Download and test the binary on your platform:

```bash
curl -L https://github.com/opencenter-cloud/openCenter-cli/releases/download/v1.0.0-rc07/opencenter-1.0.0-rc07-linux-amd64 -o /tmp/opencenter-test
chmod +x /tmp/opencenter-test
/tmp/opencenter-test version
```

Verify version output matches the release tag and Git commit.

### 4. Verify Cosign Signature

```bash
cosign verify-blob \
  --bundle opencenter-1.0.0-rc07-linux-amd64.bundle \
  --certificate-identity-regexp 'https://github.com/opencenter-cloud/openCenter-cli/' \
  --certificate-oidc-issuer-regexp 'https://token.actions.githubusercontent.com' \
  opencenter-1.0.0-rc07-linux-amd64
```

---

## 8. Rollback and Hotfix Procedures

### Rollback Strategy

Never delete or overwrite published release tags. Once artifacts have been downloaded by users or CI pipelines, tags must remain immutable.

If a critical flaw is discovered in a release:
1. Mark the GitHub Release as a pre-release with an advisory note directing users to the previous stable version:
   ```bash
   gh release edit v1.0.0 --prerelease --notes "DEPRECATED: Do not use due to issue #XYZ. Please use v0.9.9."
   ```
2. Prepare an immediate patch or hotfix release.

### Hotfix Workflow

```bash
# Branch from the affected release tag
git checkout -b hotfix/1.0.1 v1.0.0

# Apply fix and verify tests
# ...
git commit -m "fix(security): resolve vulnerability in auth handling"

# Merge hotfix into main
git checkout main
git merge hotfix/1.0.1
git push origin main

# Tag and push the new patch release
git tag -a v1.0.1 -m "Release 1.0.1"
git push origin v1.0.1
```
