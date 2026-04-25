---
id: validation-rules
title: "Validation Rules"
sidebar_label: Validation Rules
description: Complete reference of configuration validation rules and constraints enforced by openCenter.
doc_type: reference
audience: "all users"
tags: [validation, rules, constraints, schema]
---

# Validation Rules

**Purpose:** For all users, provides complete reference of configuration validation rules and constraints enforced by openCenter.

This reference documents all validation rules applied to cluster configurations, organized by validation layer.

## Overview

openCenter uses multi-layered validation to catch errors early:

1. **Schema Validation:** Structure, types, formats (JSON schema)
2. **Business Rules:** Cross-field dependencies, logical consistency
3. **Provider Validation:** Provider-specific constraints
4. **Connectivity Validation:** API reachability (optional)

**Evidence:** `.kiro/steering/product.md:10`, Session 1 A3

## Validation Layers

### Layer 1: Schema Validation

Validates configuration structure, types, and formats against JSON schema.

**What's validated:**
- Field types (string, number, boolean, array, object)
- Required fields
- Field formats (email, URL, IP address, etc.)
- Enum values
- Min/max values
- String patterns (regex)

**Example violations:**

```yaml
# Invalid: worker_count is string, should be integer
opencenter:
  cluster:
    worker_count: "five"

# Error: Invalid type for opencenter.cluster.worker_count
# Expected: integer
# Actual: string
```

**Evidence:** `schema/cluster.schema.json:1-2382`, `internal/config/validator.go`

### Layer 2: Business Rules

Validates cross-field dependencies and logical consistency.

**What's validated:**
- Cross-field dependencies
- Logical consistency
- Value ranges
- Conditional requirements

**Example violations:**

```yaml
# Invalid: VRRP enabled but no VRRP IP
opencenter:
  cluster:
    networking:
      use_octavia: false
      vrrp_enabled: true
      # vrrp_ip: missing

# Error: When use_octavia=false and vrrp_enabled=true, vrrp_ip must be set
# Location: opencenter.cluster.networking.vrrp_ip
# Severity: error
```

**Evidence:** `tests/features/workflow.feature:38-43`, `internal/config/validator.go`

### Layer 3: Provider Validation

Validates provider-specific constraints.

**What's validated:**
- Provider resources exist (images, flavors, networks)
- Provider quotas sufficient
- Provider-specific requirements

**Example violations:**

```yaml
# Invalid: Image ID doesn't exist in OpenStack
opencenter:
  infrastructure:
    openstack:
      image_id: "invalid-image-id"

# Error: Image ID not found in OpenStack region sjc3
# Image ID: invalid-image-id
# Available images: [list of valid images]
```

**Evidence:** `internal/config/*_validator.go`, Session 1 A3

### Layer 4: Connectivity Validation (Optional)

Validates API reachability and credentials.

**What's validated:**
- API endpoints reachable
- Credentials valid
- Permissions sufficient

**Example violations:**

```yaml
# Invalid: OpenStack credentials incorrect
opencenter:
  infrastructure:
    openstack:
      username: "wrong-user"
      password: "wrong-password"

# Error: OpenStack authentication failed
# Auth URL: https://identity.api.rackspacecloud.com/v3
# Username: wrong-user
# Hint: Verify credentials in configuration
```

**Evidence:** `internal/config/validator.go`, Session 1 A3

## Schema Validation Rules

### Required Fields

Fields that must be present:

| Field | Required | Default |
|-------|----------|---------|
| `opencenter.meta.name` | Yes | - |
| `opencenter.meta.organization` | Yes | - |
| `opencenter.infrastructure.provider` | Yes | "openstack" |
| `opencenter.cluster.kubernetes.version` | Yes | "1.33.5" |

**Evidence:** `schema/cluster.schema.json`, `internal/config/defaults.go:48-56`

### Type Constraints

Field type requirements:

| Field | Type | Example |
|-------|------|---------|
| `opencenter.cluster.master_count` | integer | 3 |
| `opencenter.cluster.worker_count` | integer | 2 |
| `opencenter.cluster.kubernetes.version` | string | "1.33.5" |
| `opencenter.services.cert-manager.enabled` | boolean | true |
| `opencenter.cluster.networking.dns_nameservers` | array | ["8.8.8.8"] |

**Evidence:** `schema/cluster.schema.json`

### Format Constraints

String format requirements:

| Field | Format | Example |
|-------|--------|---------|
| `opencenter.infrastructure.openstack.auth_url` | URL | "https://..." |
| `opencenter.cluster.networking.pod_subnet` | CIDR | "10.42.0.0/16" |
| `opencenter.cluster.networking.service_subnet` | CIDR | "10.43.0.0/16" |
| `opencenter.services.keycloak.hostname` | hostname | "auth.example.com" |

