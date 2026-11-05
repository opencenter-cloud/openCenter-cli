---
id: deployment-recommendations
title: Deployment recommendations
sidebar_label: Deployment recommendations
description: Analysis of cluster deploy behavior with optimization recommendations for OIDC bootstrapping.
doc_type: explanation
audience: openCenter operators
tags: [deployment, oidc, keycloak]
last_updated: 2026-10-24
---
# openCenter Cluster Deployment: Current Architecture, Commit Analysis & Optimization Recommendations

**Purpose:** For openCenter operators, explains analysis of cluster deploy behavior with optimization recommendations for OIDC bootstrapping.

This document reviews the `opencenter cluster deploy` command, analyzes the latest commits regarding OIDC confidential secrets and `gitops-base` tagging, and provides actionable recommendations to accelerate deployment and eliminate OIDC-related bootstrapping failures.

---

## 1. Executive Summary & Review of Recent Commits

### Recent Commit Analysis

1. **Commit `859c908` (`chore(gitops): bump gitops-base release tag to 2026.03-rc3`)**:
   * **What changed**: Updated default release pins (`defaultGitBaseRepoRelease`) and Terraform module sources across `main-default.tf.tpl`, `main-baremetal.tf.tpl`, and `main-vmware.tf.tpl` for Calico, Cilium, Kube-OVN, and OpenStack Nova to tag `2026.03-rc3`.
   * **Significance**: Keeps the core infrastructure and CNI modules aligned with the tested release while preserving `kubespray-cluster` pinned to `2026.10.21-contract2` (enforcing lifecycle contract v2 so `inventory_output_dir` renders inside `.opencenter-state` rather than dirtying the GitOps worktree).

2. **Commit `aef0fadb` (`fix(templates): sync project-argus templates, OIDC confidential secrets, and observability`)**:
   * **What changed**:
     * Automatically generates a cryptographically random 32-character client secret during `opencenter cluster init` (`secrets.keycloak.client_secret`) and propagates it to `secrets.headlamp.oidc_client_secret`.
     * Renders this secret directly into `opencenter-realm.yaml.tpl` under the `opencenter` OIDC client.
     * Wires the secret into `headlamp` override values, `grafana-oauth-secret`, `gateway-oidc-secret`, and Envoy Gateway `SecurityPolicy` resources (`alertmanager-oidc`, `prometheus-oidc`).
     * Added CLI backend support for `opencenter secrets set keycloak-client-secret`.
   * **Significance**: Addresses the requirement for a pre-shared, random OIDC client secret before Day 1 deployment starts.

### Answer to the OIDC Deployment Question

> **Question**: *Can Keycloak with the opencenter realm be set with a random OIDC client secret so all other services can be deployed and not fail?*

**Yes—and commit `aef0fadb` put the secret generation and manifest rendering into place.** However, having the client secret pre-configured in manifests does **not** by itself prevent downstream services from failing during bootstrap. 

Services still fail or enter `CrashLoopBackOff` when deployed alongside Keycloak because:
1. **OIDC Discovery on Container Startup**: Envoy Gateway's `SecurityPolicy` filter, Headlamp, and Grafana make synchronous HTTP calls to `https://auth.<cluster_fqdn>/realms/opencenter/.well-known/openid-configuration` and `jwks_uri` on startup. If Keycloak has not finished deploying and importing the realm, these calls fail with connection refused or HTTP 404/503.
2. **Keycloak Deployment Latency**: Keycloak currently takes **6 to 12 minutes** to become ready because it waits on PostgreSQL (Zalando Spilo), OLM operator installation, unoptimized container startup (`startOptimized: false`), and `KeycloakRealmImport` reconciliation.
3. **External DNS & Hairpin Routing**: During initial bootstrap, external DNS records and external Gateway load balancers are not yet active. In-cluster pods trying to reach `https://auth.<cluster_fqdn>` fail DNS resolution unless in-cluster rewrites exist.
4. **Missing Dependency Staging in Flux**: `headlamp` and `kube-prometheus-stack` do not depend on `keycloak-cr` in Flux, so Flux reconciles them immediately and fails their readiness health checks.

---

## 2. Table: What `opencenter cluster deploy` Does Today

