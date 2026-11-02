---
last_updated: 2026-10-21
id: kind-cluster-verification
title: "Kind Cluster Verification Guide"
sidebar_label: Kind Cluster Verification Guide
description: Systematic verification of services deployed on an openCenter Kind cluster, using the real FluxCD Kustomization naming and dependency wiring.
doc_type: how-to
audience: "platform engineers, developers"
tags: [kind, verification, testing, fluxcd, troubleshooting]
---
# Kind Cluster Verification Guide

**Purpose:** For platform engineers and developers, shows how to systematically verify services deployed on an openCenter Kind cluster, following the actual FluxCD Kustomization dependency wiring.

## Prerequisites

* A running Kind cluster created via `opencenter cluster deploy`.
* `kubectl` and the `flux` CLI installed.
* Access to the cluster's kubeconfig.
* On ARM hosts, Rosetta emulation for amd64 containers, which Harbor requires (see [Pods `CrashLoopBackOff`](#pods-crashloopbackoff)).

```bash
eval "$(opencenter cluster env "$CLUSTER_NAME")"
```

## How service Kustomizations are actually wired

Every rendered service gets one or two FluxCD Kustomizations named from the service's `KustomizationName` (defaults to the service name): `<name>-base` (renders the shared Helm/manifest base from `openCenter-gitops-base`) and, for services with override values, `<name>-override` (applies the cluster-specific overlay from `applications/overlays/<cluster>/services/<name>/`, and is SOPS-decryption-enabled). Both depend on the `sources` Kustomization by default, plus whatever `ExtraDependencies` the service declares in `internal/gitops/render_catalog.go` (for hand-authored services) or an explicit descriptor (for services under `internal/services/descriptors/data/`). Do not assume a fixed universal dependency tree -- check `render_catalog.go` for the service in question if you need the exact wiring. Verified examples at the time of writing:

* `gateway-base` depends on `envoy-gateway-api-base` (not on cert-manager).
* `kube-prometheus-stack-override` depends on `sources` and `envoy-gateway-api-base` (it renders Gateway API `HTTPRoute` objects).
* `loki-base` / `loki-override` depend on `observability-namespace` and `observability-sources` in addition to `sources`.
* `rbac-manager-base` conditionally depends on `kube-prometheus-stack-base` only when `kube-prometheus-stack` is enabled.
* `kyverno` renders as a base-only Kustomization plus a `kyverno-default-ruleset` post-base stage that depends on `sources` and `kyverno-base`.

## Verified default namespaces

From `internal/config/v2/defaults.go` (`NewDefaultServiceConfig`) -- use these, not guesses, when targeting `kubectl -n`:

| Service | Namespace | Service | Namespace |
| --- | --- | --- | --- |
| calico | `calico-system` | kube-prometheus-stack | `observability` |
| cert-manager | `cert-manager` | kyverno | `kyverno` |
| etcd-backup | `kube-system` | loki | `observability` |
| external-snapshotter | `external-snapshotter` | openstack-ccm | `openstack-ccm` |
| fluxcd, sources, weave-gitops | `flux-system` | openstack-csi | `openstack-csi` |
| **gateway** | **`gateway`** (not `gateway-system`) | tempo | `observability` |
| gateway-api | `envoy-gateway-system` | velero | `velero` |
| headlamp | `headlamp` | harbor | `harbor` |
| keycloak | `keycloak` | metallb | `metallb-system` |
| postgres-operator | `postgres-operator` | olm | `olm` |
| rbac-manager | `rbac-system` | kafka-cluster | `kafka-system` (hardcoded in its templates; the config `namespace` field is not honored) |
| vsphere-csi | `vmware-system-csi` | longhorn | `longhorn-system` |
| mimir, opentelemetry-kube-stack | `observability` | sealed-secrets | `sealed-secrets` |

These are defaults applied when a service has no explicit `namespace` override in config -- always check the rendered manifest or `kubectl get ns` if the cluster config customizes it.

## Verification phases

### Phase 0: Pre-flight

```bash
kubectl get nodes                       # all Ready
kubectl get pods -n kube-system         # all Running
./bin/opencenter-local gitea status     # Running: true (local Kind clusters only)
kubectl cluster-info                    # API server reachable
```

### Phase 1: FluxCD core

```bash
kubectl get pods -n flux-system
flux get sources git -n flux-system     # flux-system READY=True
flux get kustomizations -n flux-system
```

### Phase 2: Sources

```bash
flux get kustomization sources -n flux-system
flux get sources git -A
```

Every enabled service has a corresponding `opencenter-<service>` `GitRepository` (or, for services sharing the observability chart set, `opencenter-observability`) created by the `sources` Kustomization.

### Phase 3: Per-service verification

For any service, the pattern is the same -- substitute the service name and its verified namespace from the table above:

```bash
SERVICE=cert-manager
NAMESPACE=cert-manager

flux get kustomization "${SERVICE}-base" -n flux-system
flux get kustomization "${SERVICE}-override" -n flux-system 2>/dev/null || true
flux get helmrelease "${SERVICE}" -n "${NAMESPACE}" 2>/dev/null || true
kubectl get pods -n "${NAMESPACE}"
```

Service-specific extras worth checking:

* **cert-manager**: `kubectl get crd certificates.cert-manager.io`, `kubectl get clusterissuers`.
* **gateway-api** (Envoy Gateway): `kubectl get gatewayclass`.
* **gateway**: `kubectl get gateway -n gateway`, and confirm it has an address (`kubectl get gateway -n gateway -o jsonpath='{.items[0].status.addresses}'`) -- this requires MetalLB to have allocated an IP.
* **metallb**: `kubectl get ipaddresspool,l2advertisement -n metallb-system`.
* **kube-prometheus-stack**: pods labeled `app.kubernetes.io/name=prometheus|grafana|alertmanager` in `observability`; `HTTPRoute` objects (`prometheus-http-route.yaml`, `alertmanager-http-route.yaml`, `grafana-http-route.yaml`) require `gateway-api` and `gateway` to be healthy first.
* **loki / tempo / mimir**: pods in `observability`, labeled `app.kubernetes.io/name=<service>`.
* **headlamp**: `kubectl port-forward -n headlamp svc/headlamp 8080:80` then open `http://localhost:8080`.

## Quick health check

```bash
flux get kustomizations -A        # all should be READY=True
flux get helmreleases -A          # all should be READY=True
kubectl get pods -A | grep -v Running | grep -v Completed
```

## Troubleshooting

### GitRepository not ready

```bash
kubectl describe gitrepository flux-system -n flux-system
./bin/opencenter-local gitea status
```

Common causes: Gitea not attached to the Kind network, TLS certificate missing the host IP as a SAN, network connectivity. Fix by re-running the attach step:

```bash
opencenter cluster deploy my-cluster --container-runtime podman --from-step gitea-attach-kind
```

(`gitea-attach-kind` is a real step name in the Kind deploy flow -- confirm the exact step list with `opencenter cluster deploy my-cluster --dry-run`.)

### HelmRelease stuck `Progressing` / `Unknown`

```bash
kubectl describe helmrelease <name> -n <namespace>
kubectl get events -n <namespace> --sort-by='.lastTimestamp'
flux reconcile helmrelease <name> -n <namespace>
```

### Pods `CrashLoopBackOff`

```bash
kubectl logs <pod-name> -n <namespace> --previous
kubectl describe pod <pod-name> -n <namespace>
```

Harbor core, exporter, jobservice, registry and Trivy crash with `fatal error: lfstack.push` and an `asm_amd64.s` stack trace on ARM hosts that run the amd64 `goharbor/*` images without Rosetta. Enable Rosetta for amd64 emulation in the container runtime (Docker Desktop, or Colima with `--vm-type vz --vz-rosetta`), then recreate the Kind cluster.

### Gateway has no address

```bash
kubectl describe gateway -n gateway
kubectl get ipaddresspool -n metallb-system
kubectl logs -n metallb-system -l app.kubernetes.io/name=metallb
```

Common cause: MetalLB not enabled/configured, or its `IPAddressPool` is exhausted.

### Kustomization dependency failed

```bash
flux get kustomization <dependency-name> -n flux-system
kubectl describe kustomization <dependency-name> -n flux-system
```

Resolve the failing dependency first -- the dependent Kustomization reconciles automatically once its `dependsOn` targets report ready.

## Implementation references

* Auto-generated Kustomization/source templates and naming convention (`<service>-base`, `<service>-override`): `internal/gitops/auto_descriptor.go`.
* Explicit built-in render specs and per-service `ExtraDependencies`/`OverrideDependsOn`/`ConditionalDependencies`: `internal/gitops/render_catalog.go`.
* Default namespaces: `internal/config/v2/defaults.go` (`NewDefaultServiceConfig`).
* Hand-authored descriptors and templates (cert-manager, keycloak, harbor, kafka-cluster, olm, calico, etcd-backup): `internal/services/descriptors/data/`, `internal/gitops/templates/cluster-apps-base/services/`.
* Kind bootstrap steps: `internal/cluster/kind_bootstrap_provider.go`.
