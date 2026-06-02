---
id: per-service-file-variance
title: "Per-Service File Variance Across RelayPoint Clusters"
sidebar_label: Per-Service File Variance
description: Inventory of per-service file differences across the five RelayPoint cluster overlays, confirming the bounded condition model can express them.
doc_type: reference
audience: "developers, platform engineers"
tags: [services, rendering, descriptors, parity, variance]
---
# Per-Service File Variance Across RelayPoint Clusters

**Purpose:** For developers, inventories the actual per-service file differences across the five RelayPoint cluster overlays (k8s-dev, k8s-dr, k8s-prod, k8s-qa, k8s-uat) and confirms whether the bounded condition model (`equals`, `exists`, `true`, `false`) can express each variance.

## Methodology

Each service directory under `testdata/relaypoint-logistics-shared/applications/overlays/<cluster>/services/` was compared across all five clusters. Files present in some clusters but absent in others are listed below with the condition that controls their inclusion.

## Service-Level Presence Variance

These services are present in some clusters and absent in others. This variance is handled by the `enabled_when` condition on the descriptor or by the service’s `enabled` field in config.

| Service | k8s-dev | k8s-dr | k8s-prod | k8s-qa | k8s-uat | Condition type |
| --- | --- | --- | --- | --- | --- | --- |
| harbor | yes | no | no | no | no | `true` on `opencenter.services.harbor.enabled` |
| etcd-backup | yes | no | no | no | no | `true` on `opencenter.services.etcd-backup.enabled` |
| sealed-secrets | no | no | no | yes | no | `true` on `opencenter.services.sealed-secrets.enabled` |
| kyverno | yes | yes | no | yes | yes | `true` on `opencenter.services.kyverno.enabled` |
| longhorn | no | yes | no | yes | yes | `true` on `opencenter.services.longhorn.enabled` |

All of these are expressible with the `true` operator on the service’s `enabled` field. No new operators needed.

## Per-File Variance Within Services

### cert-manager

| File | Variance | Condition |
| --- | --- | --- |
| `letsencrypt-issuer.yaml` | present in all clusters, aliased to cluster-specific names in fixture | template renders cluster-specific content; no condition needed |
| `opencenter-aws-credentials-secret.yaml` | present only when AWS credentials are configured | `exists` on `secrets.global.aws.application.access_key` |
| `rackspace-selfsigned-ca.yaml` | present in k8s-prod, k8s-uat | `true` on a cert-manager config field |

Expressible with `exists` and `true`.

### keycloak

| File | Variance | Condition |
| --- | --- | --- |
| `20-keycloak/patch-subscription.yaml` | present in clusters using OLM-managed keycloak | `true` on `opencenter.services.keycloak.pin_operator_version` or similar |
| `20-keycloak/rbac-manager-users.yaml` | absent from k8s-dr fixture | fixture drift (ignored in canonicalization) |

Expressible with `true`.

### kube-prometheus-stack

| File | Variance | Condition |
| --- | --- | --- |
| `alertmanager-routes.yaml` | present in all clusters, aliased to `alertmanager-security-policy.yaml` in some | template renders cluster-specific content; alias in canonicalization |

No condition needed; content varies by template rendering.

### velero

| File | Variance | Condition |
| --- | --- | --- |
| `rbac.yaml`, `rbac2.yaml` | present in some clusters | fixture drift (ignored in k8s-dr canonicalization) |

Fixture-specific; not a rendering condition.

### vsphere-csi

| File | Variance | Condition |
| --- | --- | --- |
| `storageclass-delete.yaml` | present in clusters with delete reclaim policy | `equals` on a vsphere-csi config field |
| `storageclass-retain.yaml` | present in clusters with retain reclaim policy | `equals` on a vsphere-csi config field |

Expressible with `equals`.

## Cluster-Scoped Variance

### customer-managed

| Feature | Variance | Condition |
| --- | --- | --- |
| entire directory | absent from k8s-prod | `true` on `opencenter.gitops.overlay_units.customer_managed.enabled` |
| Secret manifest in sources | present only in k8s-uat | `true` on `opencenter.gitops.overlay_units.customer_managed.emit_secret` |

Expressible with `true`.

### .sops.yaml

| Feature | Variance | Condition |
| --- | --- | --- |
| file presence | present in k8s-dev, k8s-dr; absent from k8s-prod, k8s-qa, k8s-uat | `true` on `opencenter.gitops.overlay_units.sops.enabled` |

Expressible with `true`.

## Conclusion

All observed per-service and per-cluster file variance in the RelayPoint fixture is expressible using the current bounded condition model:

* `true`/`false` for boolean service enablement and feature flags
* `exists` for optional credential presence
* `equals` for string-valued configuration choices

No new operators are required. The remaining differences that cannot be expressed as conditions are fixture drift artifacts handled by the canonicalization inventory (aliases, ignores).