**Evidence:** `schema/cluster.schema.json`

### Enum Constraints

Fields with allowed values:

| Field | Allowed Values |
|-------|----------------|
| `opencenter.infrastructure.provider` | "openstack", "vmware", "kind", "aws", "baremetal", "talos" |
| `opencenter.cluster.networking.cni_plugin` | "calico", "cilium", "kube-ovn" |
| `opencenter.meta.environment` | "development", "staging", "production" |

**Evidence:** `schema/cluster.schema.json`, `internal/config/defaults.go:27-31`

### Range Constraints

Numeric value ranges:

| Field | Min | Max | Default |
|-------|-----|-----|---------|
| `opencenter.cluster.master_count` | 1 | 10 | 3 |
| `opencenter.cluster.worker_count` | 0 | 100 | 2 |
| `opencenter.cluster.kubernetes.api_port` | 1 | 65535 | 443 |

**Evidence:** `schema/cluster.schema.json`

### Pattern Constraints

String pattern requirements (regex):

| Field | Pattern | Example |
|-------|---------|---------|
| `opencenter.meta.name` | `^[a-z0-9-]+$` | "my-cluster" |
| `opencenter.cluster.kubernetes.version` | `^\d+\.\d+\.\d+$` | "1.33.5" |

**Evidence:** `schema/cluster.schema.json`

## Business Rules

### Networking Rules

**Rule 1: VRRP IP Required**

When `use_octavia=false` and `vrrp_enabled=true`, `vrrp_ip` must be set.

```yaml
# Valid
opencenter:
  cluster:
    networking:
      use_octavia: false
      vrrp_enabled: true
      vrrp_ip: "192.168.1.100"

# Invalid
opencenter:
  cluster:
    networking:
      use_octavia: false
      vrrp_enabled: true
      # vrrp_ip: missing - ERROR
```

**Evidence:** `tests/features/workflow.feature:38-43`

**Rule 2: Subnet Non-Overlapping**

Pod subnet and service subnet must not overlap.

```yaml
# Valid
opencenter:
  cluster:
    networking:
      pod_subnet: "10.42.0.0/16"
      service_subnet: "10.43.0.0/16"

# Invalid
opencenter:
  cluster:
    networking:
      pod_subnet: "10.42.0.0/16"
      service_subnet: "10.42.0.0/16"  # Overlaps - ERROR
```

**Rule 3: Node Subnet Size**

Node subnet must be large enough for all nodes.

```yaml
# Valid: /22 = 1024 IPs, enough for 10 nodes
opencenter:
  cluster:
    master_count: 3
    worker_count: 5
  infrastructure:
    openstack:
      subnet_cidr: "10.2.128.0/22"

# Invalid: /28 = 16 IPs, not enough for 10 nodes
opencenter:
  cluster:
    master_count: 3
    worker_count: 5
  infrastructure:
    openstack:
      subnet_cidr: "10.2.128.0/28"  # Too small - ERROR
```

### Node Count Rules

**Rule 4: Master Count Odd**

Master count should be odd for HA (1, 3, 5, 7).

```yaml
# Valid
opencenter:
  cluster:
    master_count: 3  # Odd number

# Warning (not error)
opencenter:
  cluster:
    master_count: 4  # Even number - WARNING
```

**Rule 5: Minimum Masters for HA**

For production, master count should be >= 3.

```yaml
# Valid for production
opencenter:
  meta:
    environment: production
  cluster:
    master_count: 3

# Warning for production
opencenter:
  meta:
    environment: production
  cluster:
    master_count: 1  # Single master - WARNING
```

### Service Dependency Rules

**Rule 6: Service Dependencies**

Some services require other services.

```yaml
# Valid: Keycloak requires cert-manager
opencenter:
  services:
    cert-manager:
      enabled: true
    keycloak:
      enabled: true

# Invalid: Keycloak without cert-manager
opencenter:
  services:
    cert-manager:
      enabled: false
    keycloak:
      enabled: true  # Requires cert-manager - ERROR
```

**Evidence:** Ecosystem.md service dependencies

### Provider-Specific Rules

**Rule 7: OpenStack Image ID**

OpenStack image ID must exist in specified region.

```yaml
# Valid
opencenter:
  infrastructure:
    openstack:
      region: sjc3
      image_id: "799dcf97-3656-4361-8187-13ab1b295e33"  # Exists

# Invalid
opencenter:
  infrastructure:
    openstack:
      region: sjc3
      image_id: "invalid-id"  # Doesn't exist - ERROR
```