| Phase | Step ID / Component | Scope / Provider | What It Does Today | Underlying Tool / Mechanism | Bottlenecks / Duration |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Phase 1: Pre-flight & Pre-checks** | `checkProviderAvailability`, `checkDeployPrerequisites` | All | Validates local CLI binaries (`tofu`, `kubectl`, `git`, container runtime) and resolves cluster paths. | Go CLI checks | 1–3s |
| | `AcquireLockWithPrompt` | All | Acquires exclusive distributed lock in `<state_dir>/locks/<org>/<cluster>/deploy.lock`. | File lock manager | < 1s |
| | `checkSecretsSynced` (OCTR-787) | All | Verifies all required secret manifests exist in the GitOps overlay and are encrypted via SOPS before changing infrastructure. | SOPS manifest validator | 1–2s |
| | Git Remote & Status Check | All | Re-homes Git URL for local network reachability (e.g. Kind/Podman), checks for uncommitted changes, and aligns remote origin. | `git` CLI | 1–3s |
| | `validateBootstrapConfig` | All | Validates v2 schema, provider settings, and Harbor deployment readiness. | Go internal validator | < 1s |
| **Phase 2: Infrastructure Provisioning** | `preflight` | OpenStack | Verifies OpenStack credentials (`clouds.yaml`), project quotas, SSH keys, and network connectivity. | Gophercloud / OpenStack API | 3–10s |
| | `opentofu-init` | OpenStack / Cloud | Runs `tofu init` in `infrastructure/<cluster>`. Marked `NeverSkip: true`. | `tofu init` | 5–20s (downloads providers) |
| | `opentofu-apply` | OpenStack / Cloud | Applies OpenTofu plan; creates VMs, networks, subnets, routers, security groups, and floating VIPs. Takes Calico GitOps safety snapshots and renders inventory to `.opencenter-state/inventory`. | `tofu apply -auto-approve` | 2–5 min |
| | `kind-create` | Kind (Local) | Creates local Kind cluster using generated `kind-config.yaml`. | `kind create cluster` | 30–60s |
| | `magnum-create` | Magnum | Submits cluster creation request to OpenStack Magnum API with durable identity state. | Magnum REST API | 5–15s |
| **Phase 3: Kubernetes Deployment** | `kubespray-prepare` | OpenStack (Kubespray) | Clones Kubespray repository from GitHub, builds a Python venv (`pip install -r requirements.txt`), and clones `ansible-hardening`. | `git clone`, `python3 -m venv`, `pip` | **2–4 min** (network dependent) |
| | `kubespray-wait-cloudinit` | OpenStack (Kubespray) | Polls all cluster VMs via Ansible ad-hoc shell until `cloud-init status --wait` completes. | `ansible -m shell` | **1–3 min** |
| | `kubespray-os-hardening` | OpenStack (Kubespray) | (Optional) Applies Ansible OS hardening playbook if configured. | `ansible-playbook` | **3–6 min** |
| | `kubespray-deploy` | OpenStack (Kubespray) | Executes main Kubespray playbook `cluster.yml` with `-f 10` forks: installs containerd, etcd, Kubernetes control plane, kubelet, and base manifests. | `ansible-playbook cluster.yml` | **15–25 min** (Major bottleneck) |
| | `kubespray-export-kubeconfig` / `kind-export-kubeconfig` / `magnum-export-kubeconfig` | All | Fetches admin kubeconfig (`admin.conf`) from control plane or cloud API to local disk. | Ansible `fetch` / Kind / Magnum API | 2–5s |
| **Phase 4: Node & Network Preparation** | `openstack-normalize-kubeconfig` | OpenStack | Replaces internal/localhost IP in kubeconfig with public API VIP / Floating IP. | Go regex / file write | < 1s |
| | `network-plugin-install` / `kind-install-cni` | OpenStack / Kind | In managed CNI mode, applies Calico operator CRDs and upgrades/installs the Calico Helm chart before Flux starts. | `kubectl apply`, `helm upgrade --install` | 1–2 min |
| | `coredns-toleration` | OpenStack | Patches CoreDNS deployment with toleration for `node.cloudprovider.kubernetes.io/uninitialized` so DNS resolves before cloud CCM initializes. | `kubectl patch` | 2–5s |
| **Phase 5: GitOps & Secrets Ingestion** | `flux-bootstrap` | All | Bootstraps FluxCD controllers (source, kustomize, helm, notification) in `flux-system` and commits manifests to Git repository. | `flux bootstrap [git/gitea/github]` | **1–3 min** |
| | `sops-age-secret` | All | Injects Age private key into `flux-system/sops-age` Secret so in-cluster Flux can decrypt encrypted Helm values and manifests. | `kubectl apply` | 1–2s |
| | `grafana-admin-secret` | All | Imperatively creates `observability/grafana-admin-password` Secret to prevent Grafana from failing container config checks. | `kubectl apply` | 1–2s |
| | `gitea-rebase` & `gitops-push` | Kind | Rebases local Git checkout against in-cluster/local Gitea and pushes overlay tree. | `git pull --rebase`, `git push` | 5–10s |
| **Phase 6: Cluster Readiness Verification** | `waitForReady` | All | Polls Kubernetes API endpoint until nodes report `Ready` and base control plane responds. Removes resume state file on completion. | Kubernetes Client-Go polling | 10s–2 min |
| **Phase 7: Downstream Flux Reconciliation** | Foundation Stages | In-Cluster (Flux) | Flux reconciles: `sources` → `cert-manager` → `envoy-gateway-api` → `gateway` → `postgres-operator` → `olm`. | Flux Kustomization controller | 3–6 min |
| | Keycloak Stages | In-Cluster (Flux) | 1. `keycloak-postgres`: Zalando Spilo cluster creation.<br>2. `keycloak-operator`: OLM subscription & CSV install.<br>3. `keycloak-cr`: Keycloak 3-replica StatefulSet + `KeycloakRealmImport`.<br>4. `oidc-rbac`: RBACManager rules. | Flux Kustomization & HelmRelease controllers | **6–12 min** (Keycloak bottleneck) |
| | Observability & Apps | In-Cluster (Flux) | Flux reconciles `kube-prometheus-stack`, `loki`, `tempo`, `headlamp`. Services with OIDC enabled attempt to bind to Keycloak. | Flux Kustomization & HelmRelease controllers | 3–8 min (Fails if Keycloak is down) |

