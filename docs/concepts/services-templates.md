---
last_updated: 2026-10-27
id: services-templates
title: "Service Templates: How They Work"
sidebar_label: "Service Templates: How They Work"
description: How openCenter generates, deploys, and manages platform service configurations using the cert-manager service as a worked example.
doc_type: explanation
audience: "platform engineers, operators"
tags: [services, templates, cert-manager, fluxcd, gitops, kustomize]
---
# Service Templates: How They Work

**Purpose:** For platform engineers, explains how the openCenter service template system generates cluster-specific GitOps manifests from embedded Go templates, and how FluxCD reconciles them against the gitops-base repository. Uses cert-manager as the primary worked example.

## Concept Summary

Every openCenter-managed cluster runs a set of platform services (cert-manager, Kyverno, MetalLB, Keycloak, etc.). These services share a common deployment pattern:

1. The openCenter-cli reads and validates the typed cluster desired state.
2. The renderer resolves the enabled service through the immutable render catalog;
   catalog entries contain direct function references rather than mutable lookup
   state or configuration-selected renderer names.
3. An explicit descriptor, when present, declares the service's file layout,
   conditions, aggregate membership, and ownership boundaries.
4. The renderer produces a plan of generator-owned files and actions before any
   output is written.
5. The planned files are rendered atomically into the customer GitOps repository.
6. FluxCD on the cluster reconciles the generated manifests, pulling base
   definitions from `openCenter-gitops-base` and applying cluster-specific
   overrides from the customer repository.

The result is a two-tier Kustomize overlay model: a shared base maintained
centrally composed with per-cluster overrides generated from typed desired state.

## Desired State to Owned Output

The rendering boundary is deliberately one-way:

```
Typed desired state
        │
        ▼
Immutable render catalog
(service identity + direct function references)
        │
        ▼
Explicit descriptor
(files, conditions, aggregates, ownership)
        │
        ▼
Generation plan
(planned generator-owned files and actions)
        │
        ▼
Atomic rendered overlay
(user-owned custom/ remains untouched)
```

Service configuration describes what the operator wants: whether a service is
enabled and its typed settings. The catalog and descriptor describe how the
implementation fulfills that desired state. Renderer selection, stage topology,
raw Helm override values, and generated-file ownership are internal concerns;
they are not public cluster configuration.

If a service needs an operator-facing setting, add a typed field and validate it.
If an operator needs a manifest or value not modeled by the service, place it in
the service overlay's `custom/` directory. Do not expose descriptor topology or
raw renderer inputs in YAML.

## How It Works

### The Three Layers

Every service deployment involves three distinct layers, each owned by a different artifact:

| Layer | Owner | Location | Contains |
| --- | --- | --- | --- |
| Base manifests | `openCenter-gitops-base` repo | `applications/base/services/<service>/` | HelmRelease, HelmRepository, namespace, hardened Helm values |
| Overlay manifests | Customer GitOps repo | `applications/overlays/<cluster>/services/<service>/` | Issuers, secrets, Helm value overrides, custom resources |
| FluxCD wiring | Customer GitOps repo | `applications/overlays/<cluster>/services/fluxcd/` and `sources/` | GitRepository sources, Kustomization resources that connect base → overlay |

### Desired-State Rendering Pipeline

When you run `opencenter cluster generate <cluster-name>`, `SetupService`
invokes the live GitOps copy, descriptor, catalog, infrastructure, Flux-bridge,
and promotion functions. It does **not** construct `PipelineGenerator` or
`ServiceStage` from `internal/gitops/stages`; those are supporting APIs used by
their own callers and tests.

```
Cluster desired state
        │
        ▼
┌─────────────────────┐
│  1. Read and        │  Uses typed, validated service configuration.
│     validate        │
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  2. Resolve service │  Selects an explicit descriptor or immutable catalog
│     plan            │  planning/rendering function references.
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  3. Read descriptor │  Declares files, conditions, aggregates, and ownership.
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  4. Plan owned      │  Computes the generator-owned files and actions before
│     files           │  writing anything.
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  5. Render & write  │  Go templates and direct functions write atomically;
│                     │  user-owned `custom/` files are preserved.
└─────────────────────┘
```

The template engine uses Go’s `text/template` with Sprig function support, an LRU cache for repeated renders, and template composition for shared partials. All templates are compiled into the CLI binary via `go:embed`.

Source: `internal/gitops/embed.go`

