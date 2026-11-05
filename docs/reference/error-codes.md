---
last_updated: 2026-11-05
id: error-codes
title: "Error Codes"
sidebar_label: Error Codes
description: Reference of the structured error codes registered in internal/ui/error_formatter.go, with remediation for each.
doc_type: reference
audience: "users, operators, contributors"
tags: [errors, error-codes, diagnostics, reference]
---

# Error Codes

**Purpose:** For users and operators, lists every structured error
code the CLI can report, what it means, and how to recover. Process
exit codes are a separate concern and are covered by
[Exit Codes](exit-codes.md); this page covers the `E*` codes only.

## Status of this registry

These codes are defined in the `errorRegistry` of
`internal/ui/error_formatter.go`, which `internal/di.ProvideErrorFormatter`
constructs and `internal/di.AppContainer` exposes. As of the `last_updated`
date, **no command calls `FormatWithCode`, `FormatWithErrorInfo`, or
`GetErrorInfo`**, so the CLI does not currently print these codes or the
`DocsURL` field. The registry is wired but dormant.

Two consequences you should know when reading this page:

- If you never see an `E*` code in your terminal output, that is the
  current behaviour, not a missing installation step.
- Commands that do report problems today emit plain error text with an
  inline remediation hint, not a code. See
  [Troubleshooting Deployment Failures](../operations/troubleshoot-deployment.md).

## Configuration errors