---

## 3. Table: Optimization Recommendations & Proposed Updates

| Area | Current Problem / Bottleneck | Proposed Optimization / Update | Estimated Time Saved | Complexity |
| :--- | :--- | :--- | :--- | :--- |
| **1. OIDC & Keycloak Decoupling** | Headlamp, Envoy Gateway `SecurityPolicy`, and Grafana fail on boot because `auth.<fqdn>` discovery is unreachable while Keycloak provisions. | **a. Stage Envoy Gateway `SecurityPolicy`**: Move `alertmanager-oidc` and `prometheus-oidc` into a post-Keycloak stage (`oidc-policies`) that depends on `keycloak-cr`. Routes deploy immediately and become protected once Keycloak is healthy.<br>**b. In-Cluster CoreDNS Rewrite**: Add `rewrite name exact auth.<cluster_fqdn> envoy-gateway.gateway.svc.cluster.local` to CoreDNS so internal pods bypass external DNS/hairpin routing.<br>**c. Soften Headlamp Startup Probes**: Configure Headlamp startup probe with higher failure threshold and backoff so it does not CrashLoop while waiting for OIDC discovery. | Eliminates 5–15 min cascading failures and manual retries | Medium |
| **2. Keycloak Startup Optimization** | Keycloak CR sets `startOptimized: false` and runs 3 instances on Day 0. Every pod performs runtime Quarkus augmentation (60–90s each) while Spilo PostgreSQL initializes. | **a. Enable `startOptimized: true`**: Use a pre-built Keycloak image with PostgreSQL drivers pre-compiled.<br>**b. Single Replica Bootstrap**: Set `instances: 1` during initial bootstrap to reduce Spilo DB lock contention and memory footprint, scaling to 3 replicas post-deploy.<br>**c. Direct Operator Manifests**: Replace OLM subscription for Keycloak with direct Helm chart or static manifests to eliminate OLM CSV resolution delay (saves ~2m). | **Saves 5–8 minutes** on Keycloak readiness | Medium |
| **3. Kubespray Provisioning Acceleration** | Kubespray runs `cluster.yml` from scratch on blank OS images, downloading binaries and images per node over WAN. Clones repos and builds Python venv on host. | **a. Golden Node Images (Packer)**: Pre-bake containerd, kubelet, kubeadm, and pre-pull core Kubernetes pause/control plane images into the glance image.<br>**b. Containerized Kubespray Runner**: Replace local `python3 -m venv` + `pip install` with a pre-built `quay.io/kubespray/kubespray` container.<br>**c. Increase Ansible Concurrency**: Increase fork count from `-f 10` to `-f 30`. | **Saves 10–15 minutes** (Cuts Kubespray from ~20m to ~6m) | High |
| **4. FluxCD Reconciliation Concurrency** | Foundation Kustomizations serialize unnecessarily and use conservative 1m retry / 15m reconciliation intervals. | **a. Flatten Foundation Dependencies**: Allow `cert-manager`, `envoy-gateway-api`, and `postgres-operator` to reconcile in parallel (they have no interdependencies).<br>**b. Bootstrap Retry Interval**: Set `retryInterval: 15s` during bootstrap so temporary dependency failures re-check immediately rather than waiting 60s per attempt. | **Saves 2–4 minutes** | Low |
| **5. Preflight & Infrastructure Improvements** | Preflights run sequentially. `tofu init` runs without local provider caching. | **a. Concurrent Preflight Checks**: Run OpenStack quota checks, SSH key validation, and secret verification in parallel goroutines.<br>**b. Enable Provider Cache**: Export `TF_PLUGIN_CACHE_DIR` to avoid re-downloading OpenTofu provider binaries across runs. | **Saves 30–60s** | Low |