**Rule 8: VMware VM Inventory**

VMware node count must match VM inventory.

```yaml
# Valid
opencenter:
  cluster:
    master_count: 3
    worker_count: 3
  infrastructure:
    vmware:
      masters:
        - hostname: master-1
        - hostname: master-2
        - hostname: master-3
      workers:
        - hostname: worker-1
        - hostname: worker-2
        - hostname: worker-3

# Invalid
opencenter:
  cluster:
    master_count: 3
    worker_count: 3
  infrastructure:
    vmware:
      masters:
        - hostname: master-1
        - hostname: master-2
        # Missing master-3 - ERROR
      workers:
        - hostname: worker-1
        - hostname: worker-2
        - hostname: worker-3
```

**Rule 9: Kind Node Limits**

Kind clusters limited to 1 control plane, max 10 workers.

```yaml
# Valid
opencenter:
  infrastructure:
    kind:
      control_plane_nodes: 1
      worker_nodes: 5

# Invalid
opencenter:
  infrastructure:
    kind:
      control_plane_nodes: 3  # Max 1 - ERROR
      worker_nodes: 15  # Max 10 - ERROR
```

## Validation Severity Levels

### Error

Configuration is invalid, deployment will fail.

**Action:** Must fix before deployment.

**Example:**
```
Error: When use_octavia=false and vrrp_enabled=true, vrrp_ip must be set
Location: opencenter.cluster.networking.vrrp_ip
Severity: error
```

### Warning

Configuration is valid but not recommended.

**Action:** Review and consider fixing.

**Example:**
```
Warning: Master count is even (4), odd number recommended for HA
Location: opencenter.cluster.master_count
Severity: warning
```

### Info

Informational message, no action required.

**Action:** Informational only.

**Example:**
```
Info: Using default Kubernetes version 1.33.5
Location: opencenter.cluster.kubernetes.version
Severity: info
```

## Validation Commands

### Validate Configuration

```bash
# Validate cluster configuration
opencenter cluster validate my-cluster

# Validate with connectivity checks
opencenter cluster validate my-cluster --check-connectivity

# Validate specific configuration file
opencenter cluster validate my-cluster
```

### Validation Output

**Success:**
```
✓ Schema validation passed
✓ Business rules validation passed
✓ Provider validation passed

Configuration is valid and ready for deployment.
```

**Failure:**
```
✗ Schema validation failed
  Error: Invalid type for opencenter.cluster.worker_count
  Expected: integer
  Actual: string
  Location: opencenter.cluster.worker_count

✗ Business rules validation failed
  Error: When use_octavia=false and vrrp_enabled=true, vrrp_ip must be set
  Location: opencenter.cluster.networking.vrrp_ip

Configuration has 2 errors. Fix errors before deployment.
```

## Bypassing Validation

**Warning:** Bypassing validation can lead to deployment failures.

```bash
# Skip validation (not recommended)
opencenter cluster generate my-cluster --skip-validation

# Skip specific validation layer
opencenter cluster validate my-cluster --skip-provider-validation
```

**Use cases for bypassing:**
- Testing configuration changes
- Offline development (no provider API access)
- Known false positives

## Custom Validation Rules

Validation rules can be extended via plugins:

```go
// internal/core/validation/validators/custom_validator.go
type CustomValidator struct{}

func (v *CustomValidator) Validate(config *config.Config) []ValidationError {
    var errors []ValidationError
    
    // Custom validation logic
    if config.Cluster.WorkerCount > 50 {
        errors = append(errors, ValidationError{
            Field: "opencenter.cluster.worker_count",
            Message: "Worker count exceeds recommended maximum (50)",
            Severity: Warning,
        })
    }
    
    return errors
}
```

**Evidence:** `internal/core/validation/`, Session 1 A6

## Related Topics

- [Validate Configuration](../how-to/validate-configuration.md) - Validation procedures
- [Configuration Schema](configuration-schema.md) - Complete field reference
- [Troubleshoot Deployment](../how-to/troubleshoot-deployment.md) - Fix validation errors

---

## Evidence

This reference is based on:

- Validation layers: `.kiro/steering/product.md:10`, Session 1 A3
- Schema validation: `schema/cluster.schema.json:1-2382`
- Business rules: `tests/features/workflow.feature:38-43`, `internal/config/validator.go`
- Provider validation: `internal/config/*_validator.go`
- Service dependencies: Ecosystem.md
- Custom validators: `internal/core/validation/`, Session 1 A6
