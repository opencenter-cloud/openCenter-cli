# Adding Services to Your Cluster


## Table of Contents

- [Who This Is For](#who-this-is-for)
- [Service Architecture](#service-architecture)
- [Available Services](#available-services)
- [Enabling Services](#enabling-services)
- [Service Configuration Patterns](#service-configuration-patterns)
- [Adding Custom Services](#adding-custom-services)
- [Template Variables](#template-variables)
- [Service Patterns](#service-patterns)
- [Secrets Management](#secrets-management)
- [Validation](#validation)
- [Best Practices](#best-practices)
- [Complete Examples](#complete-examples)
- [Troubleshooting](#troubleshooting)
- [Related Documentation](#related-documentation)
**doc_type: how-to**

This guide shows you how to enable, configure, and add services to your openCenter cluster. Services are Kubernetes applications deployed and managed through FluxCD.

## Who This Is For

Cluster operators who need to enable built-in services or add custom services to their openCenter deployment.

## Service Architecture

openCenter uses a registry-based service system where each service:
1. Has a configuration type registered in `internal/config/services/`
2. Stores manifests in `internal/gitops/templates/cluster-apps-base/services/<service-name>/`
3. Defines FluxCD Kustomization in `internal/gitops/templates/cluster-apps-base/services/fluxcd/<service-name>.yaml.tpl`
4. Specifies GitRepository source in `internal/gitops/templates/cluster-apps-base/services/sources/opencenter-<service-name>.yaml.tpl`
5. Validates configuration through JSON schema

## Available Services

openCenter includes these built-in services:

**Networking & Ingress**
- `calico` - Calico CNI with network policies
- `gateway` - Kubernetes Gateway API implementation
- `gateway-api` - Gateway API CRDs

**Storage**
- `external-snapshotter` - Volume snapshot controller
- `openstack-csi` - OpenStack Cinder CSI driver
- `vsphere-csi` - vSphere CSI driver

**Security & Identity**
- `cert-manager` - TLS certificate management
- `keycloak` - Identity and access management
- `kyverno` - Policy engine

**Observability**
- `loki` - Log aggregation
- `prometheus-stack` - Prometheus, Grafana, Alertmanager
- `alert-proxy` - Alert routing proxy
- `headlamp` - Kubernetes dashboard

**GitOps & Deployment**
- `fluxcd` - GitOps continuous delivery
- `weave-gitops` - Weave GitOps UI
- `sources` - FluxCD source definitions

**Backup & Recovery**
- `velero` - Cluster backup and restore
- `etcd-backup` - etcd backup automation

**Operators**
- `olm` - Operator Lifecycle Manager
- `postgres-operator` - PostgreSQL operator
- `rbac-manager` - RBAC management

## Enabling Services

Add services to your cluster configuration under `opencenter.services`:

```yaml
opencenter:
  services:
    cert-manager:
      enabled: true
      email: admin@example.com
      letsencrypt_server: https://acme-v02.api.letsencrypt.org/directory
    
    loki:
      enabled: true
      namespace: loki
      loki_storage_type: swift
      loki_bucket_name: my-loki-logs
      loki_volume_size: 50
    
    prometheus-stack:
      enabled: true
      grafana_volume_size: 10
      prometheus_volume_size: 50
```

Run `openCenter cluster init` to generate the GitOps repository with your enabled services.

## Service Configuration Patterns

### Simple Services (BaseConfig)

Services with only enable/disable and basic fields:

```yaml
opencenter:
  services:
    fluxcd:
      enabled: true
      namespace: flux-system
    
    gateway-api:
      enabled: true
```

Available fields:
- `enabled` - Enable or disable the service
- `status` - Deployment status (managed by openCenter)
- `namespace` - Kubernetes namespace
- `hostname` - HTTPRoute hostname
- `image_repository` - Custom image repository
- `image_tag` - Custom image tag
- `release` - Release version
- `branch` - Git branch
- `uri` - Git repository URI
- `gitops_source_repo` - GitOps source repository URL
- `gitops_source_release` - GitOps source release tag
- `gitops_source_branch` - GitOps source branch

### Loki (LokiConfig)

Log aggregation with object storage:

```yaml
opencenter:
  services:
    loki:
      enabled: true
      namespace: loki
      loki_storage_type: swift  # or s3
      loki_bucket_name: cluster-logs
      loki_volume_size: 50
      loki_storage_class: standard
      
      # Swift storage
      swift_auth_url: https://keystone.example.com:5000/v3
      swift_region: RegionOne
      swift_auth_version: 3
      swift_application_credential_id: abc123
      swift_container_name: loki-logs
      swift_user_domain_name: Default
      
      # S3 storage (alternative)
      loki_s3_endpoint: https://s3.example.com
      loki_s3_region: us-east-1
      loki_s3_force_path_style: false
      loki_s3_insecure: false
```

### Prometheus Stack (PrometheusStackConfig)

Monitoring with Prometheus, Grafana, and Alertmanager:

```yaml
opencenter:
  services:
    prometheus-stack:
      enabled: true
      namespace: monitoring
      grafana_volume_size: 10
      grafana_storage_class: standard
      prometheus_volume_size: 50
      prometheus_storage_class: standard
      alertmanager_volume_size: 5
      alertmanager_storage_class: standard
      webhook_url: https://alerts.example.com/webhook
```

### Cert-Manager (CertManagerConfig)

TLS certificate management:

```yaml
opencenter:
  services:
    cert-manager:
      enabled: true
      namespace: cert-manager
      email: admin@example.com
      letsencrypt_server: https://acme-v02.api.letsencrypt.org/directory
```

### Keycloak (KeycloakConfig)

Identity and access management:

```yaml
opencenter:
  services:
    keycloak:
      enabled: true
      namespace: keycloak
      hostname: auth.example.com
      keycloak_realm: kubernetes
      keycloak_frontend_url: https://auth.example.com
      keycloak_client_id: kubernetes
```

### Velero (VeleroConfig)

Cluster backup and restore:

```yaml
opencenter:
  services:
    velero:
      enabled: true
      namespace: velero
      velero_backup_bucket: cluster-backups
      velero_region: us-east-1
```

### vSphere CSI (VSphereCSIConfig)

vSphere storage driver:

```yaml
opencenter:
  services:
    vsphere-csi:
      enabled: true
      namespace: vmware-system-csi
      image_repository: registry.k8s.io/csi-vsphere
      image_tag: v3.3.0

secrets:
  vsphere_csi:
    vcenter_host: vcenter.example.com
    username: administrator@vsphere.local
    password: your-password
    datacenters: Datacenter1
    insecure_flag: "false"
    port: "443"
```

### Calico (CalicoConfig)

CNI with network policies:

```yaml
opencenter:
  services:
    calico:
      enabled: true
      namespace: calico-system
      calico_kube_api_server: https://api.cluster.example.com:6443
```

### Headlamp (HeadlampConfig)

Kubernetes dashboard with OIDC:

```yaml
opencenter:
  services:
    headlamp:
      enabled: true
      namespace: headlamp
      hostname: dashboard.example.com
      headlamp_oidc_issuer_url: https://auth.example.com/realms/kubernetes
      headlamp_oidc_client_id: headlamp
```

### Alert Proxy (AlertProxyConfig)

Alert routing:

```yaml
opencenter:
  services:
    alert-proxy:
      enabled: true
      namespace: monitoring
      alert_manager_base_url: http://alertmanager:9093
      http_route_fqdn: alerts.example.com
```

### Weave GitOps (WeaveGitOpsConfig)

GitOps UI:

```yaml
opencenter:
  services:
    weave-gitops:
      enabled: true
      namespace: flux-system
      hostname: gitops.example.com
```

### etcd Backup (EtcdBackupConfig)

Automated etcd backups:

```yaml
opencenter:
  services:
    etcd-backup:
      enabled: true
      namespace: kube-system
```

## Adding Custom Services

Follow these steps to add a new service to openCenter.

### Prerequisites

- Go 1.25.2 or later
- Mise installed
- openCenter source code

### Step 1: Create Service Configuration Type

Create a new file in `internal/config/services/<service-name>.go`:

```go
package services

import (
	"github.com/rackerlabs/openCenter-cli/internal/config/registry"
)

// MyServiceConfig extends BaseConfig with service-specific configuration
type MyServiceConfig struct {
	BaseConfig `yaml:",inline"`

	// Add service-specific fields
	CustomField string `yaml:"custom_field,omitempty" json:"custom_field,omitempty" jsonschema:"description=Custom field description"`
}

func init() {
	registry.RegisterServiceConfig("my-service", MyServiceConfig{})
}
```

For simple services without custom fields, register with `DefaultServiceConfig`:

```go
func init() {
	registry.RegisterServiceConfig("my-service", DefaultServiceConfig{})
}
```

### Step 2: Create Service Manifests

Create manifests directory:

```bash
mkdir -p internal/gitops/templates/cluster-apps-base/services/my-service
```

Add a `kustomization.yaml` (static) or `kustomization.yaml.tpl` (templated):

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: {{ .OpenCenter.Services.my-service.Namespace | default "my-service" }}
resources:
  - https://github.com/example/my-service/releases/download/1.0.0/install.yaml
```

### Step 3: Create FluxCD Kustomization

Create `internal/gitops/templates/cluster-apps-base/services/fluxcd/my-service.yaml.tpl`:

```yaml
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: my-service-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
  interval: 15m
  sourceRef:
    kind: GitRepository
    name: opencenter-my-service
  path: applications/base/services/my-service
  targetNamespace: {{ .OpenCenter.Services.my-service.Namespace | default "my-service" }}
  prune: true
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: my-service-override
  namespace: flux-system
spec:
  interval: 15m
  sourceRef:
    kind: GitRepository
    name: flux-system
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  path: ./applications/overlays/{{ .ClusterName }}/services/my-service
  targetNamespace: {{ .OpenCenter.Services.my-service.Namespace | default "my-service" }}
  prune: true
```

### Step 4: Create GitRepository Source

Create `internal/gitops/templates/cluster-apps-base/services/sources/opencenter-my-service.yaml.tpl`:

```yaml
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-my-service
  namespace: flux-system
spec:
  interval: 15m
  url: {{ .OpenCenter.GitOps.GitOpsBaseRepo }}
  ref:
    {{- if .OpenCenter.GitOps.GitOpsBaseRelease }}
    tag: {{ .OpenCenter.GitOps.GitOpsBaseRelease }}
    {{- else }}
    branch: {{ .OpenCenter.GitOps.GitOpsBranch | default "main" }}
    {{- end }}
  secretRef:
    name: opencenter-base
```

### Step 5: Update Kustomization Templates

Add conditional includes to `internal/gitops/templates/cluster-apps-base/services/sources/kustomization.yaml.tpl`:

```yaml
resources:
  {{- if index .OpenCenter.Services "my-service" }}
  {{- if (index .OpenCenter.Services "my-service").Enabled }}
  - opencenter-my-service.yaml
  {{- end }}
  {{- end }}
```

Add to `internal/gitops/templates/cluster-apps-base/services/fluxcd/kustomization.yaml.tpl`:

```yaml
resources:
  {{- if index .OpenCenter.Services "my-service" }}
  {{- if (index .OpenCenter.Services "my-service").Enabled }}
  - my-service.yaml
  {{- end }}
  {{- end }}
```

### Step 6: Build and Test

```bash
# Build the CLI
mise run build

# Create test configuration
cat > test-config.yaml <<EOF
opencenter:
  services:
    my-service:
      enabled: true
      namespace: my-service
      custom_field: example-value
EOF

# Generate GitOps repository
./bin/openCenter cluster init --config test-config.yaml

# Verify generated files
ls -la ~/.config/openCenter/clusters/*/my-cluster/gitops/
```

### Step 7: Validate Schema

```bash
# Regenerate JSON schema
mise run schema

# Validate configuration
./bin/openCenter cluster validate --config test-config.yaml
```

## Template Variables

When using `.tpl` files, you have access to the full cluster configuration:

- `.OpenCenter.Services.<service-name>` - Service-specific configuration
- `.OpenCenter.Cluster.ClusterName` - Cluster name
- `.OpenCenter.GitOps.GitOpsBaseRepo` - Base repository URL
- `.OpenCenter.GitOps.GitOpsBaseRelease` - Base repository release tag
- `.OpenCenter.GitOps.GitOpsBranch` - Base repository branch
- `.ClusterName` - Shorthand for cluster name

## Service Patterns

### Pattern 1: Simple Service (No Configuration)

For services that just need to be enabled/disabled:

1. Static `kustomization.yaml` (no `.tpl`)
2. FluxCD Kustomization with `.tpl` for conditional inclusion
3. GitRepository source with `.tpl` for repo configuration

### Pattern 2: Configurable Service

For services requiring runtime configuration:

1. Templated `kustomization.yaml.tpl` with config values
2. Additional config files (secrets, configmaps) with `.tpl`
3. FluxCD Kustomization with `.tpl`
4. GitRepository source with `.tpl`

### Pattern 3: Helm-Based Service

For services deployed via Helm:

1. `kustomization.yaml` with HelmRelease reference
2. Helm values in `helm-values/override-values.yaml` or `.tpl`
3. FluxCD Kustomization with health checks for HelmRelease
4. GitRepository source

## Secrets Management

For services requiring secrets:

1. Add secret fields to `Secrets` struct in `internal/config/config.go`
2. Create SOPS-encrypted secret manifests with `.yaml` extension
3. Reference secrets in service manifests
4. Add validation in `internal/config/validator.go`

Example secret file:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: <service-name>-secret
  namespace: <service-namespace>
type: Opaque
data:
  key: ENC[AES256_GCM,data:...,iv:...,tag:...,type:str]
sops:
  # SOPS metadata
```

## Validation

Add validation rules in `internal/config/validator.go` to ensure:

1. Required secrets are present when service is enabled
2. Configuration values are valid
3. Dependencies between services are satisfied

Example:

```go
func (v *Validator) validateServiceName(ctx context.Context, config *Config) []*errors.StructuredError {
    if config.OpenCenter.Services["<service-name>"].Enabled {
        if config.Secrets.<ServiceName>.RequiredField == "" {
            return []*errors.StructuredError{
                errors.NewStructuredError(
                    "missing_secret",
                    "secrets.<service-name>.required_field is required when <service-name> is enabled",
                    "secrets.<service-name>.required_field",
                ),
            }
        }
    }
    return nil
}
```

## Best Practices

1. **Use consistent naming**: Follow the pattern `<service-name>` throughout
2. **Add health checks**: Include health checks in FluxCD Kustomizations
3. **Document dependencies**: Use `dependsOn` in FluxCD Kustomizations
4. **Version control**: Use specific versions in resource URLs
5. **Test thoroughly**: Test with both enabled and disabled states
6. **Add examples**: Include example configurations in `testdata/`
7. **Update documentation**: Document service-specific configuration options

## Complete Examples

### Example 1: vsphere-csi Service (Raw Manifests with Templating)

The vsphere-csi service demonstrates a fully templated service that uses raw Kubernetes manifests from upstream:

**Files:**
- Manifests: `internal/gitops/templates/cluster-apps-base/services/vsphere-csi/`
  - `kustomization.yaml.tpl` - Templated kustomization with version and image configuration
  - `vsphere-config-secret.yaml.tpl` - Templated secret with vSphere credentials
- FluxCD: `internal/gitops/templates/cluster-apps-base/services/fluxcd/vsphere-csi.yaml.tpl`
- Source: `internal/gitops/templates/cluster-apps-base/services/sources/opencenter-vsphere-csi.yaml.tpl`
- Config: Custom fields in `ServiceCfg` (Namespace, ImageRepository, ImageTag)
- Secrets: `VSphereCsiSecrets` struct in `internal/config/config.go`

**Key Features:**
- Uses upstream manifests directly via URL with templated version
- Includes templated configuration secret with vSphere credentials
- Supports custom image registry and tags via Kustomize
- Templated namespace for flexibility
- Service-specific secrets structure for vCenter configuration
- Disabled by default (VMware-specific)

**Configuration Example:**
```yaml
opencenter:
  services:
    vsphere-csi:
      enabled: true
      namespace: vmware-system-csi
      image_repository: registry.k8s.io/csi-vsphere
      image_tag: v3.3.0

secrets:
  vsphere_csi:
    vcenter_host: vcenter.example.com
    username: administrator@vsphere.local
    password: your-password
    datacenters: Datacenter1
    insecure_flag: "false"
    port: "443"
```

### Example 2: cert-manager Service (Helm-Based)

The cert-manager service demonstrates a Helm-based service with custom configuration:

**Files:**
- Manifests: `internal/gitops/templates/cluster-apps-base/services/cert-manager/`
  - `kustomization.yaml.tpl` - Generates Helm values secret
  - `helm-values/override-values.yaml.tpl` - Templated Helm values
  - Additional resources (issuers, secrets)
- FluxCD: `internal/gitops/templates/cluster-apps-base/services/fluxcd/cert-manager.yaml.tpl`
- Source: `internal/gitops/templates/cluster-apps-base/services/sources/opencenter-cert-manager.yaml.tpl`
- Config: Custom fields in `ServiceCfg` (Email, Region, LetsEncryptServer)

**Key Features:**
- Helm-based deployment with custom values
- Multiple dependent resources (issuers, CAs)
- Service-specific configuration fields
- Supports custom Git repository per service

### Example 3: openstack-csi Service (Helm with Overrides)

The openstack-csi service shows a simpler Helm pattern:

**Files:**
- Manifests: `internal/gitops/templates/cluster-apps-base/services/openstack-csi/`
  - `kustomization.yaml` - Static kustomization
  - `helm-values/override-values.yaml` - Static Helm values
- FluxCD: `internal/gitops/templates/cluster-apps-base/services/fluxcd/openstack-csi.yaml.tpl`
- Source: `internal/gitops/templates/cluster-apps-base/services/sources/opencenter-openstack-csi.yaml.tpl`

**Key Features:**
- Minimal configuration required
- Static Helm values (no templating)
- Simple enable/disable toggle

## Troubleshooting

### Service not appearing in GitOps repo

- Check that service is enabled in config: `opencenter.services.<service-name>.enabled: true`
- Verify FluxCD kustomization includes the service conditionally
- Ensure sources kustomization includes the service source

### Template rendering errors

- Validate Go template syntax in `.tpl` files
- Check that referenced config fields exist in `ServiceCfg` struct
- Use `mise run build && ./bin/openCenter cluster init` to test

### FluxCD reconciliation failures

- Check FluxCD logs: `kubectl logs -n flux-system -l app=kustomize-controller`
- Verify GitRepository source is accessible
- Ensure health checks reference correct resources
- Check for missing dependencies in `dependsOn`

## Related Documentation

- [Configuration Reference](reference/configuration.md)
- [CLI Commands](reference/cli-commands.md)
- [Architecture Overview](architecture.md)
