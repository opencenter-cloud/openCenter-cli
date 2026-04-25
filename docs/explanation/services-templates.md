---
id: services-templates
title: "Service Templates: How They Work"
sidebar_label: Service Templates
description: How openCenter generates, deploys, and manages platform service configurations using the cert-manager service as a worked example.
doc_type: explanation
audience: "platform engineers, operators"
tags: [services, templates, cert-manager, fluxcd, gitops, kustomize]
---

**Purpose:** For platform engineers, explains how the openCenter service template system generates cluster-specific GitOps manifests from embedded Go templates, and how FluxCD reconciles them against the gitops-base repository. Uses cert-manager as the primary worked example.

## Concept Summary

Every openCenter-managed cluster runs a set of platform services (cert-manager, Kyverno, MetalLB, Keycloak, etc.). These services share a common deployment pattern:

1. The openCenter-cli reads a cluster configuration YAML file.
2. For each enabled service, the CLI renders a set of embedded Go templates into the customer's GitOps repository.
3. FluxCD on the cluster reconciles those rendered manifests, pulling base definitions from `openCenter-gitops-base` and applying cluster-specific overrides from the customer repository.

The result is a two-tier Kustomize overlay model: a shared base (maintained centrally) composed with per-cluster overrides (generated per customer).

## How It Works

### The Three Layers

Every service deployment involves three distinct layers, each owned by a different artifact:

| Layer | Owner | Location | Contains |
|---|---|---|---|
| Base manifests | `openCenter-gitops-base` repo | `applications/base/services/<service>/` | HelmRelease, HelmRepository, namespace, hardened Helm values |
| Overlay manifests | Customer GitOps repo | `applications/overlays/<cluster>/services/<service>/` | Issuers, secrets, Helm value overrides, custom resources |
| FluxCD wiring | Customer GitOps repo | `applications/overlays/<cluster>/services/fluxcd/` and `sources/` | GitRepository sources, Kustomization resources that connect base → overlay |

### Template Rendering Pipeline

When you run `opencenter cluster generate <cluster-name>`, the CLI executes a multi-stage pipeline. The `ServiceStage` handles service template generation:

```
Cluster Config (.k8s-*-config.yaml)
        │
        ▼
┌─────────────────────┐
│  1. Read enabled     │  Iterates .OpenCenter.Services map,
│     services         │  collects names where Enabled: true
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  2. Resolve          │  Orders services by dependency graph
│     dependencies     │  (e.g., cert-manager before gateway)
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  3. Match templates  │  Finds embedded .tpl files tagged
│     to services      │  for each enabled service
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  4. Evaluate         │  Checks render conditions (provider
│     conditions       │  type, feature flags, etc.)
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  5. Render & write   │  Go template engine + Sprig functions
│     output files     │  writes to customer repo
└─────────────────────┘
```

The template engine uses Go's `text/template` with Sprig function support, an LRU cache for repeated renders, and template composition for shared partials. All templates are compiled into the CLI binary via `go:embed`.

Source: `openCenter-cli/internal/gitops/embed.go`
```go
//go:embed all:gitops-base-dir all:templates
var Files embed.FS
```

Source: `openCenter-cli/internal/gitops/stages/service_stage.go` — `Execute()` method.

### Cluster Configuration: The Input

Each cluster has a configuration file (`.k8s-<env>-config.yaml`) that drives template rendering. The services section controls which services are enabled and provides service-specific parameters.

For cert-manager, the relevant config block looks like this:

```yaml
# From customers/1643323-Federal-Farm-Credit/.k8s-dev-config.yaml
opencenter:
  services:
    cert-manager:
      enabled: true
      email: mpk-support@rackspace.com
      region: us-east-1
```

The Go config struct backing this (`internal/config/services/cert_manager.go`):

```go
type CertManagerConfig struct {
    BaseConfig          `yaml:",inline"`
    LetsEncryptServer   string       `yaml:"letsencrypt_server"`
    Email               string       `yaml:"email"`
    Region              string       `yaml:"region"`
    DNSZones            []string     `yaml:"dns_zones"`
    CreateClusterIssuer bool         `yaml:"create_cluster_issuer"`
    Issuers             []CertIssuer `yaml:"issuers"`
    DNSProvider         string       `yaml:"dns_provider"`
}
```

These fields become available in templates as `.OpenCenter.Services["cert-manager"].<Field>`.

## Cert-Manager: A Complete Walkthrough

### Embedded Templates (CLI Side)

The cert-manager templates live in `openCenter-cli/internal/gitops/templates/cluster-apps-base/services/`. The CLI generates three categories of output files from these templates:

