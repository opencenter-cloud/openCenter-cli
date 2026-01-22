# cluster service


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Subcommands](#subcommands)
- [Service Types](#service-types)
- [Common Service Parameters](#common-service-parameters)
- [Service-Specific Configuration](#service-specific-configuration)
- [Validation](#validation)
- [Rendering](#rendering)
- [Use Cases](#use-cases)
- [See Also](#see-also)
**doc_type:** reference

Manage cluster services and their configurations.

## Synopsis

```bash
openCenter cluster service <subcommand> [flags]
```

## Description

The `cluster service` command manages services in a cluster's configuration. Services can be either standard services or managed services. Each service can have configuration parameters and secrets.

## Subcommands

### enable

Enable a service in the cluster configuration.

```bash
openCenter cluster service enable <service-name> [flags]
```

**Flags:**
- `--managed` - Enable as a managed service
- `--param stringArray` - Set service parameter (format: `key=value`)
- `--secret stringArray` - Set service secret (format: `key=value`)
- `--cluster string` - Specify cluster name
- `--force` - Force re-enable an already enabled service
- `--render` - Render service templates immediately after enabling

**Examples:**

```bash
# Enable cert-manager with required email parameter
openCenter cluster service enable cert-manager --param="email=admin@example.com"

# Enable managed service with secret
openCenter cluster service enable my-managed-service --managed --secret="api_key=secret123"

# Force re-enable (re-render) an already enabled service
openCenter cluster service enable prometheus --force

# Enable and immediately render templates
openCenter cluster service enable loki --render

# Enable with multiple parameters
openCenter cluster service enable loki \
  --param="loki_bucket_name=my-bucket" \
  --param="loki_storage_type=s3" \
  --secret="s3_access_key_id=AKIA..." \
  --secret="s3_secret_access_key=secret"
```

### disable

Disable a service in the cluster configuration.

```bash
openCenter cluster service disable <service-name> [flags]
```

**Flags:**
- `--managed` - Disable from managed services list
- `--cluster string` - Specify cluster name

**Examples:**

```bash
# Disable cert-manager service
openCenter cluster service disable cert-manager

# Disable managed service
openCenter cluster service disable my-managed-service --managed
```

### status

Display status of all services in the cluster.

```bash
openCenter cluster service status [flags]
```

**Flags:**
- `--cluster string` - Specify cluster name

**Examples:**

```bash
# Show status of all services in active cluster
openCenter cluster service status

# Show status for specific cluster
openCenter cluster service status --cluster my-cluster
```

**Output Format:**

```
SERVICE NAME                   ENABLED         STATUS
------------------------------  ---------------  ---------------
cert-manager                   enabled         deployed
loki                           enabled         pending
prometheus                     disabled        -
keycloak                       enabled         running
alert-proxy (managed)          enabled         success
```

### options

Display available configuration options for a service.

```bash
openCenter cluster service options <service-name> [flags]
```

**Flags:**
- `--managed` - Show options for a managed service

**Examples:**

```bash
# Show options for cert-manager
openCenter cluster service options cert-manager

# Show options for loki
openCenter cluster service options loki

# Show options for managed service
openCenter cluster service options alert-proxy --managed
```

**Output:**

```
Configuration options for service 'cert-manager':

Common Fields:
  enabled (boolean) - Enable or disable this service
  status (string) - Service deployment status (pending/running/success/failed)
  release (string) - Release version or tag (mutually exclusive with branch)
  branch (string) - Git branch (mutually exclusive with release)
  uri (string) - Git repository URI

Service-Specific Parameters:
  email (string) - Email address for Let's Encrypt certificate notifications [REQUIRED]
  letsencrypt_server (string) - LetsEncrypt ACME server URL
  region (string) - AWS region for Route53 DNS validation

Service-Specific Secrets:
  aws_access_key (string) - AWS access key for Route53 DNS validation
  aws_secret_access_key (string) - AWS secret access key for Route53 DNS validation

Usage Examples:
  openCenter cluster service enable cert-manager --param="email=value"
  openCenter cluster service enable cert-manager --secret="aws_access_key=secret-value"
```

## Service Types

### Standard Services

Standard services are deployed as part of the cluster infrastructure:

- `cert-manager` - Certificate management
- `loki` - Log aggregation
- `prometheus` - Metrics collection
- `grafana` - Metrics visualization
- `keycloak` - Identity and access management
- `headlamp` - Kubernetes dashboard
- `weave-gitops` - GitOps dashboard
- `kube-prometheus-stack` - Complete monitoring stack
- `velero` - Backup and disaster recovery
- `calico` - Network plugin

### Managed Services

Managed services are external services managed through GitOps:

- `alert-proxy` - Alert routing and management
- Custom managed services

## Common Service Parameters

All services support these common fields:

- `enabled` (boolean) - Enable or disable the service
- `status` (string) - Deployment status (pending, running, success, failed)
- `release` (string) - Release version or tag
- `branch` (string) - Git branch (mutually exclusive with release)
- `uri` (string) - Git repository URI

Managed services also support:

- `gitops_source_repo` (string) - GitOps source repository URL
- `gitops_source_release` (string) - GitOps source release tag
- `gitops_source_branch` (string) - GitOps source branch

## Service-Specific Configuration

### cert-manager

**Required Parameters:**
- `email` - Email for Let's Encrypt notifications

**Optional Parameters:**
- `letsencrypt_server` - ACME server URL
- `region` - AWS region for Route53

**Optional Secrets:**
- `aws_access_key` - AWS access key
- `aws_secret_access_key` - AWS secret key

### loki

**Required Parameters:**
- `loki_bucket_name` - Storage bucket/container name

**Optional Parameters:**
- `loki_storage_type` - Storage backend (s3 or swift)
- `loki_volume_size` - Persistent volume size in GB
- `swift_auth_url` - Swift authentication URL
- `swift_region` - Swift region
- `swift_application_credential_id` - Swift app credential ID
- `loki_s3_endpoint` - S3 endpoint URL
- `loki_s3_region` - S3 region

**Optional Secrets:**
- `swift_application_credential_secret` - Swift app credential secret
- `swift_password` - Swift password (legacy)
- `s3_access_key_id` - S3 access key
- `s3_secret_access_key` - S3 secret key

### keycloak

**Required Secrets:**
- `admin_password` - Keycloak admin password

**Optional Parameters:**
- `keycloak_realm` - Realm name
- `keycloak_frontend_url` - Frontend URL
- `keycloak_client_id` - Client ID

**Optional Secrets:**
- `client_secret` - OIDC client secret

### vsphere-csi

**Required Secrets:**
- `vcenter_host` - vCenter hostname or IP
- `username` - vCenter username
- `password` - vCenter password
- `datacenters` - Comma-separated datacenter list

**Optional Secrets:**
- `insecure_flag` - Skip SSL verification (true/false)
- `port` - vCenter port (default: 443)

## Validation

Services are validated when enabled:

- Required parameters must be provided
- Required secrets must be provided
- Parameter types are validated
- Mutual exclusivity rules are enforced

**Example Validation Error:**

```
Error: missing required parameter 'email' for service 'cert-manager'.
Example: --param="email=your-email@example.com"
```

## Rendering

When `--render` flag is used, service templates are immediately rendered to the GitOps directory:

```bash
openCenter cluster service enable loki --render
```

This is equivalent to:
```bash
openCenter cluster service enable loki
openCenter cluster render my-cluster
```

## Use Cases

### Enable Service with Configuration

```bash
# Enable cert-manager with email
openCenter cluster service enable cert-manager \
  --param="email=admin@example.com"

# Enable loki with S3 storage
openCenter cluster service enable loki \
  --param="loki_storage_type=s3" \
  --param="loki_bucket_name=my-loki-bucket" \
  --param="loki_s3_region=us-east-1" \
  --secret="s3_access_key_id=AKIA..." \
  --secret="s3_secret_access_key=secret"
```

### Check Service Status

```bash
# View all service statuses
openCenter cluster service status

# Check specific cluster
openCenter cluster service status --cluster prod-cluster
```

### Discover Service Options

```bash
# See what parameters a service accepts
openCenter cluster service options loki

# See managed service options
openCenter cluster service options alert-proxy --managed
```

### Update Service Configuration

```bash
# Force re-enable to update configuration
openCenter cluster service enable loki \
  --param="loki_volume_size=200" \
  --force \
  --render
```

## See Also

- [cluster render](render.md) - Render service templates
- [cluster setup](setup.md) - Setup GitOps directory structure
- [cluster validate](../cli-commands.md#cluster-validate) - Validate cluster configuration
