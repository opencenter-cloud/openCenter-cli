---
last_updated: 2026-11-02
id: service-gateway
title: "Gateway"
sidebar_label: Gateway
description: Gateway service configuration, listener fields, and catalog rendering.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, gateway, services]
---

> **Evidence:** `internal/config/services/gateway.go`, `internal/config/v2/defaults.go`, `internal/services/plugins/registry.go`, and `internal/gitops/render_catalog.go`.

## Configuration

`GatewayConfig` embeds `BaseConfig`, adds Gateway identity fields, optional bring-your-own TLS settings, and listeners. The generated default is enabled in `gateway`.

```yaml
opencenter:
  services:
    gateway:
      enabled: true
      namespace: gateway
      gateway_name:
      gateway_namespace:
      gateway_class:
      default_issuer:
      floating_ip:
      tls:
        default_cluster_certificate: false
        wildcard_secret_name:
        secret_namespace:
        per_listener_secrets: {}
      listeners:
        - name:
          port: 443
          protocol: HTTPS
          hostname:
          tls_secret_name:
      adoption_mode: managed
      address_pool:
```

| Field | Default | Evidence |
|-------|---------|----------|
| `enabled` | `true` | `NewDefaultServiceConfig` |
| `namespace` | `gateway` | `NewDefaultServiceConfig` |
| `gateway_name`, `gateway_namespace`, `gateway_class`, `default_issuer` | empty in config type | `GatewayConfig` |
| `floating_ip` | empty | `GatewayConfig.FloatingIP` |
| `tls.*` | absent | `GatewayTLSConfig` |
| `tls.default_cluster_certificate` | false | `GatewayTLSConfig.DefaultClusterCertificate` |
| `listeners[].name`, `port`, `protocol` | required by schema tags | `GatewayListener` |
| `listeners[].hostname`, `tls_secret_name` | empty | `GatewayListener` |

## Runtime TLS behavior

`GatewayConfig.IsBYO` is true when `tls.default_cluster_certificate` is set, or a wildcard or per-listener secret is configured; the generator then omits the cert-manager annotation. `TLSSecretFor` resolves a per-listener secret, then the wildcard, then the default cluster certificate, then its supplied built-in default. A non-default Secret name configured on cert-manager is copied into `WildcardSecretName` by the renderer, because `GatewayConfig` has no view of the cert-manager service. `secret_namespace` is returned separately and is empty when unset.

`floating_ip` pins a pre-allocated OpenStack floating IP on the Envoy data-plane LoadBalancer Service (`loadbalancer.openstack.org/floating-ip`) so the CCM binds that address instead of allocating a new one. Rackspace IAD3/SJC3 (provider-os CCM v1.36) was measured to ignore the annotation even when present at Service creation, so the pin does not hold there; verify against the target cloud before relying on it. It is ignored on non-OpenStack providers.

## Rendering and dependencies

The plugin registry records `gateway-api`; the catalog records `envoy-gateway-api-base`. The catalog marks Gateway as single-stage and names `namespace.yaml`, `gateway-class.yaml`, `gateway.yaml`, and `envoy-proxy-config.yaml` as generated resources. No service descriptor file names `gateway`.

## Commands

```bash
opencenter cluster service enable gateway
opencenter cluster service disable gateway
opencenter cluster service options gateway
```
