# cluster preflight


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Arguments](#arguments)
- [Examples](#examples)
- [Checks Performed](#checks-performed)
- [Status Updates](#status-updates)
- [Output](#output)
- [Exit Codes](#exit-codes)
- [Common Issues](#common-issues)
- [Use Cases](#use-cases)
- [See Also](#see-also)
**doc_type:** reference

Run preflight checks for required tools and provider requirements.

## Synopsis

```bash
openCenter cluster preflight [name]
```

## Description

The `cluster preflight` command validates that all required tools are installed and accessible, and performs provider-specific connectivity and configuration checks before cluster deployment.

## Arguments

- `name` - Cluster name (optional if active cluster is set)

## Examples

```bash
# Run preflight checks for active cluster
openCenter cluster preflight

# Run for specific cluster
openCenter cluster preflight my-cluster
```

## Checks Performed

### Tool Availability

The command checks for the presence of required tools:

- `git` - Version control for GitOps repository
- `kubectl` - Kubernetes command-line tool
- `talosctl` - Talos Linux control tool

**Output:**
```
git: OK
kubectl: OK
talosctl: OK
```

**Missing Tool:**
```
git: OK
kubectl: MISSING
talosctl: OK
```

### Provider-Specific Checks

#### OpenStack

For OpenStack clusters, the command performs:

1. **Authentication URL Validation**
   - Verifies auth_url is configured
   - Tests connectivity to Keystone endpoint
   - Validates authentication endpoint accessibility

2. **API Connectivity**
   - Tests connection to OpenStack APIs
   - Verifies network reachability
   - Checks SSL certificate validity

**Output:**
```
OpenStack auth_url: https://keystone.example.com:5000/v3
OpenStack connectivity: OK
OpenStack API version: v3
```

**Connectivity Issues:**
```
OpenStack auth_url: https://keystone.example.com:5000/v3
OpenStack connectivity: FAILED - connection timeout
```

#### AWS

For AWS clusters (when implemented):
- AWS CLI availability
- Credential validation
- Region accessibility
- API connectivity

#### Kind

For Kind clusters:
- Docker or Podman availability
- Container runtime accessibility
- Network configuration

## Status Updates

On successful completion, cluster status is updated to:
- Stage: `preflight`
- Status: `success`

If status update fails, a warning is displayed but the command does not fail:
```
Warning: failed to update cluster status: <error>
```

## Output

Successful preflight check:
```
git: OK
kubectl: OK
talosctl: OK
OpenStack auth_url: https://keystone.example.com:5000/v3
OpenStack connectivity: OK
Preflight complete.
```

## Exit Codes

- `0` - All checks passed
- `1` - One or more checks failed
- `2` - Configuration error

## Common Issues

### Missing Tools

**Problem:**
```
kubectl: MISSING
```

**Solution:**
Install the missing tool:
```bash
# Install kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/
```

### OpenStack Connectivity Failed

**Problem:**
```
OpenStack connectivity: FAILED - connection timeout
```

**Solutions:**
1. Verify auth_url is correct in configuration
2. Check network connectivity to OpenStack endpoint
3. Verify firewall rules allow access
4. Check SSL certificate validity

### Invalid Configuration

**Problem:**
```
Error: failed to load cluster configuration: cluster "my-cluster" not found
```

**Solution:**
```bash
# Verify cluster exists
openCenter cluster list

# Initialize cluster if needed
openCenter cluster init my-cluster
```

## Use Cases

### Pre-Deployment Validation

Run preflight checks before deploying a cluster:
```bash
openCenter cluster preflight my-cluster
```

### CI/CD Integration

Include preflight checks in deployment pipelines:
```bash
#!/bin/bash
set -e

# Run preflight checks
openCenter cluster preflight my-cluster

# If successful, proceed with deployment
openCenter cluster bootstrap my-cluster
```

### Troubleshooting

Diagnose environment issues:
```bash
# Check tool availability
openCenter cluster preflight my-cluster

# Verify specific tool
which kubectl
kubectl version --client
```

## See Also

- [cluster validate](../cli-commands.md#cluster-validate) - Validate cluster configuration
- [cluster bootstrap](bootstrap.md) - Bootstrap cluster infrastructure
- [cluster setup](setup.md) - Setup GitOps directory structure
