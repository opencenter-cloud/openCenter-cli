---
id: service-gateway-api
title: "Gateway API CRDs"
sidebar_label: Gateway API
description: Gateway API Custom Resource Definitions for advanced traffic routing.
doc_type: reference
audience: "platform engineers, operators"
tags: [networking, gateway-api, crds, routing]
---

> **Purpose:** For platform engineers and operators, documents the Gateway API CRD service, covering supported route types and verification.

## Overview

Gateway API installs the Kubernetes Gateway API Custom Resource Definitions (CRDs), enabling HTTPRoute, TLSRoute, TCPRoute, GRPCRoute, and Gateway class resources. This is a CRD-only service that provides the API types consumed by gateway implementations such as Envoy Gateway.

## Configuration

```yaml
opencenter:
  services:
    gateway-api:
      enabled: true                    # default: true
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Install Gateway API CRDs |

## Dependencies

None.

## Verification

```bash
# Check Gateway API CRDs are installed
kubectl get crds | grep gateway.networking.k8s.io

# Verify specific route CRDs
kubectl get crds httproutes.gateway.networking.k8s.io
kubectl get crds tlsroutes.gateway.networking.k8s.io
kubectl get crds tcproutes.gateway.networking.k8s.io
kubectl get crds grpcroutes.gateway.networking.k8s.io
kubectl get crds gatewayclasses.gateway.networking.k8s.io

# List available GatewayClasses
kubectl get gatewayclasses
```

## CLI Commands

```bash
# Enable Gateway API CRDs
opencenter cluster service enable gateway-api

# Disable Gateway API CRDs
opencenter cluster service disable gateway-api

# View service options
opencenter cluster service options gateway-api

# Check service status
opencenter cluster service status
```