```go
//go:embed all:gitops-base-dir all:templates
var Files embed.FS
```

The live path is implemented by `internal/cluster/setup_service.go` and the
`internal/gitops` descriptor/catalog planners; `ServiceStage` is not the live
generator.

### Cluster Configuration: The Input

Each cluster has a v2 configuration file at
`<blueprints-dir>/<organization>/<cluster>/<cluster>-config.yaml` that drives
rendering. The services section controls which services are enabled and
provides service-specific parameters.

For cert-manager, the relevant config block looks like this:

```yaml
# From <blueprints-dir>/example-platform/dev/dev-config.yaml
opencenter:
  services:
    cert-manager:
      enabled: true
      email: platform-support@example.test
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

Template: `services/sources/opencenter-cert-manager.yaml.tpl`, which delegates to `sourceAuthBlockForService "cert-manager"` (`internal/gitops/copy.go`).

All base-service source templates (cert-manager, calico, etcd-backup, fluxcd, harbor, kafka-cluster, keycloak, olm) call this one shared renderer. It preserves any service-level `source.repo`, `source.branch`, or `source.release` override while rendering both authentication variants as an active block plus a fully commented alternative. `--gitops-auth` selects which block is active for this generation run only; the other block remains commented so an operator can inspect or deliberately switch it later.

Rendered output with the default token method:

```yaml
# applications/overlays/dev/services/sources/opencenter-cert-manager.yaml
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

Use `opencenter cluster generate <cluster> --gitops-auth=ssh` to make the SSH block active. **There is no `secretRef` here.** `openCenter-gitops-base` is a public repository, so `sourceAuthBlockForService` renders base-service sources through `sourceAuthBlockAnonymous`, which omits `secretRef` entirely (`internal/gitops/copy.go`, `internal/gitops/source_auth.go`). Attaching a credential Secret here would make Flux try to authenticate to public GitHub with the wrong credential — the source code comments explicitly call out the failure mode (HTTP 401) this is guarding against. Bootstrap explicitly does **not** create an `opencenter-base` credential Secret (see the comments in `internal/cluster/bootstrap_provider_infra.go` and `internal/cluster/kind_bootstrap_provider.go`).

`opencenter-keycloak-config` is genuinely different: it sources the *customer* GitOps repository (not the public base repo) through `sourceAuthBlockCustomerRepository`, which does attach `secretRef: name: flux-system` — the credential Secret that the `flux bootstrap` CLI itself creates when bootstrapping FluxCD against the customer repository.

#### Category 2: FluxCD Kustomizations (The Wiring)

Template: `services/fluxcd/cert-manager.yaml.tpl`

This is the most important template. It generates two FluxCD Kustomization resources that implement the base+overlay pattern (fields shown are the actual current template, not simplified):

```yaml
# Kustomization 1: Deploy base from gitops-base repo
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
    name: opencenter-cert-manager    # Points to gitops-base
    namespace: flux-system
  path: applications/base/services/cert-manager  # Path INSIDE gitops-base
  targetNamespace: cert-manager
  prune: true
  wait: true
  force: {{ adoptionForce "cert-manager" }}
  suspend: {{ adoptionSuspend "cert-manager" }}
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: cert-manager
      namespace: cert-manager
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: cert-manager
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
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
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  decryption:
    provider: sops
    secretRef:
      name: sops-age             # SOPS key for decrypting secrets
  sourceRef:
    kind: GitRepository
    name: flux-system            # Points to THIS customer repo
    namespace: flux-system
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/cert-manager
  targetNamespace: cert-manager
  prune: true
  wait: true
  force: {{ adoptionForce "cert-manager" }}
  suspend: {{ adoptionSuspend "cert-manager" }}
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: cert-manager
      namespace: cert-manager
```

`adoptionForce` and `adoptionSuspend` (`internal/gitops/copy.go`) are template functions, not literal service fields — they let generation control Flux's adoption/suspend behavior per service. The `{{ .OpenCenter.Cluster.ClusterName }}` token resolves to the cluster name from config (e.g., `k8s-dev`, `dev`, `k8s-sandbox`).

The dependency chain is: `sources` → `cert-manager-base` → `cert-manager-override`. This ordering guarantees the base HelmRelease is healthy before overrides are applied.

#### Category 3: Service Overlay Resources

The literal files under `internal/gitops/templates/cluster-apps-base/services/cert-manager/` are:

| File | Kind | Purpose |
| --- | --- | --- |
| `rackspace-selfsigned-issuer.yaml` | Static copy | Self-signed `Issuer` for bootstrapping the internal CA chain |
| `rackspace-selfsigned-ca.yaml.tpl` | Rendered | `Certificate` resource for the internal CA; `commonName` set from `.OpenCenter.Cluster.BaseDomain` |
| `rackspace-ca-issuer.yaml` | Static copy | `ClusterIssuer` that uses the self-signed CA |
| `opencenter-openstack-designate-credentials-secret.yaml.tpl` | Rendered + SOPS encrypted | Designate credentials, only emitted when `DNSProvider: designate` |
| `helm-values/override-values.yaml` | Static copy (empty) | Placeholder for Helm value overrides |
| `README.md` | Static copy | Service documentation placeholder |

**Everything else — the per-credential AWS/Cloudflare `Secret` and `ClusterIssuer` objects, the optional Designate `ClusterIssuer`, and the overlay's own `kustomization.yaml` — is generated by Go code, not `.tpl` files.** `planCertManagerDynamicActions` in `internal/gitops/cert_manager_renderer.go` reads `cfg.ResolvedCertManagerAWSCredentials()` and `cfg.ResolvedCertManagerCloudflareCredentials()` — the enabled entries of `cfg.Secrets.CertManager.AWS` / `.Cloudflare`, plus the credential derived from `DNSProvider` when no named entry is enabled — and, for each entry, emits a `Secret` plus a matching `ClusterIssuer` from Go string templates embedded in that file. A credential without `dns_zones` gets `selector.dnsZones` defaulted to `opencenter.cluster.base_domain`. If `DNSProvider: designate` is set, it also emits a Designate `ClusterIssuer`. It then emits `kustomization.yaml` itself, listing exactly the files it just planned. There is no `letsencrypt-issuer.yaml.tpl` or `kustomization.yaml.tpl` on disk for cert-manager — this is a deliberate design change from AWS-Route53-only to a multi-credential, multi-provider (AWS Route53, Cloudflare, OpenStack Designate) model.

The self-signed CA template shows how config values still flow into the file-walk-rendered resources:

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

An AWS-backed Let's Encrypt issuer, generated inline by `cert_manager_renderer.go` (not a `.tpl` file), looks like this once rendered for a named credential `route53-prod`:

```yaml
# services/cert-manager/letsencrypt-route53-prod-issuer.yaml (generated)
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-route53-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: mpk-support@rackspace.com
    privateKeySecretRef:
      name: letsencrypt-dns01-route53-prod
    solvers:
      - dns01:
          route53:
            region: us-east-1
            accessKeyIDSecretRef:
              name: "opencenter-aws-credentials-secret-route53-prod"
              key: aws-access-key
            secretAccessKeySecretRef:
              name: "opencenter-aws-credentials-secret-route53-prod"
              key: aws-secret-access-key
        selector:
          dnsZones:
            - example.com
```

The ACME server and email above are the fallback values `extractCertManagerConfig` applies when the cert-manager service config leaves `LetsEncryptServer`/`Email` empty — they are not stored defaults (see [Plugin Internal Services](plugin-internal-services.md)).

### The Kustomization Manifest (Overlay Glue)

`kustomization.yaml` for cert-manager is generated by `renderInlineTemplateContent(kustomizationTemplate, ...)` in `internal/gitops/cert_manager_renderer.go` — not by a `.tpl` file. It lists exactly the bootstrap files plus whatever credentials/issuers were planned for this cluster, and creates a Kubernetes Secret from the Helm override values:

```yaml
# Generated for a cluster with one AWS credential named "route53-prod"
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: cert-manager
resources:
  - "./rackspace-selfsigned-issuer.yaml"
  - "./rackspace-selfsigned-ca.yaml"
  - "./rackspace-ca-issuer.yaml"
  - "./opencenter-aws-credentials-secret-route53-prod.yaml"
  - "./letsencrypt-route53-prod-issuer.yaml"
secretGenerator:
  - name: cert-manager-values-override
    files: [override.yaml=helm-values/override-values.yaml]
    options:
      disableNameSuffixHash: true
```

The `secretGenerator` creates a Kubernetes Secret named `cert-manager-values-override` containing the Helm override values. The base HelmRelease in `openCenter-gitops-base` references this Secret via `valuesFrom`, allowing the overlay to inject custom Helm values without modifying the base.