#### Category 1: GitRepository Source

Template: `services/sources/opencenter-cert-manager.yaml.tpl`

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-cert-manager
  namespace: flux-system
spec:
  interval: 15m
  {{- $service := index .OpenCenter.Services "cert-manager" }}
  url: {{ $service.Uri | default .OpenCenter.GitOps.GitOpsBaseRepo }}
  ref:
    branch: {{ $service.Branch | default .OpenCenter.GitOps.GitOpsBranch | default "main" }}
  secretRef:
    name: opencenter-base
```

This tells FluxCD where to find the base cert-manager manifests. The URL defaults to the `openCenter-gitops-base` repository but can be overridden per-service (useful for testing forks).

Rendered output (Uniphore example):
```yaml
# customers/6427159-Uniphore/applications/overlays/dev/services/sources/opencenter-cert-manager.yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-cert-manager
  namespace: flux-system
spec:
  interval: 15m
  url: https://github.com/rackerlabs/openCenter-gitops-base.git
  ref:
    branch: main
```

Note: Some customers use SSH URLs with `secretRef` for private repo access (Federal Farm Credit), while others use HTTPS (Uniphore). The template handles both via the config.

#### Category 2: FluxCD Kustomizations (The Wiring)

Template: `services/fluxcd/cert-manager.yaml.tpl`

This is the most important template. It generates two FluxCD Kustomization resources that implement the base+overlay pattern:

```yaml
# Kustomization 1: Deploy base from gitops-base repo
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: cert-manager-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources          # Wait for GitRepository sources to be ready
  sourceRef:
    kind: GitRepository
    name: opencenter-cert-manager    # Points to gitops-base
  path: applications/base/services/cert-manager  # Path INSIDE gitops-base
  targetNamespace: cert-manager
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: cert-manager
      namespace: cert-manager
---
# Kustomization 2: Apply cluster-specific overrides
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: cert-manager-override
  namespace: flux-system
spec:
  dependsOn:
    - name: cert-manager-base    # Wait for base to be healthy
  decryption:
    provider: sops
    secretRef:
      name: sops-age             # SOPS key for decrypting secrets
  sourceRef:
    kind: GitRepository
    name: flux-system            # Points to THIS customer repo
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/cert-manager
  targetNamespace: cert-manager
  prune: true
  wait: true
```

The `{{ .OpenCenter.Cluster.ClusterName }}` token is the only dynamic part. It resolves to the cluster name from config (e.g., `k8s-dev`, `dev`, `k8s-sandbox`).

The dependency chain is: `sources` → `cert-manager-base` → `cert-manager-override`. This ordering guarantees the base HelmRelease is healthy before overrides are applied.

#### Category 3: Service Overlay Resources

These templates generate the cluster-specific resources that live in the overlay directory:

| Template | Output | Purpose |
|---|---|---|
| `rackspace-selfsigned-issuer.yaml` | Static copy | Creates a self-signed Issuer for bootstrapping the CA chain |
| `rackspace-selfsigned-ca.yaml.tpl` | Rendered | Certificate resource for the internal CA; `commonName` set from `.OpenCenter.Cluster.BaseDomain` |
| `rackspace-ca-issuer.yaml` | Static copy | ClusterIssuer that uses the self-signed CA |
| `letsencrypt-issuer.yaml.tpl` | Rendered | ClusterIssuer for Let's Encrypt with Route53 DNS-01 validation |
| `opencenter-aws-credentials-secret.yaml.tpl` | Rendered + SOPS encrypted | AWS credentials for Route53 access |
| `helm-values/override-values.yaml` | Static copy (empty) | Placeholder for Helm value overrides |
| `kustomization.yaml.tpl` | Rendered | Kustomize manifest listing all overlay resources |
| `README.md` | Static copy | Service documentation placeholder |

The self-signed CA template shows how config values flow into resources:

```yaml
# rackspace-selfsigned-ca.yaml.tpl
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: rackspace-selfsigned-ca
spec:
  isCA: true
  commonName: {{ .OpenCenter.Cluster.BaseDomain | default "rmpk.dev" }}
  secretName: rackspace-root-secret
  duration: 87600h0m0s       # 10 years
  renewBefore: 360h0m0s      # 15 days
  privateKey:
    algorithm: ECDSA
    size: 256
  issuerRef:
    name: rackspace-selfsigned-issuer
    kind: Issuer
