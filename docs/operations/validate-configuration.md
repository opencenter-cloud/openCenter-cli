---
id: validate-configuration
title: "Validate Configuration"
sidebar_label: Validate Configuration
description: How to validate cluster configuration before deployment to catch errors early.
doc_type: how-to
audience: "all users"
tags: [validation, configuration, schema, errors]
---
# Validate Configuration

**Purpose:** For all users, shows how to validate cluster configuration before deployment, covering validation types and error resolution.

Validation catches configuration errors before you deploy, saving time and preventing failed deployments. This guide shows you how to validate your configuration and fix common errors.

## Prerequisites

* openCenter CLI installed
* Cluster configuration file created (via `opencenter cluster init`)
* Active cluster useed (via `opencenter cluster use`)

## Basic Validation

Validate your active cluster configuration:

```bash
opencenter cluster validate
```

This performs:

* Schema validation against JSON schema
* Required field validation
* Cross-field dependency validation
* GitOps configuration and local repository validation
* Network configuration validation
* SOPS key validation

Expected output for valid configuration:

```
✓ Validation successful

Cluster: opencenter/my-cluster
Organization: opencenter
Provider: openstack
Validation mode: offline

Summary: passed
```

## Validate Specific Cluster

Validate a cluster without selecting it:

```bash
opencenter cluster validate my-cluster
```

Or validate a specific configuration file:

```bash
opencenter cluster validate my-cluster
```

## Validation Modes

Validation uses one mode per run. The default comes from global CLI config:

```bash
opencenter settings get behavior.validation
```

The default is `offline`, which never contacts cloud providers, Git remotes,
Kubernetes APIs, or other external services. Offline validation covers schema
rules, required fields, readiness rules, enabled-service settings, GitOps URL
and auth shape, local GitOps path checks, and local git status.

Use `online` for one run when you want provider and Git remote checks:

```bash
opencenter cluster validate --validation online
```

To change the default mode for future runs:

```bash
opencenter settings set behavior.validation online
```

Online validation runs the offline profile plus provider discovery/connectivity
and Git remote checks.

## Verbose Output

See detailed validation information:

```bash
opencenter cluster validate --verbose
```

This shows:

* Each validation step
* Field-by-field checks
* Warning messages
* Suggestions for improvements

## Generate Debug Configuration

Create a complete configuration file for debugging:

```bash
opencenter cluster validate --generate-debug-config
```

This creates a file with:

* All configuration values (including defaults)
* Resolved template variables
* Computed fields
* Validation results

Output location: `./debug-config-<cluster>-<timestamp>.yaml`

Specify custom output directory:

```bash
opencenter cluster validate --generate-debug-config --output-dir /tmp
```

## Common Validation Errors

### Missing Required Fields

**Error:**

```
ERROR: Required field missing: opencenter.infrastructure.cloud.openstack.application_credential_id
```

**Fix:** Add the required field to your configuration:

```yaml
opencenter:
  infrastructure:
    cloud:
      openstack:
        application_credential_id: "your-app-cred-id"
```

### Invalid CIDR Format

**Error:**

```
ERROR: Invalid CIDR format: opencenter.cluster.kubernetes.subnet_pods
Value: "10.42.0.0/33"
```

**Fix:** Use valid CIDR notation (prefix length 0-32):

```yaml
opencenter:
  cluster:
    kubernetes:
      subnet_pods: "10.42.0.0/16"
```

### Overlapping Subnets

**Error:**

```
ERROR: Subnet overlap detected
Field: opencenter.cluster.kubernetes.subnet_services
Overlaps with: opencenter.cluster.kubernetes.subnet_pods
```

**Fix:** Use non-overlapping CIDR ranges:

```yaml
opencenter:
  cluster:
    kubernetes:
      subnet_pods: "10.42.0.0/16"
      subnet_services: "10.43.0.0/16"
```

### VRRP IP Required

**Error:**

```
ERROR: vrrp_ip must be set when use_octavia=false and vrrp_enabled=true
Field: opencenter.cluster.networking.vrrp_ip
```

**Fix:** Set VRRP IP address:

```yaml
opencenter:
  cluster:
    networking:
      use_octavia: false
      vrrp_enabled: true
      vrrp_ip: "10.0.0.10"
```

