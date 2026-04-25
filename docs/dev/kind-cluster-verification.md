---
id: kind-cluster-verification
title: "Kind Cluster Verification Guide"
sidebar_label: Kind Verification
description: Systematic verification of services deployed on an openCenter Kind cluster.
doc_type: how-to
audience: "platform engineers, developers"
tags: [kind, verification, testing, fluxcd, troubleshooting]
---

# Kind Cluster Verification Guide

**Purpose:** For platform engineers and developers, shows how to systematically verify services deployed on an openCenter Kind cluster, following the FluxCD dependency chain.

## Prerequisites

- A running Kind cluster created via `opencenter cluster deploy`
- `kubectl`, `flux` CLI tools installed
- Access to the cluster's kubeconfig

Set up your environment:

```bash
CLUSTER_NAME="my-cluster"
GITOPS_DIR=$(opencenter cluster describe "$CLUSTER_NAME" 2>/dev/null | grep "git_dir:" | awk '{print $2}')
export KUBECONFIG="$GITOPS_DIR/infrastructure/clusters/$CLUSTER_NAME/kubeconfig.yaml"
```

## Service Dependency Graph

Services must be verified in dependency order. The graph below shows the FluxCD Kustomization dependencies:

```
                    ┌─────────────────┐
                    │   flux-system   │
                    │  (GitRepository)│
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │     sources     │
                    │ (GitRepositories│
                    │  for all svcs)  │
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
┌───────▼───────┐   ┌────────▼────────┐   ┌───────▼───────┐
│  cert-manager │   │   gateway-api   │   │    metallb    │
│    (base)     │   │ (envoy-gateway) │   │    (base)     │
└───────┬───────┘   └────────┬────────┘   └───────┬───────┘
        │                    │                    │
        └───────────┬────────┴────────────────────┘
                    │
           ┌────────▼────────┐
           │     gateway     │
           │ (depends on     │
           │  cert-manager & │
           │  gateway-api)   │
           └────────┬────────┘
                    │
        ┌───────────┴─────────────────────────────┐
        │                                         │
┌───────▼───────┐                         ┌───────▼───────┐
│   headlamp    │                         │observability- │
│               │                         │  namespace    │
└───────────────┘                         └───────┬───────┘
                                                  │
                                          ┌───────▼───────┐
                                          │kube-prometheus│
                                          │    -stack     │
                                          └───────┬───────┘
                                                  │
                                          ┌───────▼───────┐
                                          │     loki      │
                                          └───────────────┘
```

## Verification Phases

### Phase 0: Pre-Flight Checks

Verify cluster infrastructure before checking services.

```bash
# Check nodes
kubectl get nodes
# Expected: All nodes Ready

# Check system pods
kubectl get pods -n kube-system
# Expected: All pods Running

# Check Gitea connectivity (for local Kind clusters)
opencenter local gitea status
# Expected: Running: true, Kind Attached: true
```

| Check | Command | Expected |
|-------|---------|----------|
| Nodes | `kubectl get nodes` | All `Ready` |
| System Pods | `kubectl get pods -n kube-system` | All `Running` |
| Gitea | `opencenter local gitea status` | `Running: true` |
| API Server | `kubectl cluster-info` | Reachable |

### Phase 1: FluxCD Core

Verify FluxCD controllers are operational.

```bash
# Check Flux pods
kubectl get pods -n flux-system

# Check Git source
flux get sources git -n flux-system
# Expected: flux-system READY=True

# Check all kustomizations
flux get kustomizations -n flux-system
```

| Component | Command | Expected |
|-----------|---------|----------|
| Source Controller | `kubectl get deploy source-controller -n flux-system` | `READY 1/1` |
| Kustomize Controller | `kubectl get deploy kustomize-controller -n flux-system` | `READY 1/1` |
| Helm Controller | `kubectl get deploy helm-controller -n flux-system` | `READY 1/1` |
| GitRepository | `flux get sources git -n flux-system` | `READY=True` |

### Phase 2: Sources

Verify all service GitRepositories are ready.

```bash
# Check sources kustomization
flux get kustomization sources -n flux-system

# List all GitRepositories
flux get sources git -A
```

Key sources to verify:
- `opencenter-cert-manager`
- `opencenter-gateway-api`
- `opencenter-metallb`
- `opencenter-headlamp`
- `opencenter-kube-prometheus-stack` (if observability enabled)
- `opencenter-observability` (if observability enabled)

### Phase 3: Infrastructure Services

These services form the foundation for application services.

#### 3.1 cert-manager

```bash
# Kustomization status
flux get kustomization cert-manager-base -n flux-system

# HelmRelease status
flux get helmrelease cert-manager -n cert-manager

# Pod status
kubectl get pods -n cert-manager

# Verify CRDs installed
kubectl get crd certificates.cert-manager.io

# Check cluster issuers
kubectl get clusterissuers
```