```

The Let's Encrypt issuer template pulls multiple config fields:

```yaml
# letsencrypt-issuer.yaml.tpl
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-{{ .OpenCenter.Cluster.ClusterName }}
spec:
  acme:
    server: {{ (index .OpenCenter.Services "cert-manager").LetsEncryptServer | default "https://acme-v02.api.letsencrypt.org/directory" }}
    email: {{ (index .OpenCenter.Services "cert-manager").Email | default "mpk-support@rackspace.com" }}
    privateKeySecretRef:
      name: letsencrypt-dns01
    solvers:
      - dns01:
          route53:
            region: {{ (index .OpenCenter.Services "cert-manager").Region }}
            accessKeyIDSecretRef:
              name: "opencenter-aws-credentials-secret"
              key: access-key-id
            secretAccessKeySecretRef:
              name: "opencenter-aws-credentials-secret"
              key: secret-access-key
        selector:
          dnsZones:
            - {{ .OpenCenter.Cluster.ClusterFQDN }}
```

### The Kustomization Manifest (Overlay Glue)

The overlay `kustomization.yaml.tpl` ties all overlay resources together and creates a Kubernetes Secret from the Helm override values:

```yaml
# Rendered output for Federal Farm Credit k8s-dev
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: cert-manager
resources:
  - "./rackspace-selfsigned-issuer.yaml"
  - "./rackspace-selfsigned-ca.yaml"
  - "./rackspace-ca-issuer.yaml"
  - "./opencenter-aws-credentials-secret.yaml"
  - "./letsencrypt-k8s-dev.yaml"
secretGenerator:
  - name: cert-manager-values-override
    type: Opaque
    files: [override.yaml=helm-values/override-values.yaml]
    options:
      disableNameSuffixHash: true
```

The `secretGenerator` creates a Kubernetes Secret named `cert-manager-values-override` containing the Helm override values. The base HelmRelease in `openCenter-gitops-base` references this Secret via `valuesFrom`, allowing the overlay to inject custom Helm values without modifying the base.

### Conditional Rendering

The `sources/kustomization.yaml.tpl` and `fluxcd/kustomization.yaml.tpl` templates use Go conditionals to include only enabled services:

```yaml
# sources/kustomization.yaml.tpl (excerpt)
resources:
{{- if (index $services "cert-manager").Enabled }}
  - "opencenter-cert-manager.yaml"
{{- end }}
{{- if (index $services "kyverno").Enabled }}
  - "opencenter-kyverno.yaml"
{{- end }}
```

The `ServiceStage` also supports render conditions evaluated at the template level. Conditions can check infrastructure provider, service enablement, or arbitrary config fields:

```go
// From service_stage.go
switch condition.Type {
case template.ConditionTypeEquals:
    return fieldValue == condition.Value
case template.ConditionTypeExists:
    return fieldValue != nil && fieldValue != ""
// ...
}
```

This allows templates to be conditionally rendered based on provider type (e.g., vSphere CSI templates only render when `provider: vmware`).

### Service Plugin Validation

Before templates are rendered, the cert-manager plugin (`internal/services/plugins/cert_manager.go`) validates the configuration:

```go
func (p *CertManagerPlugin) validate(config interface{}) error {
    cfg, ok := config.(*services.CertManagerConfig)
    if !ok {
        return fmt.Errorf("invalid config type for cert-manager")
    }
    if cfg.IsEnabled() {
        if cfg.LetsEncryptServer != "" && !strings.HasPrefix(cfg.LetsEncryptServer, "https://") {
            return fmt.Errorf("letsencrypt_server must be an HTTPS URL")
        }
        if cfg.Email != "" && !strings.Contains(cfg.Email, "@") {
            return fmt.Errorf("email must be a valid email address")
        }
    }
    return nil
}
```

Validation runs before rendering. If it fails, the pipeline stops and no files are written.

## Generated Output Structure

After `opencenter cluster generate`, the customer repository contains this structure for cert-manager:

```
applications/overlays/<cluster>/
├── kustomization.yaml                          # Top-level: includes flux-system + services/fluxcd
│
├── services/
│   ├── sources/
│   │   ├── kustomization.yaml                  # Lists all GitRepository sources
│   │   └── opencenter-cert-manager.yaml        # GitRepository → gitops-base
│   │
│   ├── fluxcd/
│   │   ├── kustomization.yaml                  # Lists all FluxCD Kustomizations
│   │   ├── sources.yaml                        # Kustomization for sources/
│   │   └── cert-manager.yaml                   # cert-manager-base + cert-manager-override
│   │
│   └── cert-manager/
│       ├── kustomization.yaml                  # Lists overlay resources + secretGenerator
│       ├── rackspace-selfsigned-issuer.yaml    # Issuer (self-signed bootstrap)
│       ├── rackspace-selfsigned-ca.yaml        # Certificate (internal CA)
│       ├── rackspace-ca-issuer.yaml            # ClusterIssuer (CA-based)
│       ├── letsencrypt-<cluster>.yaml          # ClusterIssuer (Let's Encrypt + Route53)
│       ├── opencenter-aws-credentials-secret.yaml  # SOPS-encrypted AWS creds
│       ├── helm-values/
│       │   └── override-values.yaml            # Helm value overrides (empty by default)
│       └── README.md
```

## FluxCD Reconciliation Flow

Once FluxCD is bootstrapped on the cluster, reconciliation follows this dependency chain:

```
flux-system (GitRepository)
    │
    ▼
