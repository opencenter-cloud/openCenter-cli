---
last_updated: 2026-10-27
id: service-cert-manager
title: "cert-manager"
sidebar_label: cert-manager
description: cert-manager service configuration, conditional credentials, and descriptor ownership.
doc_type: reference
audience: "platform engineers, operators"
tags: [cert-manager, tls, services]
---

> **Evidence:** `internal/config/services/cert_manager.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/validators.go`, `internal/config/services/provider_registry.go`, `internal/config/services/secrets_validator.go`, and `internal/services/descriptors/data/service-cert-manager.yaml`.

## Configuration

The generated default is enabled in `cert-manager`. `CertManagerConfig` embeds `BaseConfig`.

```yaml
opencenter:
  services:
    cert-manager:
      enabled: true
      namespace: cert-manager
      letsencrypt_server:
      email:
      region:
      dns_zones: []
      create_cluster_issuer: false
      dns_provider:
      issuers:
        - name:
          type: letsencrypt
          server:
      adoption_mode: managed
      address_pool:
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | `true` | `NewDefaultServiceConfig` |
| `namespace` | `cert-manager` | `NewDefaultServiceConfig` |
| `letsencrypt_server`, `email`, `region`, `dns_zones`, `issuers`, `dns_provider` | empty | `CertManagerConfig` |
| `create_cluster_issuer` | false in Go zero value; schema tag documents true | `CertManagerConfig` |
| `issuers[].name`, `type` | required fields | `CertIssuer` |
| `issuers[].server` | empty | `CertIssuer` |

The registered validator checks that a configured `letsencrypt_server` starts with `https://` and a configured `email` contains `@`. The provider registry records DNS provider choices and provider compatibility; it does not change the config type's fields.

## Secrets and credential resolution

`OpenCenterSecrets.CertManager` contains named AWS and Cloudflare credential maps plus legacy flat fields. The service secret mapping records conditional credentials for Route53, Cloudflare, Cloud DNS, and Azure DNS; Designate is documented in that mapping as using infrastructure credentials.

`dns_provider` alone is enough to render an issuer. `Config.ResolvedCertManagerAWSCredentials` and `Config.ResolvedCertManagerCloudflareCredentials` (`internal/config/v2/helpers.go`) are the single source of truth for which credentials the renderer emits and readiness requires:

1. Every enabled entry of `secrets.cert_manager.aws` / `secrets.cert_manager.cloudflare` renders, keyed by its map key.
2. When none is enabled and `dns_provider` is `route53`, the AWS application credentials (or the legacy flat `aws_access_key`/`aws_secret_access_key` pair) become one enabled credential named `route53`, with the region resolved from `services.cert-manager.region` → `secrets.global.aws.application.region` → `opencenter.meta.region`.
3. When none is enabled and `dns_provider` is `cloudflare`, the legacy flat `secrets.cert_manager.cloudflare_api_token` becomes one enabled credential named `cloudflare`.

The rendered ClusterIssuer is named `letsencrypt-<credential>`. Which credential becomes the default is decided by `Config.ResolvedDNSCredential` (`internal/config/v2/helpers.go`), and the gateway annotation `cert-manager.io/cluster-issuer` follows it, so `dns_provider` also selects the default issuer. Precedence: a per-service `dns_credential`, then a credential whose `dns_zones` cover the hostname being certified, then `services.cert-manager.default_credential`, then the sole enabled credential, then the lowest-sorted name. A credential with no `dns_zones` gets `selector.dnsZones` defaulted to `opencenter.cluster.base_domain`, matching every customer overlay. Designate is the exception: its issuer needs no service credential and resolves as `letsencrypt-designate`.

## Turning ACME off

`dns_provider: none` (aliases `off`, `disabled`, `internal`) disables ACME DNS-01 entirely: no credential Secret, no ACME ClusterIssuer, and every Gateway and service certificate resolves to `rackspace-ca`, the in-cluster self-signed CA the base templates always render. It is strictly opt-in — the registry auto-selection never yields it — and an explicitly enabled credential under `none` is a validation error rather than a silently ignored one.

```bash
opencenter cluster set my-cluster opencenter.services.cert-manager.dns_provider=none
```

The tradeoff is trust, not function: `rackspace-ca` is issued by a self-signed CA created inside the cluster, so it is self-healing but not browser-trusted. Use a real provider for anything a browser reaches.

## Following settings

`dns_provider`, `email`, and `dns_zones` are provenance-tagged like `base_domain`. While `opencenter.cluster.managed.dns_provider` is `default`, `cluster_defaults.dns_provider` / `dns_email` / `dns_zones` from `settings.yaml` are re-applied at every load, so changing the provider in settings propagates through `generate` and the stale issuer is pruned. Setting the field with `cluster set` pins it, and the cluster then owns the value.

Write the named map rather than the legacy flat fields; the flat fields exist for migration only:

```bash
opencenter cluster set my-cluster \
  opencenter.services.cert-manager.dns_provider=cloudflare \
  secrets.cert_manager.cloudflare.cloudflare.enabled=true \
  secrets.cert_manager.cloudflare.cloudflare.api_token=<token>
```

`cluster set` creates a missing intermediate credential key with the map's element type, so the command above works on a config that has no `cloudflare` block yet.

Setting the credential in the config is only half of it. The credential `Secret`
that the issuer references is materialized and SOPS-encrypted by `secrets sync`,
which owns `services/cert-manager/opencenter-<provider>-credentials-secret-<name>.yaml`;
`cluster generate` renders the issuer and the kustomization only, and adopts an
existing credential file rather than rewriting it with plaintext. So after
changing a DNS credential:

```bash
opencenter secrets sync my-cluster --services cert-manager
opencenter cluster generate my-cluster
```

`secrets sync` skips a credential whose material is empty or still the `CHANGEME`
placeholder, matching the renderer, which emits no issuer for it; and it refuses
to record a file that is not encrypted afterwards, so a credential cannot reach
the repository in plaintext unnoticed. See
[Services and Templates](../../concepts/services-templates.md#secrets-handling).

## Descriptor rendering

`service-cert-manager.yaml` declares `service: cert-manager`, root `services/cert-manager`, templates for its source and Flux files, and aggregate targets `services-fluxcd-aggregate` and `services-sources-aggregate`. It declares no `when` condition.

## Commands

```bash
opencenter cluster service enable cert-manager
opencenter cluster service disable cert-manager
opencenter cluster service options cert-manager
```