| Check | Command | Expected |
|-------|---------|----------|
| Kustomization | `flux get kustomization cert-manager-base` | `READY=True` |
| HelmRelease | `flux get helmrelease cert-manager -n cert-manager` | `READY=True` |
| Pods | `kubectl get pods -n cert-manager` | All `Running` |
| Webhook | `kubectl get validatingwebhookconfigurations cert-manager-webhook` | Exists |

#### 3.2 gateway-api (Envoy Gateway)

```bash
# Kustomization status
flux get kustomization envoy-gateway-api-base -n flux-system

# HelmRelease status
flux get helmrelease envoy-gateway-api -n envoy-gateway-system

# Pod status
kubectl get pods -n envoy-gateway-system

# Verify GatewayClass
kubectl get gatewayclass
```

| Check | Command | Expected |
|-------|---------|----------|
| Kustomization | `flux get kustomization envoy-gateway-api-base` | `READY=True` |
| HelmRelease | `flux get helmrelease envoy-gateway-api -n envoy-gateway-system` | `READY=True` |
| Pods | `kubectl get pods -n envoy-gateway-system` | All `Running` |
| GatewayClass | `kubectl get gatewayclass` | `envoy-gateway` exists |

#### 3.3 metallb

```bash
# Kustomization status
flux get kustomization metallb-base -n flux-system

# HelmRelease status
flux get helmrelease metallb -n metallb-system

# Pod status
kubectl get pods -n metallb-system

# Check IP address pool
kubectl get ipaddresspool -n metallb-system
```

| Check | Command | Expected |
|-------|---------|----------|
| Kustomization | `flux get kustomization metallb-base` | `READY=True` |
| HelmRelease | `flux get helmrelease metallb -n metallb-system` | `READY=True` |
| Pods | `kubectl get pods -n metallb-system` | All `Running` |
| IPAddressPool | `kubectl get ipaddresspool -n metallb-system` | Pool configured |

### Phase 4: Gateway

The gateway depends on cert-manager and gateway-api.

```bash
# Kustomization status
flux get kustomization gateway -n flux-system

# Gateway resource
kubectl get gateway -A

# Check gateway has address
kubectl get gateway -n gateway-system -o wide

# HTTPRoutes
kubectl get httproutes -A

# Envoy proxy pods
kubectl get pods -n gateway-system
```

| Check | Command | Expected |
|-------|---------|----------|
| Kustomization | `flux get kustomization gateway` | `READY=True` |
| Gateway | `kubectl get gateway -A` | `Programmed=True` |
| Address | `kubectl get gateway -n gateway-system -o jsonpath='{.items[0].status.addresses}'` | Has IP |
| Envoy Pods | `kubectl get pods -n gateway-system` | All `Running` |

### Phase 5: Application Services

#### 5.1 headlamp

```bash
# Kustomization status
flux get kustomization headlamp-base -n flux-system

# HelmRelease status
flux get helmrelease headlamp -n headlamp

# Pod status
kubectl get pods -n headlamp

# HTTPRoute
kubectl get httproute -n headlamp
```

Access test:
```bash
kubectl port-forward -n headlamp svc/headlamp 8080:80
# Open http://localhost:8080
```

### Phase 6: Observability Stack

Skip this phase if observability services are disabled.

#### 6.1 observability-namespace

```bash
flux get kustomization observability-namespace -n flux-system
kubectl get namespace observability
```

#### 6.2 kube-prometheus-stack

```bash
# Kustomization status
flux get kustomization kube-prometheus-stack-base -n flux-system

# HelmRelease status
flux get helmrelease kube-prometheus-stack -n observability

# Component pods
kubectl get pods -n observability -l app.kubernetes.io/name=prometheus
kubectl get pods -n observability -l app.kubernetes.io/name=grafana
kubectl get pods -n observability -l app.kubernetes.io/name=alertmanager
```

#### 6.3 loki

```bash
# Kustomization status
flux get kustomization loki-base -n flux-system

# HelmRelease status
flux get helmrelease loki -n observability

# Pods
kubectl get pods -n observability -l app.kubernetes.io/name=loki
```

## Quick Health Check

Run this for a rapid cluster health assessment:

```bash
# All kustomizations should be READY=True
flux get kustomizations -A

# All HelmReleases should be READY=True
flux get helmreleases -A

# Find unhealthy pods
kubectl get pods -A | grep -v Running | grep -v Completed
```

## Troubleshooting

### GitRepository Not Ready

**Symptoms:** `flux get sources git` shows `READY=False`

**Diagnosis:**
```bash
kubectl describe gitrepository flux-system -n flux-system
opencenter local gitea status
```

**Common causes:**
- Gitea not attached to Kind network
- TLS certificate missing host IP as SAN
- Network connectivity issues

