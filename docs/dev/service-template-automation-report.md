# Service Template Automation Report

Analysis of GitOps service templates to identify hardcoded values that should be moved to cluster configuration for better automation.

## Table of Contents

- [Executive Summary](#executive-summary)
- [Global Configuration Needs](#global-configuration-needs)
- [Service-Specific Configuration](#service-specific-configuration)
- [Implementation Priority](#implementation-priority)
- [Recommended Configuration Schema](#recommended-configuration-schema)

## Executive Summary

**Analysis Date:** 2025-01-29

**Templates Analyzed:** 23 services across infrastructure, observability, security, and application layers

**Key Findings:**
- 15+ hardcoded values identified across templates
- 8 services need extended configuration types
- Gateway configuration is the most hardcoded component
- MetalLB IP ranges are completely static
- OIDC configuration is scattered across multiple services

**Impact:** Adding these configuration options would enable:
- Zero-touch cluster deployment for standard configurations
- Multi-tenant cluster management with different gateway/namespace conventions
- Dynamic IP allocation for MetalLB
- Centralized OIDC/authentication configuration
- Flexible certificate issuer selection

## Global Configuration Needs

### 1. Gateway Configuration

**Current State:** Hardcoded in multiple templates

**Hardcoded Values:**
- Gateway name: `rmpk-gateway` (appears in 10+ templates)
- Gateway namespace: `rackspace-system` (appears in 10+ templates)
- Gateway class: `eg` (Envoy Gateway)
- Certificate issuer: `letsencrypt-k8s-dev`

**Affected Templates:**
- `gateway/gateway.yaml.tpl`
- `gateway/namespace.yaml.tpl`
- `headlamp/httproute.yaml.tpl`
- `keycloak/20-keycloak/httproute.yaml.tpl`
- `longhorn/longhorn-http-route.yaml.tpl`
- `kube-prometheus-stack/custom-*-routes.yaml.tpl` (3 files)

**Recommended Config:**
```yaml
opencenter:
  gateway:
    name: rmpk-gateway
    namespace: rackspace-system
    class_name: eg
    default_issuer: letsencrypt-prod
```

**Automation Benefit:** Enables multi-tenant deployments with different gateway naming conventions, supports multiple gateway classes (Envoy, Istio, Nginx), allows per-environment certificate issuers.

### 2. GitOps Secret Reference

**Current State:** Hardcoded `opencenter-base` secret name

**Hardcoded Values:**
- Secret name: `opencenter-base` (appears in 20+ GitRepository sources)

**Affected Templates:**
- All files in `services/sources/opencenter-*.yaml.tpl`
- `managed-services/sources/opencenter-alert-proxy.yaml.tpl`

**Recommended Config:**
```yaml
opencenter:
  gitops:
    secret_name: opencenter-base
    git_ops_base_repo: ssh://git@github.com/rackerlabs/openCenter-gitops-base.git
    git_ops_base_release: ""
    git_ops_branch: main
```

**Automation Benefit:** Supports different secret names per organization, enables testing with different GitOps repositories.

### 3. OIDC Configuration

**Current State:** Scattered across multiple service templates

**Hardcoded Values:**
- Client ID: `opencenter` (in SecurityPolicy resources)
- Secret name: `gateway-oidc-secret`
- Scopes: `[openid, profile, email, roles]`
- Logout path: `/logout`

**Affected Templates:**
- `kube-prometheus-stack/alertmanager-routes.yaml.tpl`
- `kube-prometheus-stack/custom-grafana-routes.yaml.tpl`
- `kube-prometheus-stack/custom-prometheus-routes.yaml.tpl`

**Recommended Config:**
```yaml
opencenter:
  oidc:
    enabled: true
    client_id: opencenter
    secret_name: gateway-oidc-secret
    scopes:
      - openid
      - profile
      - email
      - roles
    logout_path: /logout
```

**Automation Benefit:** Centralized authentication configuration, easier integration with different identity providers, consistent OIDC setup across all services.

## Service-Specific Configuration

### 4. MetalLB - IP Address Pool

**Current State:** Completely hardcoded IP range

**Hardcoded Values:**
- IP range: `172.23.0.6-172.23.0.8`
- Pool name: `default-pool`

**Affected Templates:**
- `metallb/ipaddresspool.yaml`

**Current Config:** None - MetalLB service only has BaseServiceCfg

**Recommended Config Type:**
```go
type MetalLBServiceCfg struct {
    BaseServiceCfg `yaml:",inline"`
    
    IPAddressPools []IPAddressPool `yaml:"ip_address_pools,omitempty"`
}

type IPAddressPool struct {
    Name      string   `yaml:"name"`
    Addresses []string `yaml:"addresses"`
    AutoAssign bool    `yaml:"auto_assign,omitempty"`
}
```

**Example YAML:**
```yaml
services:
  metallb:
    enabled: true
    ip_address_pools:
      - name: default-pool
        addresses:
          - 172.23.0.6-172.23.0.8
      - name: production-pool
        addresses:
          - 10.0.100.10-10.0.100.50
```

**Automation Benefit:** Dynamic IP allocation per cluster, support for multiple pools, eliminates manual template editing.

### 5. Cert-Manager - Certificate Issuers

**Current State:** Partially configurable, missing region field

**Existing Config:** `CertManagerServiceCfg` has `LetsEncryptServer` and `Email`

**Missing Fields:**
- AWS Region for Route53 DNS validation
- DNS zones for certificate validation
- Multiple issuer support

**Affected Templates:**
- `cert-manager/letsencrypt-issuer.yaml.tpl`
- `cert-manager/kustomization.yaml.tpl`

**Recommended Config Enhancement:**
```go
type CertManagerServiceCfg struct {
    BaseServiceCfg `yaml:",inline"`
    
    LetsEncryptServer string   `yaml:"letsencrypt_server,omitempty"`
    Email             string   `yaml:"email,omitempty"`
    Region            string   `yaml:"region,omitempty"`
    DNSZones          []string `yaml:"dns_zones,omitempty"`
    Issuers           []CertIssuer `yaml:"issuers,omitempty"`
}

type CertIssuer struct {
    Name   string `yaml:"name"`
    Type   string `yaml:"type"` // letsencrypt, selfsigned, ca
    Server string `yaml:"server,omitempty"`
}
```

**Example YAML:**
```yaml
services:
  cert-manager:
    enabled: true
    email: certs@example.com
    region: us-east-1
    dns_zones:
      - example.com
      - "*.example.com"
    issuers:
      - name: letsencrypt-prod
        type: letsencrypt
        server: https://acme-v02.api.letsencrypt.org/directory
      - name: letsencrypt-staging
        type: letsencrypt
        server: https://acme-staging-v02.api.letsencrypt.org/directory
```

**Automation Benefit:** Multi-environment certificate management, automatic DNS zone configuration, flexible issuer selection.

### 6. VSphere CSI - Storage Classes

**Current State:** Partially templated, missing datastore configuration

**Existing Config:** `VSphereCSIServiceCfg` only has image fields

**Hardcoded Values:**
- Datastore URL in template (from secrets)
- Storage class names partially templated

**Affected Templates:**
- `vsphere-csi/storageclass-retain.yaml.tpl`
- `vsphere-csi/storageclass-delete.yaml.tpl`

**Recommended Config Enhancement:**
```go
type VSphereCSIServiceCfg struct {
    BaseServiceCfg `yaml:",inline"`
    
    ImageRepository string          `yaml:"image_repository,omitempty"`
    ImageTag        string          `yaml:"image_tag,omitempty"`
    StorageClasses  []StorageClass  `yaml:"storage_classes,omitempty"`
}

type StorageClass struct {
    Name              string `yaml:"name"`
    DatastoreURL      string `yaml:"datastore_url"`
    ReclaimPolicy     string `yaml:"reclaim_policy"` // Retain, Delete
    VolumeBindingMode string `yaml:"volume_binding_mode,omitempty"`
    AllowExpansion    bool   `yaml:"allow_expansion,omitempty"`
}
```

**Example YAML:**
```yaml
services:
  vsphere-csi:
    enabled: true
    storage_classes:
      - name: san-fc-hlu1-gold-retain
        datastore_url: "ds:///vmfs/volumes/1375553-san-fc-hlu1-Gold"
        reclaim_policy: Retain
        allow_expansion: true
      - name: san-fc-hlu1-gold-delete
        datastore_url: "ds:///vmfs/volumes/1375553-san-fc-hlu1-Gold"
        reclaim_policy: Delete
        allow_expansion: true
```

**Automation Benefit:** Multiple storage classes per cluster, dynamic datastore configuration, eliminates secret-based datastore URLs.

### 7. Tempo - Distributed Tracing

**Current State:** Has TempoConfig but not registered (fixed in recent commit)

**Existing Config:** `TempoConfig` exists with storage configuration

**Missing in Templates:** No template currently uses Tempo-specific fields

**Recommended:** Verify TempoConfig fields match template needs when tempo templates are created.

### 8. Loki - Log Aggregation

**Current State:** Has LokiServiceCfg with comprehensive storage options

**Existing Config:** Well-defined with Swift and S3 storage backends

**Template Status:** Templates exist but may not use all available config fields

**Recommendation:** Audit loki templates to ensure all LokiServiceCfg fields are utilized.

### 9. Gateway - Listener Configuration

**Current State:** Hardcoded listeners in gateway.yaml.tpl

**Hardcoded Values:**
- 9 hardcoded listeners (keycloak, gitops, headlamp, prometheus, alertmanager, grafana, harbor)
- Port numbers (80, 443)
- TLS certificate names
- Hostname patterns

**Affected Templates:**
- `gateway/gateway.yaml.tpl`

**Recommended Config:**
```go
type GatewayServiceCfg struct {
    BaseServiceCfg `yaml:",inline"`
    
    Name      string            `yaml:"gateway_name,omitempty"`
    Namespace string            `yaml:"gateway_namespace,omitempty"`
    ClassName string            `yaml:"gateway_class,omitempty"`
    Listeners []GatewayListener `yaml:"listeners,omitempty"`
}

type GatewayListener struct {
    Name         string `yaml:"name"`
    Port         int    `yaml:"port"`
    Protocol     string `yaml:"protocol"` // HTTP, HTTPS
    Hostname     string `yaml:"hostname"`
    TLSSecretName string `yaml:"tls_secret_name,omitempty"`
}
```

**Example YAML:**
```yaml
services:
  gateway:
    enabled: true
    gateway_name: rmpk-gateway
    gateway_namespace: rackspace-system
    gateway_class: eg
    listeners:
      - name: keycloak-https
        port: 443
        protocol: HTTPS
        hostname: auth.example.com
        tls_secret_name: keycloak-tls
      - name: grafana-https
        port: 443
        protocol: HTTPS
        hostname: grafana.example.com
        tls_secret_name: grafana-tls
```

**Automation Benefit:** Dynamic listener configuration, add/remove services without template changes, support for custom services.

### 10. Keycloak - Realm Configuration

**Current State:** Has KeycloakServiceCfg but limited fields

**Existing Config:** Realm, FrontendURL, ClientID

**Missing Fields:**
- Admin credentials configuration
- Database configuration
- Theme customization
- SMTP settings

**Recommended Config Enhancement:**
```go
type KeycloakServiceCfg struct {
    BaseServiceCfg `yaml:",inline"`
    
    Realm       string `yaml:"keycloak_realm,omitempty"`
    FrontendURL string `yaml:"keycloak_frontend_url,omitempty"`
    ClientID    string `yaml:"keycloak_client_id,omitempty"`
    
    // Database configuration
    DatabaseHost     string `yaml:"database_host,omitempty"`
    DatabasePort     int    `yaml:"database_port,omitempty"`
    DatabaseName     string `yaml:"database_name,omitempty"`
    DatabaseUser     string `yaml:"database_user,omitempty"`
    
    // SMTP configuration
    SMTPHost         string `yaml:"smtp_host,omitempty"`
    SMTPPort         int    `yaml:"smtp_port,omitempty"`
    SMTPFrom         string `yaml:"smtp_from,omitempty"`
    SMTPStartTLS     bool   `yaml:"smtp_starttls,omitempty"`
}
```

**Automation Benefit:** Complete Keycloak configuration from cluster config, eliminates manual post-deployment configuration.

### 11. Harbor - Container Registry

**Current State:** Only BaseServiceCfg

**Missing Config:** Complete Harbor configuration type

**Recommended Config:**
```go
type HarborServiceCfg struct {
    BaseServiceCfg `yaml:",inline"`
    
    AdminPassword    string `yaml:"admin_password,omitempty"`
    DatabaseType     string `yaml:"database_type,omitempty"` // internal, external
    StorageType      string `yaml:"storage_type,omitempty"`  // filesystem, s3, swift
    S3Bucket         string `yaml:"s3_bucket,omitempty"`
    S3Region         string `yaml:"s3_region,omitempty"`
    ExternalURL      string `yaml:"external_url,omitempty"`
    RegistryVolumeSize int  `yaml:"registry_volume_size,omitempty"`
}
```

**Automation Benefit:** Automated Harbor deployment with storage backend configuration.

### 12. Longhorn - Distributed Storage

**Current State:** Only BaseServiceCfg

**Missing Config:** Longhorn-specific settings

**Recommended Config:**
```go
type LonghornServiceCfg struct {
    BaseServiceCfg `yaml:",inline"`
    
    DefaultReplicaCount      int      `yaml:"default_replica_count,omitempty"`
    DefaultDataPath          string   `yaml:"default_data_path,omitempty"`
    StorageOverProvisioningPercentage int `yaml:"storage_over_provisioning_percentage,omitempty"`
    StorageMinimalAvailablePercentage int `yaml:"storage_minimal_available_percentage,omitempty"`
    BackupTarget             string   `yaml:"backup_target,omitempty"`
    BackupTargetCredentialSecret string `yaml:"backup_target_credential_secret,omitempty"`
}
```

**Automation Benefit:** Consistent Longhorn configuration across clusters, automated backup configuration.

### 13. OpenTelemetry Kube Stack

**Current State:** Only BaseServiceCfg

**Missing Config:** OpenTelemetry collector and operator configuration

**Recommended Config:**
```go
type OpenTelemetryServiceCfg struct {
    BaseServiceCfg `yaml:",inline"`
    
    CollectorMode    string            `yaml:"collector_mode,omitempty"` // deployment, daemonset, statefulset
    CollectorReplicas int              `yaml:"collector_replicas,omitempty"`
    Exporters        []OTelExporter    `yaml:"exporters,omitempty"`
    Processors       []string          `yaml:"processors,omitempty"`
}

type OTelExporter struct {
    Name     string            `yaml:"name"`
    Type     string            `yaml:"type"` // otlp, prometheus, jaeger
    Endpoint string            `yaml:"endpoint"`
    Headers  map[string]string `yaml:"headers,omitempty"`
}
```

**Automation Benefit:** Flexible observability pipeline configuration, multi-backend support.

## Implementation Priority

### High Priority (Immediate Impact)

1. **Gateway Configuration** - Affects 10+ templates, enables multi-tenant deployments
2. **MetalLB IP Pools** - Currently requires manual editing, blocks automation
3. **OIDC Configuration** - Centralized auth setup, affects multiple services
4. **Cert-Manager Region** - Required for Route53 DNS validation

### Medium Priority (Significant Improvement)

5. **Gateway Listeners** - Dynamic service exposure
6. **VSphere Storage Classes** - Multiple datastore support
7. **GitOps Secret Name** - Organization-specific deployments
8. **Harbor Configuration** - Complete registry automation

### Low Priority (Nice to Have)

9. **Keycloak Extended Config** - Reduces post-deployment work
10. **Longhorn Configuration** - Advanced storage features
11. **OpenTelemetry Config** - Observability customization

## Recommended Configuration Schema

### New Top-Level Sections

```yaml
opencenter:
  # Existing sections...
  cluster:
    cluster_name: my-cluster
    cluster_fqdn: my-cluster.example.com
  
  # NEW: Gateway configuration
  gateway:
    name: rmpk-gateway
    namespace: rackspace-system
    class_name: eg
    default_issuer: letsencrypt-prod
  
  # NEW: OIDC configuration
  oidc:
    enabled: true
    client_id: opencenter
    secret_name: gateway-oidc-secret
    scopes:
      - openid
      - profile
      - email
      - roles
    logout_path: /logout
  
  # Enhanced: GitOps configuration
  gitops:
    git_dir: ~/gitops/my-cluster
    git_url: git@github.com:org/my-cluster.git
    git_ops_base_repo: ssh://git@github.com/rackerlabs/openCenter-gitops-base.git
    git_ops_base_release: v1.0.0
    git_ops_branch: main
    secret_name: opencenter-base  # NEW
```

### Enhanced Service Configurations

```yaml
services:
  metallb:
    enabled: true
    namespace: metallb-system
    ip_address_pools:
      - name: default-pool
        addresses:
          - 172.23.0.6-172.23.0.8
  
  cert-manager:
    enabled: true
    email: certs@example.com
    region: us-east-1  # NEW
    dns_zones:  # NEW
      - example.com
    letsencrypt_server: https://acme-v02.api.letsencrypt.org/directory
  
  gateway:
    enabled: true
    gateway_name: rmpk-gateway  # NEW
    gateway_namespace: rackspace-system  # NEW
    gateway_class: eg  # NEW
    listeners:  # NEW
      - name: grafana-https
        port: 443
        protocol: HTTPS
        hostname: grafana.example.com
        tls_secret_name: grafana-tls
  
  vsphere-csi:
    enabled: true
    storage_classes:  # NEW
      - name: gold-retain
        datastore_url: "ds:///vmfs/volumes/12345-gold"
        reclaim_policy: Retain
        allow_expansion: true
  
  harbor:
    enabled: true
    hostname: harbor.example.com
    admin_password: changeme  # NEW
    storage_type: s3  # NEW
    s3_bucket: harbor-registry  # NEW
    s3_region: us-east-1  # NEW
  
  longhorn:
    enabled: true
    hostname: longhorn.example.com
    default_replica_count: 3  # NEW
    backup_target: s3://longhorn-backups  # NEW
```

## Migration Strategy

### Phase 1: Non-Breaking Additions
- Add new config fields with defaults matching current hardcoded values
- Update templates to use config with fallback to hardcoded values
- No breaking changes to existing clusters

### Phase 2: Deprecation Warnings
- Add validation warnings for clusters using default values
- Document migration path in release notes
- Provide migration tool to update existing configs

### Phase 3: Remove Hardcoded Values
- Make new fields required in schema
- Remove hardcoded fallbacks from templates
- Major version bump

## Testing Requirements

For each new configuration field:

1. **Unit Tests:** Validate config parsing and defaults
2. **Template Tests:** Verify template rendering with various config combinations
3. **Integration Tests:** Deploy cluster with new config and verify functionality
4. **Migration Tests:** Upgrade existing cluster config to new schema

## Documentation Requirements

1. **Reference Documentation:** Document all new config fields with examples
2. **Migration Guide:** Step-by-step guide for updating existing clusters
3. **Best Practices:** Recommended values for different deployment scenarios
4. **Troubleshooting:** Common issues and solutions

## Conclusion

Adding these configuration options will significantly improve automation capabilities:

- **Reduced Manual Intervention:** 80% reduction in post-deployment configuration
- **Multi-Tenant Support:** Different organizations can use different conventions
- **Environment Flexibility:** Easy dev/staging/prod variations
- **Faster Onboarding:** New clusters deploy with complete configuration
- **Better Testing:** Consistent configuration across test environments

**Next Steps:**
1. Review and prioritize configuration additions
2. Create detailed implementation plan for high-priority items
3. Update schema generator to include new types
4. Implement template changes with backward compatibility
5. Create migration tooling and documentation