---

## 4. Implementation Blueprints

### Blueprint A: Staging Envoy Gateway OIDC SecurityPolicies Behind Keycloak

In `internal/gitops/templates/cluster-apps-base/services/fluxcd/keycloak.yaml.tpl`, add a dedicated stage for OIDC security policies:

```yaml
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: oidc-security-policies
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: keycloak-cr
      namespace: flux-system
    - name: kube-prometheus-stack-override
      namespace: flux-system
  interval: 15m
  retryInterval: 30s
  timeout: 5m
  sourceRef:
    kind: GitRepository
    name: opencenter-keycloak-config
    namespace: flux-system
  path: applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/observability/oidc-policies
  targetNamespace: observability
  prune: true
  wait: true
```

*Effect*: `kube-prometheus-stack`, Prometheus, Alertmanager, and Grafana HTTPRoutes deploy and become ready immediately. Once Keycloak is healthy and imports the `opencenter` realm, the SecurityPolicies attach authentication seamlessly.

### Blueprint B: In-Cluster CoreDNS Rewrite for Keycloak FQDN

Add a CoreDNS rewrite entry in CoreDNS Corefile so that in-cluster pods can resolve `auth.<cluster_fqdn>` without depending on external DNS:

```text
rewrite name exact auth.{{ .ClusterFQDN }} envoy-gateway.gateway.svc.cluster.local
```

*Effect*: In-cluster services resolve Keycloak through Envoy Gateway as soon as internal networking is established, preventing external DNS timeout issues during bootstrap.

### Blueprint C: Keycloak Custom Resource Optimization

Update `applications/base/services/keycloak/20-keycloak/keycloak-cr.yaml`:

```yaml
apiVersion: k8s.keycloak.org/v2alpha1
kind: Keycloak
metadata:
  name: keycloak
  namespace: keycloak
spec:
  startOptimized: true
  instances: 1
  image: quay.io/keycloak/keycloak:26.7.5
  db:
    vendor: postgres
    usernameSecret:
      name: keycloak.postgres-cluster.credentials.postgresql.acid.zalan.do
      key: username
    passwordSecret:
      name: keycloak.postgres-cluster.credentials.postgresql.acid.zalan.do
      key: password
    url: jdbc:postgresql://postgres-cluster.keycloak.svc.cluster.local:5432/keycloak
  http:
    httpEnabled: true
  hostname:
    strict: false
    backchannelDynamic: true
```

*Effect*: Reduces Keycloak pod startup time from ~90 seconds to ~8 seconds and prevents DB connection storms against the new PostgreSQL cluster. Scale to 3 replicas via HPA or post-deploy after bootstrap finishes.