**Fix:**
```bash
opencenter cluster deploy my-cluster --container-runtime podman --from-step gitea-attach-kind
```

### HelmRelease Stuck Progressing

**Symptoms:** `flux get helmrelease` shows `READY=Unknown` or `Progressing`

**Diagnosis:**
```bash
kubectl describe helmrelease <name> -n <namespace>
kubectl get events -n <namespace> --sort-by='.lastTimestamp'
```

**Common causes:**
- Helm chart download failed
- Resource constraints on nodes
- Missing CRDs

**Fix:**
```bash
# Force reconciliation
flux reconcile helmrelease <name> -n <namespace>

# Suspend and resume
flux suspend helmrelease <name> -n <namespace>
flux resume helmrelease <name> -n <namespace>
```

### Pods CrashLoopBackOff

**Symptoms:** `kubectl get pods` shows `CrashLoopBackOff`

**Diagnosis:**
```bash
kubectl logs <pod-name> -n <namespace> --previous
kubectl describe pod <pod-name> -n <namespace>
```

**Common causes:**
- Configuration errors in Helm values
- Missing secrets or configmaps
- Resource limits too restrictive

### Gateway No Address

**Symptoms:** Gateway shows no external IP

**Diagnosis:**
```bash
kubectl describe gateway -n gateway-system
kubectl get ipaddresspool -n metallb-system
kubectl logs -n metallb-system -l app.kubernetes.io/name=metallb
```

**Common causes:**
- MetalLB not configured
- IPAddressPool exhausted
- L2Advertisement missing

**Fix:**
```bash
# Check MetalLB configuration
kubectl get ipaddresspool,l2advertisement -n metallb-system -o yaml
```

### Kustomization Dependency Failed

**Symptoms:** Kustomization shows `dependency 'xxx' is not ready`

**Diagnosis:**
```bash
flux get kustomization <dependency-name> -n flux-system
kubectl describe kustomization <dependency-name> -n flux-system
```

**Fix:** Resolve the dependency first, then the dependent kustomization will reconcile automatically.

## Verification Script

Save this script to run a complete verification:

```bash
#!/bin/bash
set -e

CLUSTER_NAME="${1:-my-cluster}"

echo "=========================================="
echo "openCenter Kind Cluster Verification"
echo "Cluster: $CLUSTER_NAME"
echo "=========================================="

GITOPS_DIR=$(opencenter cluster describe "$CLUSTER_NAME" 2>/dev/null | grep "git_dir:" | awk '{print $2}')
export KUBECONFIG="$GITOPS_DIR/infrastructure/clusters/$CLUSTER_NAME/kubeconfig.yaml"

echo ""
echo "=== Phase 0: Pre-Flight ==="
kubectl get nodes
opencenter local gitea status 2>/dev/null || true

echo ""
echo "=== Phase 1: FluxCD Core ==="
kubectl get pods -n flux-system
flux get sources git -n flux-system

echo ""
echo "=== Phase 2: Sources ==="
flux get sources git -A

echo ""
echo "=== Phase 3: Infrastructure ==="
for svc in cert-manager-base envoy-gateway-api-base metallb-base; do
  echo "--- $svc ---"
  flux get kustomization "$svc" -n flux-system 2>/dev/null || echo "Not deployed"
done

echo ""
echo "=== Phase 4: Gateway ==="
flux get kustomization gateway -n flux-system 2>/dev/null || echo "Not deployed"
kubectl get gateway -A 2>/dev/null || echo "No gateways"

echo ""
echo "=== Phase 5: Applications ==="
flux get kustomization headlamp-base -n flux-system 2>/dev/null || echo "Not deployed"

echo ""
echo "=== Summary ==="
echo "--- Kustomizations ---"
flux get kustomizations -A
echo ""
echo "--- HelmReleases ---"
flux get helmreleases -A

echo ""
echo "=========================================="
echo "Verification Complete"
echo "=========================================="
```

## Service Dependency Reference

| Service | Depends On | Namespace |
|---------|------------|-----------|
| sources | flux-system | flux-system |
| cert-manager-base | sources | cert-manager |
| envoy-gateway-api-base | sources | envoy-gateway-system |
| metallb-base | sources | metallb-system |
| gateway | cert-manager-base, envoy-gateway-api-base | gateway-system |
| headlamp-base | sources | headlamp |
| observability-namespace | sources | observability |
| kube-prometheus-stack-base | sources, observability-namespace | observability |
| loki-base | kube-prometheus-stack-base | observability |

## Evidence

- FluxCD Kustomization templates: `internal/gitops/templates/cluster-apps-base/services/fluxcd/`
- Kind bootstrap provider: `internal/cluster/kind_bootstrap_provider.go`
- Service dependency definitions: `internal/gitops/templates/cluster-apps-base/services/fluxcd/*.yaml.tpl`
- Kind defaults: `internal/config/defaults/kind.yaml`