### Invalid Kubernetes Version

**Error:**

```
ERROR: Invalid Kubernetes version format
Field: opencenter.cluster.kubernetes.version
Value: "1.33"
Expected: "1.33.5" (semantic version)
```

**Fix:** Use full semantic version:

```yaml
opencenter:
  cluster:
    kubernetes:
      version: "1.33.5"
```

### CNI Plugin Conflict

**Error:**

```
ERROR: Multiple CNI plugins enabled
Only one CNI plugin can be enabled at a time
Enabled: calico, cilium
```

**Fix:** Enable only one CNI plugin:

```yaml
opencenter:
  cluster:
    kubernetes:
      network_plugin:
        calico:
          enabled: true
        cilium:
          enabled: false
```

### Missing SOPS Key

**Error:**

```
ERROR: SOPS age key file not found
Field: secrets.sopsAgeKeyFile
Path: ~/.config/sops/age/my-cluster-key.txt
```

**Fix:** Generate SOPS key:

```bash
opencenter secrets keys generate
```

Or specify existing key:

```yaml
secrets:
  sopsAgeKeyFile: "/path/to/existing/key.txt"
```

### Invalid Provider Configuration

**Error:**

```
ERROR: Provider-specific validation failed
Provider: openstack
Issue: Image ID not found: 799dcf97-3656-4361-8187-13ab1b295e33
```

**Fix:** Use valid image ID from your OpenStack cloud:

```bash
# List available images
openstack image list

# Update configuration
opencenter cluster set my-cluster opencenter.infrastructure.cloud.openstack.image_id=<valid-image-id>
```

## Validation Workflow

Recommended validation workflow before deployment:

1. **Basic validation:**

   ```bash
   opencenter cluster validate
   ```
2. **Fix any errors** shown in output
3. **Run online validation when credentials and network access are available:**

   ```bash
   opencenter cluster validate --validation online
   ```
4. **Generate debug config** (if needed):

   ```bash
   opencenter cluster validate --generate-debug-config
   ```
5. **Proceed to setup:**

   ```bash
   opencenter cluster generate
   ```

## Validation in CI/CD

Integrate validation into your CI/CD pipeline:

```bash
#!/bin/bash
set -e

# Validate configuration
opencenter cluster validate my-cluster

# Run online checks if credentials and network access are available
if [ -n "$OPENSTACK_APPLICATION_CREDENTIAL_ID" ]; then
  opencenter cluster validate my-cluster --validation online
fi

# Generate debug config for artifacts
opencenter cluster validate my-cluster \
  --generate-debug-config \
  --output-dir ./artifacts
```

## Troubleshooting

### Validation Hangs

**Problem:** Validation command appears stuck

**Cause:** Online validation is waiting on provider or Git remote checks

**Solution:** Use offline validation:

```bash
opencenter cluster validate --validation offline
```

### False Positive Errors

**Problem:** Validation reports errors for valid configuration

**Cause:** Schema version mismatch

**Solution:** Check schema version:

```yaml
schema_version: "2.0"  # Must be "2.0"
```

Any other `schema_version` value is invalid.

### Provider Validation Fails

**Problem:** Provider-specific validation fails with credentials error

**Cause:** Credentials not accessible or expired

**Solution:** Verify credentials:

```bash
# OpenStack
openstack token issue

# AWS
aws sts get-caller-identity
```

Update credentials in configuration if expired.

## Next Steps

After successful validation:

* [Setup GitOps Repository](../tutorials/getting-started.md#step-6-setup-gitops-repository)
* [Bootstrap Cluster](../tutorials/getting-started.md#step-8-bootstrap-cluster)
* [Customize Services](operations/customize-services.md)
* [Configure Networking](operations/configure-networking.md)

---

## Evidence

This how-to guide is based on:

* Validation command: `cmd/cluster_validate.go:1-108`
* Validation options: `cmd/cluster_validate.go:66-74`
* Validation flags: `cmd/cluster_validate.go:100-105`
* Workflow validation: `tests/features/workflow.feature:38-50`
* Product validation layers: `.kiro/steering/product.md:10`
* Session 1 validation review: A7
* Session 2 facts inventory: B0 section 13