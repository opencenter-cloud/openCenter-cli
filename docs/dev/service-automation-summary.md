# Service Template Automation - Quick Summary

**Full Report:** [service-template-automation-report.md](./service-template-automation-report.md)

## Critical Findings

### Top 5 Hardcoded Values Blocking Automation

1. **Gateway Name/Namespace** (`rmpk-gateway` / `rackspace-system`)
   - Appears in: 10+ templates
   - Impact: Blocks multi-tenant deployments
   - Priority: HIGH

2. **MetalLB IP Ranges** (`172.23.0.6-172.23.0.8`)
   - Appears in: `metallb/ipaddresspool.yaml`
   - Impact: Requires manual editing per cluster
   - Priority: HIGH

3. **OIDC Configuration** (client_id: `opencenter`)
   - Appears in: 3+ SecurityPolicy resources
   - Impact: Scattered auth configuration
   - Priority: HIGH

4. **GitOps Secret Name** (`opencenter-base`)
   - Appears in: 20+ GitRepository sources
   - Impact: Limits organization flexibility
   - Priority: MEDIUM

5. **Certificate Issuer** (`letsencrypt-k8s-dev`)
   - Appears in: Gateway annotations
   - Impact: Can't switch between staging/prod
   - Priority: HIGH

## Services Needing Configuration Types

| Service | Current Config | Missing Fields | Priority |
|---------|---------------|----------------|----------|
| MetalLB | BaseServiceCfg only | IP pools, L2 config | HIGH |
| Gateway | BaseServiceCfg only | Name, namespace, listeners | HIGH |
| Harbor | BaseServiceCfg only | Storage, database, admin | MEDIUM |
| Longhorn | BaseServiceCfg only | Replicas, backup target | MEDIUM |
| OpenTelemetry | BaseServiceCfg only | Collectors, exporters | LOW |
| Cert-Manager | Partial | Region, DNS zones | HIGH |
| VSphere CSI | Partial | Storage classes | MEDIUM |
| Keycloak | Partial | Database, SMTP | LOW |

## Recommended New Config Sections

### Proposed Defaults (Backward Compatible)

```yaml
opencenter:
  # NEW: Gateway configuration
  gateway:
    name: rmpk-gateway                    # Default: rmpk-gateway
    namespace: rackspace-system           # Default: rackspace-system
    class_name: eg                        # Default: eg (Envoy Gateway)
    default_issuer: letsencrypt-prod      # Default: letsencrypt-{cluster_name}
  
  # NEW: OIDC configuration
  oidc:
    enabled: true                         # Default: true
    client_id: opencenter                 # Default: opencenter
    secret_name: gateway-oidc-secret      # Default: gateway-oidc-secret
    scopes:                               # Default: [openid, profile, email, roles]
      - openid
      - profile
      - email
      - roles
    logout_path: /logout                  # Default: /logout
  
  # ENHANCED: GitOps configuration
  gitops:
    secret_name: opencenter-base          # Default: opencenter-base (NEW)
    git_ops_base_repo: ssh://git@github.com/rackerlabs/openCenter-gitops-base.git  # Default
    git_ops_base_release: ""              # Default: "" (use branch)
    git_ops_branch: main                  # Default: main
```

### Service-Specific Defaults

