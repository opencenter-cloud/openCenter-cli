---
last_updated: 2026-09-25
id: openstack-provider-storage-operations
title: "Explain OpenStack Provider and Storage Operations"
sidebar_label: OpenStack Provider and Storage
description: "Execution map for read-only OpenStack provider planning and explicit one-service storage planning/apply, including persistence and recovery."
doc_type: explanation
audience: "contributors, maintainers, operators"
tags: [openstack, provider, storage, plan, apply, credentials]
---
# OpenStack provider and storage operations

These are two separate command families:

```text
cluster provider openstack plan <cluster>
cluster provider openstack apply <cluster>
cluster service storage plan <service> --cluster <cluster> --backend swift|s3
cluster service storage apply <service> --cluster <cluster> --backend swift|s3
```

## Feature → subsystem → symbol

| Feature | Subsystem / package | File → symbol | Dependencies and evidence |
|---|---|---|---|
| Provider command | Cobra handler / `cmd` | [`cmd/cluster_provider_openstack.go`](../../cmd/cluster_provider_openstack.go) → `newClusterProviderOpenStackCmd`, `runClusterProviderOpenStack` | v2 public decode, profile loading, output/confirmation; [`cmd/cluster_provider_openstack_test.go`](../../cmd/cluster_provider_openstack_test.go) |
| Profile discovery | cloud adapter / `internal/cloud/openstack` | [`internal/cloud/openstack/profile.go`](../../internal/cloud/openstack/profile.go) → `LoadProfile`; [`internal/cloud/openstack/read_only_discovery.go`](../../internal/cloud/openstack/read_only_discovery.go) → `ProfileDiscovery.DiscoverWithOptions` | Gophercloud reads images/networks/subnets/project scope; [`internal/cloud/openstack/provider_test.go`](../../internal/cloud/openstack/provider_test.go) |
| Provider plan | typed planner / `internal/cluster/provider/openstack` | [`internal/cluster/provider/openstack/service.go`](../../internal/cluster/provider/openstack/service.go) → `Plan` | Candidate v2 config, ambiguity/replacement rules, redacted result; [`internal/cluster/provider/openstack/service_test.go`](../../internal/cluster/provider/openstack/service_test.go) |
| Provider persistence | typed persistence / same package | [`internal/cluster/provider/openstack/service.go`](../../internal/cluster/provider/openstack/service.go) → `ApplyPersistence.Apply` | Re-read/stale-byte check, backup, public marshal, atomic write; [`internal/cluster/provider/openstack/service_test.go`](../../internal/cluster/provider/openstack/service_test.go) |
| Storage command | Cobra handler / `cmd` | [`cmd/cluster_service_storage.go`](../../cmd/cluster_service_storage.go) → `newClusterServiceStorageCmd`, `runClusterServiceStorage` | Requires one service, cluster, backend; maps partial result to exit code 4; [`cmd/cluster_service_storage_test.go`](../../cmd/cluster_service_storage_test.go) |
| Storage plan | typed workflow / `internal/cluster/storage/openstack` | [`internal/cluster/storage/openstack/service.go`](../../internal/cluster/storage/openstack/service.go) → `ValidateOptions`, `Plan` | Service/backend allowlist, endpoint/container checks, credential reuse/rotation, preflight; [`internal/cluster/storage/openstack/service_test.go`](../../internal/cluster/storage/openstack/service_test.go) |
| Remote adapter | OpenStack API / `internal/cloud/openstack` | [`internal/cloud/openstack/storage.go`](../../internal/cloud/openstack/storage.go) → `StorageAdapter`, `Preflight`, `EnsureContainer`, `CreateAppCredential`, `CreateEC2Credentials` | Gophercloud object storage/Keystone; [`internal/cloud/openstack/storage_test.go`](../../internal/cloud/openstack/storage_test.go) |
| Storage apply/recovery | typed workflow | [`internal/cluster/storage/openstack/service.go`](../../internal/cluster/storage/openstack/service.go) → `Apply`, `partialResult` | Recovery journal, credential creation, stale re-read, atomic persistence, old-credential revoke; [`cmd/task16_storage_contract_test.go`](../../cmd/task16_storage_contract_test.go) |

## Actual provider path

```text
runClusterProviderOpenStack
  -> resolve cluster paths -> os.ReadFile -> DecodePublicConfig
  -> LoadProfile -> ProfileDiscovery.DiscoverWithOptions (read-only)
  -> provider/openstack.Plan
  -> ConfigIOHandler.ValidateConfig(candidate)
  -> [apply + confirmed + non-dry-run]
       -> ApplyPersistence.Apply -> stale check -> backup -> atomic public config write
```

Provider planning fills unambiguous selections and reports ambiguous candidates. Every ambiguous field also emits a warning naming its config path, the candidate count, and the selector that resolves it (`--image-id`, `--windows-image-id`, `--network-id`, `--subnet-id`, `--external-network-id`, `--availability-zone`), so a blocked plan states its reason instead of only printing candidates. Populated values need `--replace`; optional auth/TLS imports are explicit and sensitive result values are redacted. Neither provider plan nor apply creates a remote resource, container, or credential.

## Actual storage path

```text
runClusterServiceStorage
  -> ValidateOptions -> loadStorageConfig (raw bytes + v2 decode)
  -> LoadProfile -> NewStorageAdapter (remote backends)
  -> storage/openstack.Plan
       -> effective service/backend/container
       -> adapter.Preflight(resolve owner only when needed)
       -> prospective typed config + ordered remote actions
  -> [plan/dry-run return] or confirmation
  -> storage/openstack.Apply
       -> re-read and compare original bytes
       -> re-plan current config
       -> create credential when needed + per-service recovery journal (<service>.storage-recovery.json)
       -> validate candidate -> backup + atomic persist
       -> revoke replaced credential; retain recovery on partial failure; stale-record conflicts report the owning service (or the record's persistence state for own-service records)
```

The supported remote mapping is `loki: swift|s3`, `tempo: s3`, `mimir: swift|s3`, `harbor: s3`, `etcd-backup: s3`, and `velero: s3`; non-remote `loki/etcd-backup/velero: none` and `harbor: filesystem` are validated in the same package but do not call the OpenStack adapter.

## Safe-change boundaries

- Keep provider operation read-only and storage operation explicitly mutating; do not reuse one as the other.
- Preserve raw-source comparison before persistence, public v2 serialization, backups, credential owner resolution, redaction, and recovery state.
- Never expose credential secrets or owner IDs in `Result`, JSON/YAML output, logs, or errors.
- Keep the one-service contract and exit classification; adding a mapping requires config service types, validators, renderer/secret consumers, and contract tests.
- `secrets sync` consumes resulting config later; it does not provision these OpenStack credentials.

## Related maps

[Providers](providers.md) · [Config system](config-system.md) · [Secrets management](secrets-management.md) · [Cluster lifecycle](cluster-lifecycle.md)