top-level kustomization.yaml
    │
    ├── flux-system/           (FluxCD components)
    ├── services/fluxcd/       (all service Kustomizations)
    └── managed-services/fluxcd/
         │
         ▼
    sources (Kustomization)
         │  Deploys all GitRepository resources
         │  including opencenter-cert-manager
         │
         ▼
    cert-manager-base (Kustomization)
         │  Pulls from openCenter-gitops-base
         │  path: applications/base/services/cert-manager
         │  Deploys: HelmRelease, HelmRepository, namespace
         │  Waits for HelmRelease health check
         │
         ▼
    cert-manager-override (Kustomization)
         │  Pulls from customer repo (flux-system GitRepository)
         │  path: ./applications/overlays/<cluster>/services/cert-manager
         │  Deploys: Issuers, CA cert, AWS secret, Helm overrides
         │  Decrypts SOPS-encrypted secrets using Age key
         │
         ▼
    cert-manager running in cluster
         │  HelmRelease manages the cert-manager Helm chart
         │  Issuers and ClusterIssuers ready for certificate requests
```

The `dependsOn` fields enforce ordering. If `cert-manager-base` fails its health check (the HelmRelease isn't ready), `cert-manager-override` won't be applied.

## How Other Services Consume Cert-Manager

Once cert-manager is running, other services reference its Issuers and ClusterIssuers to obtain TLS certificates.

### Gateway / Ingress

The gateway service creates Gateway resources that reference cert-manager for TLS:

```yaml
# From a gateway template
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: opencenter-gateway
  annotations:
    cert-manager.io/cluster-issuer: rackspace-ca  # References the CA ClusterIssuer
spec:
  listeners:
    - name: https
      protocol: HTTPS
      port: 443
      tls:
        certificateRefs:
          - name: gateway-tls-cert
```

### Keycloak

Keycloak's HTTPRoute and TLS configuration depend on certificates issued by cert-manager:

```yaml
# Keycloak HTTPRoute references a cert-manager-issued certificate
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: keycloak
spec:
  parentRefs:
    - name: opencenter-gateway  # Gateway with cert-manager TLS
```

### Observability Stack (Grafana, Prometheus, Alertmanager)

The kube-prometheus-stack overlay creates HTTPRoutes for Grafana, Prometheus, and Alertmanager dashboards. These route through the Gateway, which terminates TLS using cert-manager certificates.

### Headlamp

The cluster UI (Headlamp) uses an HTTPRoute that relies on the same Gateway + cert-manager chain for HTTPS access.

## The Pattern Applied to All Services

Every service in the template directory follows the same three-file pattern:

| File | Location | Role |
|---|---|---|
| `opencenter-<service>.yaml.tpl` | `services/sources/` | GitRepository pointing to gitops-base |
| `<service>.yaml.tpl` | `services/fluxcd/` | Two Kustomizations: base + override |
| `services/<service>/` directory | `services/<service>/` | Overlay resources, Helm overrides, secrets |

The full list of services using this pattern:

```
openCenter-cli/internal/gitops/templates/cluster-apps-base/services/
├── calico/              ├── loki/
├── cert-manager/        ├── longhorn/
├── etcd-backup/         ├── metallb/
├── gateway/             ├── olm/
├── gateway-api/         ├── openstack-ccm/
├── headlamp/            ├── openstack-csi/
├── keycloak/            ├── opentelemetry-kube-stack/
├── kube-prometheus-stack/├── postgres-operator/
├── kyverno/             ├── sealed-secrets/
├── fluxcd/              ├── tempo/
├── sources/             ├── velero/
                         └── vsphere-csi/