| Code | Title | Severity |
| --- | --- | --- |
| [`E1001`](#e1001) | OpenStack region not configured | Critical |
| [`E1002`](#e1002) | SOPS key not found | Critical |
| [`E1003`](#e1003) | Invalid cluster name | Critical |
| [`E1004`](#e1004) | Configuration validation failed | Critical |
| [`E1005`](#e1005) | Required field missing | Critical |

### E1001

**OpenStack region not configured** (critical severity)

The OpenStack provider requires a region to be specified.

**Remediation:** Add region to your configuration:

```bash
opencenter cluster set {cluster} opencenter.infrastructure.cloud.openstack.region=RegionOne
```

**Note:** List available regions: openstack region list

### E1002

**SOPS key not found** (critical severity)

The SOPS Age encryption key for this cluster could not be found.

**Remediation:** Generate a new SOPS key:

```bash
opencenter secrets keys generate
```

**Note:** Check if the key file exists: ls -la ~/.config/opencenter/clusters/{org}/{cluster}/secrets/age/

### E1003

**Invalid cluster name** (critical severity)

Cluster names must start with a letter and contain only alphanumeric characters, hyphens, and underscores (max 63 characters).

**Remediation:** Use a valid cluster name:

```bash
opencenter cluster init my-cluster
```

**Note:** Valid examples: my-cluster, prod_cluster, cluster123

### E1004

**Configuration validation failed** (critical severity)

The cluster configuration contains validation errors.

**Remediation:** Run validation to see specific errors:

```bash
opencenter cluster validate {cluster}
```

**Note:** Edit the cluster configuration after reviewing validation output.

### E1005

**Required field missing** (critical severity)

A required configuration field is missing or empty.

**Remediation:** Edit the configuration and add the required field:

```bash
opencenter cluster edit {cluster}
```

**Note:** Run validation again after adding the missing field.

## Validation errors

| Code | Title | Severity |
| --- | --- | --- |
| [`E2001`](#e2001) | Command injection attempt detected | Critical |
| [`E2002`](#e2002) | Template injection attempt detected | Critical |
| [`E2003`](#e2003) | Path traversal attempt detected | Critical |
| [`E2004`](#e2004) | Invalid EDITOR environment variable | Critical |

### E2001

**Command injection attempt detected** (critical severity)

The input contains shell metacharacters that could be used for command injection.

**Remediation:** Remove shell metacharacters from the input.

**Note:** Avoid using characters like: ; | & < > $ ` \\ in names and paths

### E2002

**Template injection attempt detected** (critical severity)

The template contains dangerous functions that are not allowed.

**Remediation:** Remove dangerous template functions.

**Note:** Allowed functions: upper, lower, trim, replace, split, join, printf, quote

### E2003

**Path traversal attempt detected** (critical severity)

The path contains sequences that could be used for path traversal attacks.

**Remediation:** Use a path without .. or absolute paths.

**Note:** Paths should be relative to the configuration directory

### E2004

**Invalid EDITOR environment variable** (critical severity)

The EDITOR environment variable contains an unsafe value.

**Remediation:** Set EDITOR to a safe editor:

```bash
export EDITOR=vim
```

**Note:** Allowed editors: vim, nano, emacs, vi, code, subl

## Secrets errors

| Code | Title | Severity |
| --- | --- | --- |
| [`E3001`](#e3001) | Network timeout | Warning |
| [`E3002`](#e3002) | Connection refused | Warning |

### E3001

**Network timeout** (warning severity)

The operation timed out while waiting for a network response.

**Remediation:** Check network connectivity and retry:

```bash
ping {host}
```

**Note:** Verify firewall rules and network configuration

### E3002

**Connection refused** (warning severity)

The connection to the remote service was refused.

**Remediation:** Verify the service is running:

```bash
systemctl status {service}
```

**Note:** Check if the service is listening on the expected port

## GitOps errors

| Code | Title | Severity |
| --- | --- | --- |
| [`E4001`](#e4001) | File not found | Critical |
| [`E4002`](#e4002) | Permission denied | Critical |
| [`E4003`](#e4003) | Disk space exhausted | Critical |

### E4001

**File not found** (critical severity)

The specified file or directory does not exist.

**Remediation:** Verify the path exists:

```bash
ls -la {path}
```

**Note:** Check for typos in the file path

### E4002

**Permission denied** (critical severity)

You do not have permission to access this file or directory.

**Remediation:** Grant appropriate permissions:

```bash
chmod +w {path}
```

**Note:** Check file ownership: ls -la {path}

### E4003

**Disk space exhausted** (critical severity)

There is not enough disk space to complete the operation.

**Remediation:** Free up disk space:

```bash
df -h
```

**Note:** Remove old logs or unused files

## Provider errors

| Code | Title | Severity |
| --- | --- | --- |
| [`E5001`](#e5001) | OpenStack API error | Critical |
| [`E5002`](#e5002) | Unsupported provider API error | Critical |
| [`E5003`](#e5003) | Provider authentication failed | Critical |

### E5001

**OpenStack API error** (critical severity)

An error occurred while communicating with the OpenStack API.

**Remediation:** Verify OpenStack credentials and connectivity:

```bash
openstack server list
```

**Note:** Run the local prerequisite audit: opencenter cluster doctor

### E5002

**Unsupported provider API error** (critical severity)

An error occurred while communicating with a provider API. AWS, GCP, and Azure are not available providers and are rejected at configuration time.

**Remediation:** Select a supported provider (OpenStack, VMware, Baremetal, Kind, Magnum):

```bash
opencenter cluster set {cluster} opencenter.infrastructure.provider=openstack
```

**Note:** See docs/reference/providers.md for the provider boundary

### E5003

**Provider authentication failed** (critical severity)

Authentication with the cloud provider failed.

**Remediation:** Verify your credentials are correct.

**Note:** Check credential expiration and permissions

## Operational errors

| Code | Title | Severity |
| --- | --- | --- |
| [`E6001`](#e6001) | Drift detection failed | Warning |
| [`E6002`](#e6002) | Backup creation failed | Critical |
| [`E6003`](#e6003) | Lock acquisition failed | Warning |
| [`E6004`](#e6004) | Retry budget exhausted | Critical |

### E6001

**Drift detection failed** (warning severity)

Unable to detect configuration drift for the cluster.

**Remediation:** Verify cluster is accessible:

```bash
opencenter cluster describe {cluster}
```

**Note:** Check cloud provider connectivity

### E6002

**Backup creation failed** (critical severity)

Unable to create a backup of the cluster configuration.

**Remediation:** Check disk space and permissions:

```bash
df -h && ls -la ~/.config/opencenter/backups/
```

**Note:** Ensure the backup directory is writable

### E6003

**Lock acquisition failed** (warning severity)

Unable to acquire a lock for the cluster operation.

**Remediation:** Wait for the current operation to complete, or unlock the cluster:

```bash
opencenter cluster unlock {cluster} --reason "stale lock"
```

**Note:** Check if another operation is in progress

### E6004

**Retry budget exhausted** (critical severity)

The operation failed after exhausting all retry attempts.

**Remediation:** Check the underlying error and retry manually.

**Note:** The service may be experiencing issues

## Related

- [Exit Codes](exit-codes.md) - process exit codes for scripting and CI.
- [Troubleshooting Deployment Failures](../operations/troubleshoot-deployment.md) - resolving problems you can actually observe today.
- [Provider Reference](providers.md) - the supported provider boundary.
