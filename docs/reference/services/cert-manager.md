---
id: service-cert-manager
title: "cert-manager"
sidebar_label: cert-manager
description: Automated TLS certificate management with ACME, self-signed, and CA issuers across multiple DNS providers.
doc_type: reference
audience: "platform engineers, operators"
tags: [cert-manager, tls, certificates, acme, letsencrypt, services]
---

> **Purpose:** For platform engineers and operators, documents the complete configuration surface, secrets, dependencies, and verification steps for the cert-manager service.

## Overview

cert-manager automates TLS certificate provisioning and renewal using ACME (Let's Encrypt), self-signed, or CA-based issuers. It supports DNS-01 challenges across multiple cloud DNS providers with named multi-credential configuration, and can auto-default the DNS provider based on the cluster's infrastructure provider.

## Configuration

```yaml
opencenter:
  services:
    cert-manager:
      enabled: true
      letsencrypt_server: https://acme-v02.api.letsencrypt.org/directory  # ACME directory URL (default: production Let's Encrypt)
      email:                             # Required. Contact email for ACME registration
      region:                            # Cloud region for DNS provider API calls
      dns_zones: []                      # List of DNS zones to manage (e.g., ["example.com", "internal.example.com"])
      create_cluster_issuer: true        # Create a default ClusterIssuer resource (default: true)
      dns_provider:                      # DNS provider: route53 | designate | cloudflare | clouddns | azuredns
      issuers:                           # List of certificate issuers
        - name:                          # Issuer name (e.g., letsencrypt-prod)
          type:                          # Issuer type: letsencrypt | selfsigned | ca
          server:                        # ACME server URL (for letsencrypt type)
```

### DNS Provider Auto-Detection

When `dns_provider` is not set, it defaults based on `infrastructure.provider`:

| Infrastructure Provider | Default DNS Provider |
|------------------------|---------------------|
| openstack | designate |
| aws | route53 |

Other providers require explicit `dns_provider` configuration.

## Secrets

### Multi-Credential Configuration (Recommended)

Named credentials allow managing certificates across multiple accounts or zones.

**Route53 (AWS):**

```yaml
secrets:
  cert_manager:
    aws:
      <name>:                            # Credential set name (e.g., "production")
        enabled: true
        aws_access_key:                  # AWS access key ID
        aws_secret_access_key:           # AWS secret access key
        region:                          # AWS region for Route53 API
        dns_zones: []                    # Zones managed by this credential
```

**Cloudflare:**

```yaml
secrets:
  cert_manager:
    cloudflare:
      <name>:                            # Credential set name
        enabled: true
        api_token:                       # Cloudflare API token (scoped to DNS edit)
        dns_zones: []                    # Zones managed by this token
```

### Legacy Flat Fields

```yaml
secrets:
  cert_manager:
    aws_access_key:                      # AWS access key ID
    aws_secret_access_key:               # AWS secret access key
```

### Required Secrets by Provider

| DNS Provider | Required Secrets |
|-------------|-----------------|
| route53 | `aws_access_key` + `aws_secret_access_key` |
| cloudflare | `cloudflare_api_token` |
| clouddns | `gcp_service_account_key` |
| azuredns | `azure_client_id` + `azure_client_secret` + `azure_tenant_id` |
| designate | None (uses infrastructure credentials) |

## Dependencies

None. cert-manager is a foundational service with no service-level dependencies.

## Verification

```bash
# Check cert-manager pods
kubectl get pods -n cert-manager

# Verify webhook is ready
kubectl get deployment cert-manager-webhook -n cert-manager

# List ClusterIssuers
kubectl get clusterissuers

# Check issuer status
kubectl describe clusterissuer letsencrypt-prod

# List certificates across all namespaces
kubectl get certificates --all-namespaces

# Check certificate readiness
kubectl get certificates -A -o custom-columns='NAMESPACE:.metadata.namespace,NAME:.metadata.name,READY:.status.conditions[0].status'

# View recent certificate events
kubectl get events -n cert-manager --sort-by=.lastTimestamp --field-selector reason=Issuing
```

## CLI Commands

```bash
# Enable cert-manager
opencenter cluster service enable cert-manager

# Disable cert-manager
opencenter cluster service disable cert-manager

# View service status
opencenter cluster service status cert-manager

# Show configuration options
opencenter cluster service options cert-manager

# Set DNS provider
opencenter cluster set <cluster> services.cert-manager.dns_provider=route53

# Set ACME email
opencenter cluster set <cluster> services.cert-manager.email=ops@example.com
```