```

### Template File Types

Templates use two file extensions:

- `.tpl` — Go template files. Rendered by the template engine with the full cluster config context. Contains `{{ }}` expressions.
- `.yaml` (no `.tpl`) — Static files. Copied as-is without rendering. Used when no cluster-specific values are needed (e.g., `rackspace-selfsigned-issuer.yaml`).

Some services also use `.jtpl` for Jinja-style templates (e.g., `gateway/envoy-proxy-config.yaml.jtpl`), handled by a separate rendering path.

### Template Variables Reference

All templates receive the full cluster configuration object. Common access patterns:

| Expression | Resolves To |
|---|---|
| `.OpenCenter.Cluster.ClusterName` | Cluster name (e.g., `k8s-dev`) |
| `.OpenCenter.Cluster.BaseDomain` | Base domain (e.g., `rmpk.dev`) |
| `.OpenCenter.Cluster.ClusterFQDN` | Full cluster domain |
| `(index .OpenCenter.Services "cert-manager").Email` | Service-specific field |
| `(index .OpenCenter.Services "cert-manager").Enabled` | Service enabled flag |
| `.OpenCenter.GitOps.GitOpsBaseRepo` | gitops-base repository URL |
| `.OpenCenter.GitOps.GitOpsBranch` | Git branch for base repo |
| `.OpenCenter.Infrastructure.Provider` | Infrastructure provider (`vmware`, `openstack`, `aws`, `baremetal`) |

Sprig functions are available: `default`, `quote`, `upper`, `lower`, `trimSuffix`, `toYaml`, etc.

## Secrets Handling

The cert-manager overlay includes SOPS-encrypted secrets (AWS credentials for Route53). The encryption flow:

1. The CLI renders `opencenter-aws-credentials-secret.yaml.tpl` with plaintext credentials from config.
2. SOPS encrypts the `data` fields using the cluster's Age key (stored in `secrets/age/`).
3. The encrypted file is committed to Git.
4. FluxCD's `cert-manager-override` Kustomization has `decryption.provider: sops` configured, referencing the `sops-age` Kubernetes Secret.
5. During reconciliation, FluxCD decrypts the secret on-the-fly and applies the plaintext Secret to the cluster.

The `.sops.yaml` file at the overlay level controls which fields get encrypted:

```yaml
encrypted_regex: ^(data|stringData|credentials|password|secret|key|token|cert|ca|crt|tls|clientSecret|accessKeyId|secretAccessKey)$
```

## Customizing a Service After Generation

### Modifying Helm Values

Edit `services/cert-manager/helm-values/override-values.yaml` in the customer repo. These values merge with (and override) the hardened defaults from gitops-base.

Example — increase cert-manager replicas:
```yaml
# helm-values/override-values.yaml
replicaCount: 3
webhook:
  replicaCount: 3
cainjector:
  replicaCount: 3
```

### Adding Custom Issuers

Add a new YAML file to `services/cert-manager/` and reference it in `kustomization.yaml`:

```yaml
# kustomization.yaml — add the new resource
resources:
  - "./rackspace-selfsigned-issuer.yaml"
  - "./rackspace-selfsigned-ca.yaml"
  - "./rackspace-ca-issuer.yaml"
  - "./opencenter-aws-credentials-secret.yaml"
  - "./letsencrypt-k8s-dev.yaml"
  - "./my-custom-issuer.yaml"          # New
```

### Pinning the Base Version

Change the GitRepository source to reference a specific tag instead of a branch:

```yaml
# services/sources/opencenter-cert-manager.yaml
spec:
  ref:
    tag: v1.2.0    # Pin to specific release
    # branch: main  # Remove or comment out
```

## Troubleshooting

### Template rendering fails during `opencenter cluster generate`

Check that the cluster config file has the required fields for the service. For cert-manager, `region` is required when using Route53 DNS-01 validation. Run `opencenter cluster validate <cluster>` to catch config issues before setup.

### FluxCD shows "path not found" on cert-manager-base

The `path: applications/base/services/cert-manager` must exist in the gitops-base repo at the ref (branch/tag) specified in the GitRepository source. Verify:

```bash
git ls-tree --name-only -r <branch> -- applications/base/services/cert-manager
```

### SOPS decryption fails on cert-manager-override

The `sops-age` Secret in `flux-system` namespace must contain the Age private key matching the public key used for encryption. Recreate it:

```bash
kubectl create secret generic sops-age \
  --from-file=age.agekey=<path-to-age-keys.txt> \
  -n flux-system --dry-run=client -o yaml | kubectl apply -f -
```

### cert-manager-override stuck waiting on cert-manager-base

Check the HelmRelease health:

```bash
kubectl get helmrelease cert-manager -n cert-manager
flux get kustomization cert-manager-base
```

The base Kustomization has a health check on the HelmRelease. If the Helm chart fails to install (image pull issues, CRD conflicts), the override will never proceed.