### Conditional Rendering

The literal `sources/kustomization.yaml.tpl` and `fluxcd/kustomization.yaml.tpl` templates use Go conditionals for the handful of services that still have explicit `.tpl` source/Flux files (cert-manager, harbor, kafka-cluster, keycloak, olm, plus an aggregate `opencenter-observability.yaml` entry gated on any of kube-prometheus-stack/loki/tempo/mimir/opentelemetry-kube-stack being enabled):

```yaml
# sources/kustomization.yaml.tpl (excerpt)
resources:
{{- $services := .OpenCenter.Services }}
{{- if (index $services "cert-manager").Enabled }}
  - "opencenter-cert-manager.yaml"
{{- end }}
{{- if (index $services "harbor").Enabled }}
  - "opencenter-harbor.yaml"
{{- end }}
{{- range autoServices }}
{{- $srcName := autoServiceSourceName . }}
{{- if eq $srcName (printf "opencenter-%s" .) }}
  - "{{ $srcName }}.yaml"
{{- end }}
{{- end }}
```

Everything else — kyverno, gateway, loki, tempo, velero, metallb, and the rest of the immutable-catalog services — is not listed with a per-service `if` block at all. `autoServices` (`internal/gitops/copy.go`) returns, sorted, every enabled service that has no explicit descriptor but is owned by the immutable `RenderCatalog`; `autoServiceSourceName` resolves each one's catalog-declared `SourceName`. Adding a new catalog entry does not require touching this template file.

The staged `ServiceStage` (`internal/gitops/stages/service_stage.go`) — the supporting, non-live pipeline API described above — has its own, separate render-condition mechanism keyed on provider/enablement/field checks; it is unrelated to the `autoServices` loop used by the live path.

### Service validation boundary

The `CertManagerPlugin` validator in `internal/services/plugins` belongs to the
supporting, unwired plugin subsystem. It is not a production pre-render hook
for `cluster generate`. Live validation is owned by the typed configuration,
the command/config validation paths, and the readiness checks invoked by
`SetupService`; the exact checks should be read from those source paths rather
than inferred from `ServicePlugin.Validate`.

This distinction matters for every service: a validator or dependency listed in
`internal/services/plugins` is not enforced by the live renderer unless a live
validation path also implements it. See [Platform Services Architecture](../reference/platform-services.md)
for the authoritative boundary.

## Generated Output Structure

After `opencenter cluster generate`, the customer repository contains this structure for cert-manager:

Generated files in the overlay's `services/`, `managed-services/`, and
`customer-managed/` paths are tracked in the version-2 ledgers under
`.opencenter/ownership/`: the active cluster ledger is
`.opencenter/ownership/clusters/<cluster>.json`, while the small exact global
allowlist is `.opencenter/ownership/global.json`. Keep resources not modeled by
openCenter in a service's user-owned `custom/` directory; generation does not
modify or delete existing custom content. Run
`opencenter cluster migrate-layout --custom --org <organization> --cluster
<cluster> --apply` to move pre-existing hand-authored files before regenerating.

```
applications/overlays/<cluster>/
├── kustomization.yaml                          # Top-level: includes flux-system + services/fluxcd
│
├── services/
│   ├── sources/
│   │   ├── kustomization.yaml                  # Lists all GitRepository sources
│   │   └── opencenter-cert-manager.yaml        # GitRepository → gitops-base (anonymous, no secretRef)
│   │
│   ├── fluxcd/
│   │   ├── kustomization.yaml                  # Lists all FluxCD Kustomizations
│   │   ├── sources.yaml                        # Kustomization for sources/
│   │   └── cert-manager.yaml                   # cert-manager-base + cert-manager-override
│   │
│   └── cert-manager/
│       ├── kustomization.yaml                  # Generated: lists resources + secretGenerator
│       ├── rackspace-selfsigned-issuer.yaml    # Issuer (self-signed bootstrap)
│       ├── rackspace-selfsigned-ca.yaml        # Certificate (internal CA)
│       ├── rackspace-ca-issuer.yaml            # ClusterIssuer (CA-based)
│       ├── letsencrypt-<credential-name>-issuer.yaml       # Generated per named AWS/Cloudflare credential
│       ├── opencenter-aws-credentials-secret-<credential-name>.yaml     # `secrets sync`, SOPS-encrypted
│       ├── opencenter-cloudflare-credentials-secret-<credential-name>.yaml  # `secrets sync`, SOPS-encrypted
│       ├── opencenter-openstack-designate-credentials-secret.yaml  # Only when DNSProvider: designate
│       ├── helm-values/
│       │   └── override-values.yaml            # Helm value overrides (empty by default)
│       └── README.md
└── .opencenter/ownership/                      # Version-2 ownership ledgers
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

The `dependsOn` fields enforce ordering. If `cert-manager-base` fails its health check (the HelmRelease isn’t ready), `cert-manager-override` won’t be applied.

## How Other Services Consume Cert-Manager

Once cert-manager is running, other services reference its Issuers and ClusterIssuers to obtain TLS certificates.

### Gateway / Ingress

`gateway` is one of the immutable-catalog services (`internal/gitops/render_catalog.go`; `EmitSource: true`, no `.tpl` directory), so its Gateway resource is generated by Go code, not by a template file. Conceptually it still creates a Gateway resource that references cert-manager for TLS:

```yaml
# Illustrative — generated, not a template file
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

