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

```yaml
opencenter:
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
    scopes: [openid, profile, email, roles]
  
  # ENHANCED: GitOps configuration
  gitops:
    secret_name: opencenter-base  # NEW
```

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
