# Adding New Services to openCenter

This guide explains how to add new services to the openCenter GitOps scaffolding system. Services are Kubernetes applications that are deployed and managed through FluxCD.

## Overview

Services in openCenter follow a template-driven approach where:
1. Service manifests are stored in `internal/gitops/templates/cluster-apps-base/services/`
2. FluxCD Kustomization resources are defined in `internal/gitops/templates/cluster-apps-base/services/fluxcd/`
3. GitRepository sources are defined in `internal/gitops/templates/cluster-apps-base/services/sources/`
4. Service configuration is controlled via the cluster config YAML under `opencenter.services`
5. Schema validation is defined in `internal/config/schema.go`

## Service Architecture

Each service consists of three main components:

### 1. Service Manifests
Located in `internal/gitops/templates/cluster-apps-base/services/<service-name>/`

These contain the actual Kubernetes resources (Kustomization, HelmRelease, ConfigMaps, Secrets, etc.)

### 2. FluxCD Kustomization
Located in `internal/gitops/templates/cluster-apps-base/services/fluxcd/<service-name>.yaml.tpl`

Defines how FluxCD should reconcile the service from the GitOps repository.

### 3. GitRepository Source
Located in `internal/gitops/templates/cluster-apps-base/services/sources/opencenter-<service-name>.yaml.tpl`

Defines the Git source repository for the service manifests.

## Step-by-Step Guide

### Step 1: Create Service Manifests Directory

Create a new directory for your service:

```bash
mkdir -p internal/gitops/templates/cluster-apps-base/services/<service-name>
```

### Step 2: Add Service Manifests

Create the necessary Kubernetes manifests. At minimum, you need a `kustomization.yaml`:

**Example: `internal/gitops/templates/cluster-apps-base/services/<service-name>/kustomization.yaml`**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: <service-namespace>
resources:
  - <resource-url-or-path>
```

For services requiring configuration, you can use `.tpl` extension to enable Go templating:

**Example: `internal/gitops/templates/cluster-apps-base/services/<service-name>/kustomization.yaml.tpl`**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: {{ .OpenCenter.Services.<service-name>.Namespace | default "<default-namespace>" }}
resources:
  - <resource-url-or-path>
```

### Step 3: Create FluxCD Kustomization

Create a FluxCD Kustomization resource that tells Flux how to deploy your service:

**File: `internal/gitops/templates/cluster-apps-base/services/fluxcd/<service-name>.yaml.tpl`**

```yaml
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: <service-name>-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 5m
  sourceRef:
    kind: GitRepository
    name: opencenter-<service-name>
    namespace: flux-system
  path: applications/base/services/<service-name>
  targetNamespace: <service-namespace>
  prune: true
  healthChecks:
    - apiVersion: <api-version>
      kind: <resource-kind>
      name: <resource-name>
      namespace: <service-namespace>
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: <service-name>
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: <service-name>-override
  namespace: flux-system
spec:
  interval: 15m
  retryInterval: 1m
  timeout: 5m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  path: ./applications/overlays/{{ .ClusterName }}/services/<service-name>
  targetNamespace: <service-namespace>
  prune: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: <service-name>
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
```

### Step 4: Create GitRepository Source

Create a GitRepository source that points to the openCenter base repository:

**File: `internal/gitops/templates/cluster-apps-base/services/sources/opencenter-<service-name>.yaml.tpl`**

```yaml
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-<service-name>
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

### Step 5: Update Schema Definition

Add your service to the schema in `internal/config/schema.go`:

```go
services := map[string]any{
    "type": "object",
    "properties": map[string]any{
        // ... existing services ...
        "<service-name>": baseServiceSchema,  // For simple services
        // OR
        "<service-name>": serviceSchema,      // For services with additional config
    },
    "additionalProperties": serviceSchema,
}
```

If your service requires custom configuration fields, add them to the `ServiceCfg` struct in `internal/config/config.go`:

```go
type ServiceCfg struct {
    Enabled bool   `yaml:"enabled" json:"enabled"`
    
    // Add service-specific fields
    CustomField string `yaml:"custom_field" json:"custom_field" jsonschema:"description=Custom field description"`
}
```

### Step 6: Update Sources Kustomization

Add your service source to the sources kustomization template:

**File: `internal/gitops/templates/cluster-apps-base/services/sources/kustomization.yaml.tpl`**

```yaml
resources:
  # ... existing sources ...
  {{- if index .OpenCenter.Services "<service-name>" }}
  {{- if (index .OpenCenter.Services "<service-name>").Enabled }}
  - opencenter-<service-name>.yaml
  {{- end }}
  {{- end }}
```

### Step 7: Update FluxCD Kustomization

Add your service to the FluxCD kustomization template:

**File: `internal/gitops/templates/cluster-apps-base/services/fluxcd/kustomization.yaml.tpl`**

```yaml
resources:
  # ... existing services ...
  {{- if index .OpenCenter.Services "<service-name>" }}
  {{- if (index .OpenCenter.Services "<service-name>").Enabled }}
  - <service-name>.yaml
  {{- end }}
  {{- end }}
```

### Step 8: Test Your Service

1. Create a test cluster configuration:

```yaml
opencenter:
  services:
    <service-name>:
      enabled: true
```

2. Run the cluster init command:

```bash
mise run build
./bin/openCenter cluster init --config test-config.yaml
```

3. Verify the generated GitOps repository contains your service files.

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