Keycloak’s HTTPRoute and TLS configuration depend on certificates issued by cert-manager:

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

## Two Rendering Mechanisms, Not One Pattern

Only a small, fixed set of services still has an on-disk `services/<service>/` template directory under `internal/gitops/templates/cluster-apps-base/services/`. As of this writing, that directory contains exactly:

```
internal/gitops/templates/cluster-apps-base/services/
├── calico/
├── cert-manager/
├── etcd-backup/
├── fluxcd/
├── harbor/
├── kafka-cluster/
├── keycloak/
├── olm/
└── sources/
```

Each of these follows (or approximates) the three-file pattern: a `services/sources/opencenter-<service>.yaml.tpl` GitRepository source, a `services/fluxcd/<service>.yaml.tpl` with base+override Kustomizations, and a `services/<service>/` directory of overlay resources. `sources/` and `fluxcd/` themselves hold the two aggregate `kustomization.yaml.tpl` files described above, not a service.

**Every other built-in service — gateway, gateway-api, headlamp, kube-prometheus-stack, kyverno, loki, longhorn, metallb, mimir, olm-config, openstack-ccm, openstack-csi, opentelemetry-kube-stack, postgres-operator, rbac-manager, sealed-secrets, tempo, velero, vsphere-csi, weave-gitops, external-snapshotter, and more — has no `.tpl` directory at all.** They are entries in the immutable `RenderCatalog` (`internal/gitops/render_catalog.go`): a `RenderSpec` with direct Go field values (default namespace, source name, base path, dependencies, whether it emits a Helm-override secret, and so on) plus, where a service needs custom logic beyond the generic catalog behavior, a `catalogDynamicPlanner` (as cert-manager and etcd-backup use). `auto_descriptor.go` turns each catalog entry into GitRepository, Kustomization, namespace, and override-values actions without any embedded template file. See [GitOps Engine](../CODEMAPS/gitops-engine.md) and [Rendering Ownership and Secret Artifacts](../CODEMAPS/rendering-ownership-and-secret-artifacts.md) for the full planning/ownership model both mechanisms share.

Do not assume a service you're extending has a `.tpl` directory — check `internal/gitops/render_catalog.go` first. If it is already a catalog entry, extend its `RenderSpec` or add a dynamic planner rather than creating template files that would never be read.

### Template File Types

Templates use two file extensions:

* `.tpl` -- Go template files. Rendered by the template engine with the full cluster config context. Contains `{{ }}` expressions.
* `.yaml` (no `.tpl`) -- Static files. Copied as-is without rendering. Used when no cluster-specific values are needed (e.g., `rackspace-selfsigned-issuer.yaml`).

Some services also use `.jtpl` for Jinja-style templates (e.g., `gateway/envoy-proxy-config.yaml.jtpl`), handled by a separate rendering path.

### Template Variables Reference

All templates receive the full cluster configuration object. Common access patterns:

| Expression | Resolves To |
| --- | --- |
| `.OpenCenter.Cluster.ClusterName` | Cluster name (e.g., `k8s-dev`) |
| `.OpenCenter.Cluster.BaseDomain` | Base domain (e.g., `rmpk.dev`) |
| `.OpenCenter.Cluster.ClusterFQDN` | Full cluster domain |
| `(index .OpenCenter.Services "cert-manager").Email` | Service-specific field |
| `(index .OpenCenter.Services "cert-manager").Enabled` | Service enabled flag |
| `.OpenCenter.GitOps.BaseRepo.URL` | gitops-base repository URL |
| `.OpenCenter.GitOps.BaseRepo.Branch` / `.Release` | Git branch or tag for the base repo |
| `.OpenCenter.Infrastructure.Provider` | Infrastructure provider (schema enum: `openstack`, `aws`, `gcp`, `azure`, `baremetal`, `vsphere`, `vmware`, `kind`, `magnum`; see [Providers](../CODEMAPS/providers.md) for which of these are actually implemented) |

Sprig functions are available: `default`, `quote`, `upper`, `lower`, `trimSuffix`, `toYaml`, etc.

## Secrets Handling

The cert-manager overlay carries one SOPS-encrypted `Secret` per DNS credential
(`opencenter-aws-credentials-secret-<name>.yaml`,
`opencenter-cloudflare-credentials-secret-<name>.yaml`, or the Designate
equivalent). The credential itself lives in the cluster config
(`secrets.cert_manager.aws.<name>` / `secrets.cert_manager.cloudflare.<name>`),
written there by `cluster configure`.

Ownership of that file belongs to `secrets sync`, not to the renderer:

1. `secretartifacts.Plan` emits one artifact per credential that
   `cfg.ResolvedCertManager{AWS,Cloudflare}Credentials` reports as in force, at
   the path `CertManagerCredentialArtifactPath` derives. The renderer reads the
   same builder for the filename and for the `Secret` name its issuers
   reference, so the two sides cannot disagree.
2. `secrets sync` writes the manifest and encrypts `data`/`stringData` with the
   cluster's Age key (stored under the secrets zone; see [Secret and Config
   Separation](security-update-design.md)), then records the file in the
   ownership ledger. A file that is still plaintext afterwards is reported as
   an error rather than recorded, because a recorded hash would make every later
   sync call the readable file unchanged.
3. `cluster generate` renders only the issuers and the kustomization. If a
   credential file it would have written already exists, the renderer adopts it
   instead of overwriting it with plaintext.
4. The encrypted file is committed to Git.
5. FluxCD's `cert-manager-override` Kustomization has `decryption.provider: sops`
   configured, referencing the `sops-age` Kubernetes Secret.
6. During reconciliation, FluxCD decrypts the secret on-the-fly and applies the
   plaintext Secret to the cluster.

Because step 2 is what encrypts, `cluster generate` alone never puts a
credential in the repository: run `secrets sync` after changing a DNS
credential.

The `.sops.yaml` file at the overlay level (rendered from `internal/gitops/templates/cluster-apps-base/.sops.yaml.tpl`) controls which fields get encrypted. The built-in default (`internal/config/v2/defaults.go`) is:

```yaml
encrypted_regex: ^(data|stringData|secret)$
```

This is a per-cluster configuration value (`secrets.sops.encrypted_regex`, `internal/config/v2/config.go` `SOPSConfig`), so a given cluster's `.sops.yaml` may use a broader pattern than the default shown here.

## Customizing a Service After Generation

### Modifying Helm Values

Edit `services/cert-manager/helm-values/override-values.yaml` in the customer repo. These values merge with (and override) the hardened defaults from gitops-base.

Example -- increase cert-manager replicas:

```yaml
# helm-values/override-values.yaml
replicaCount: 3
webhook:
  replicaCount: 3
cainjector:
  replicaCount: 3
```

### Adding Custom Issuers

`services/cert-manager/kustomization.yaml` is generator-owned — `planCertManagerDynamicActions` regenerates it on every `opencenter cluster generate`, and the file is tracked in the active cluster ledger at `.opencenter/ownership/clusters/<cluster>.json`. Hand-editing it to add a resource line does not survive the next regeneration.

To add resources cert-manager's typed configuration doesn't model (a hand-authored issuer, for example), put them in `services/cert-manager/custom/` and reference them from that directory's own `kustomization.yaml`, which generation creates once and never overwrites. See [Rendering Ownership and Secret Artifacts](../CODEMAPS/rendering-ownership-and-secret-artifacts.md) for the ownership contract, and [Configuration Lifecycle](configuration-lifecycle.md) for `opencenter cluster migrate-layout --custom` if you have pre-existing hand-authored files under a generator-owned path.

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