```yaml
services:
  # MetalLB - IP Address Pools
  metallb:
    enabled: false                        # Default: false
    namespace: metallb-system             # Default: metallb-system
    ip_address_pools:                     # Default: [] (must be configured)
      - name: default-pool                # Example configuration
        addresses:
          - 172.23.0.6-172.23.0.8
        auto_assign: true                 # Default: true
  
  # Cert-Manager - Enhanced Configuration
  cert-manager:
    enabled: true                         # Default: true
    namespace: cert-manager               # Default: cert-manager
    email: mpk-support@rackspace.com      # Default: mpk-support@rackspace.com
    region: us-east-1                     # Default: "" (must be configured for Route53)
    letsencrypt_server: https://acme-v02.api.letsencrypt.org/directory  # Default: production
    dns_zones:                            # Default: [cluster_fqdn]
      - "*.example.com"
  
  # Gateway - Listener Configuration
  gateway:
    enabled: true                         # Default: true
    namespace: rackspace-system           # Default: rackspace-system
    gateway_name: rmpk-gateway            # Default: rmpk-gateway
    gateway_class: eg                     # Default: eg
    listeners:                            # Default: auto-generated from enabled services
      - name: keycloak-https
        port: 443
        protocol: HTTPS
        hostname: auth.{cluster_fqdn}     # Default: auth.{cluster_fqdn}
        tls_secret_name: keycloak-tls     # Default: {service}-tls
      - name: grafana-https
        port: 443
        protocol: HTTPS
        hostname: grafana.{cluster_fqdn}  # Default: grafana.{cluster_fqdn}
        tls_secret_name: grafana-tls
  
  # VSphere CSI - Storage Classes
  vsphere-csi:
    enabled: false                        # Default: false (provider-specific)
    namespace: vmware-system-csi          # Default: vmware-system-csi
    storage_classes:                      # Default: [] (must be configured)
      - name: default-retain              # Example configuration
        datastore_url: "ds:///vmfs/volumes/datastore1"
        reclaim_policy: Retain            # Default: Retain
        allow_expansion: true             # Default: true
        volume_binding_mode: Immediate    # Default: Immediate
  
  # Harbor - Container Registry
  harbor:
    enabled: false                        # Default: false
    namespace: harbor                     # Default: harbor
    hostname: harbor.{cluster_fqdn}       # Default: harbor.{cluster_fqdn}
    storage_type: filesystem              # Default: filesystem
    registry_volume_size: 100             # Default: 100 (GB)
    database_type: internal               # Default: internal
  
  # Longhorn - Distributed Storage
  longhorn:
    enabled: false                        # Default: false
    namespace: longhorn-system            # Default: longhorn-system
    hostname: longhorn.{cluster_fqdn}     # Default: longhorn.{cluster_fqdn}
    default_replica_count: 3              # Default: 3
    default_data_path: /var/lib/longhorn  # Default: /var/lib/longhorn
    storage_over_provisioning_percentage: 200  # Default: 200
    storage_minimal_available_percentage: 25   # Default: 25
  
  # Loki - Log Aggregation (Already Well-Defined)
  loki:
    enabled: true                         # Default: true
    namespace: observability              # Default: observability
    hostname: loki.{cluster_fqdn}         # Default: loki.{cluster_fqdn}
    loki_storage_type: swift              # Default: swift
    loki_volume_size: 50                  # Default: 50 (GB)
    swift_auth_version: 3                 # Default: 3
  
  # Tempo - Distributed Tracing (Already Well-Defined)
  tempo:
    enabled: true                         # Default: true
    namespace: observability              # Default: observability
    hostname: tempo.{cluster_fqdn}        # Default: tempo.{cluster_fqdn}
    storage_type: swift                   # Default: swift
    volume_size: 50                       # Default: 50 (GB)
  
  # Kube-Prometheus-Stack (Already Well-Defined)
  kube-prometheus-stack:
    enabled: true                         # Default: true
    namespace: observability              # Default: observability
    hostname: prometheus.{cluster_fqdn}   # Default: prometheus.{cluster_fqdn}
    grafana_volume_size: 10               # Default: 10 (GB)
    prometheus_volume_size: 50            # Default: 50 (GB)
    alertmanager_volume_size: 10          # Default: 10 (GB)
  
  # Keycloak - Identity Management (Already Partially Defined)
  keycloak:
    enabled: true                         # Default: true
    namespace: keycloak                   # Default: keycloak
    hostname: auth.{cluster_fqdn}         # Default: auth.{cluster_fqdn}
    keycloak_realm: opencenter            # Default: opencenter
    keycloak_client_id: opencenter        # Default: opencenter
  
  # Headlamp - Kubernetes Dashboard (Already Partially Defined)
  headlamp:
    enabled: true                         # Default: true
    namespace: headlamp                   # Default: headlamp
    hostname: headlamp.{cluster_fqdn}     # Default: headlamp.{cluster_fqdn}
  
  # Velero - Backup and Restore (Already Partially Defined)
  velero:
    enabled: false                        # Default: false
    namespace: velero                     # Default: velero
    velero_region: us-east-1              # Default: "" (must be configured)
  
  # OpenTelemetry Kube Stack
  opentelemetry-kube-stack:
    enabled: false                        # Default: false
    namespace: observability              # Default: observability
    collector_mode: deployment            # Default: deployment
    collector_replicas: 1                 # Default: 1
```

### Default Value Resolution Strategy

**Priority Order:**
1. User-specified value in cluster config
2. Environment-specific default (dev/staging/prod)
3. Provider-specific default (OpenStack/AWS/vSphere)
4. Global default value

**Variable Substitution:**
- `{cluster_name}` → `opencenter.cluster.cluster_name`
- `{cluster_fqdn}` → `opencenter.cluster.cluster_fqdn`
- `{organization}` → `opencenter.meta.organization`
- `{env}` → `opencenter.meta.env`
- `{region}` → `opencenter.meta.region`

## Quick Wins (Immediate Implementation)

1. **Add Gateway Config Section**
   - Files to modify: 10+ templates
   - Effort: 2-3 hours
   - Impact: Enables multi-tenant deployments

2. **Add MetalLB Config Type**
   - Files to modify: `types_services.go`, `metallb/ipaddresspool.yaml`
   - Effort: 1 hour
   - Impact: Eliminates manual IP configuration

3. **Add OIDC Config Section**
   - Files to modify: 3 SecurityPolicy templates
   - Effort: 1 hour
   - Impact: Centralized authentication

4. **Add Cert-Manager Region Field**
   - Files to modify: `types_services.go`, `letsencrypt-issuer.yaml.tpl`
   - Effort: 30 minutes
   - Impact: Fixes Route53 DNS validation

## Implementation Approach

### Phase 1: Add with Defaults (Non-Breaking)
- Add new config fields
- Templates use config with fallback to current hardcoded values
- Existing clusters continue working

### Phase 2: Deprecation Warnings
- Warn when using default values
- Provide migration documentation
- Add migration tool

### Phase 3: Remove Hardcoded Values
- Make fields required
- Remove fallbacks
- Major version bump

## Expected Benefits

- **80% reduction** in post-deployment manual configuration
- **Zero-touch deployment** for standard configurations
- **Multi-tenant support** with organization-specific conventions
- **Environment flexibility** (dev/staging/prod variations)
- **Faster onboarding** for new clusters

## Next Actions

1. Review full report: `docs/dev/service-template-automation-report.md`
2. Prioritize configuration additions
3. Create implementation tickets
4. Update schema generator
5. Implement with backward compatibility
