---
doc_type: how-to
title: "Adding New Platform Services"
audience: "developers"
---

# Adding New Platform Services

**Purpose:** For developers, shows how to add new platform services (monitoring, security, storage, etc.) to openCenter-cli.

## Prerequisites

Before adding a service, you need:
- Development environment set up (see [Development Setup](development-setup.md))
- Understanding of Helm charts and Kubernetes manifests
- Knowledge of FluxCD HelmRelease and Kustomization resources

## Service Architecture

Platform services in openCenter-cli consist of:

1. **Service defaults** - Default configuration and enabled state
2. **Helm values** - Security-hardened Helm chart values
3. **FluxCD manifests** - HelmRelease or Kustomization resources
4. **Dependencies** - Service ordering and dependencies
5. **Documentation** - Service description and configuration options

## Step 1: Add Service Defaults

Add service configuration to `internal/config/defaults.go`:

```go
// In getProviderDefaults function, add to services section
"services": map[string]interface{}{
    // ... existing services
    
    "my-service": map[string]interface{}{
        "enabled": true,  // or false for opt-in services
        "namespace": "my-service",
        "version": "1.0.0",
        "helm_repo": "https://charts.example.com",
        "helm_chart": "my-service",
        "values": map[string]interface{}{
            "replicas": 2,
            "resources": map[string]interface{}{
                "requests": map[string]interface{}{
                    "cpu":    "100m",
                    "memory": "128Mi",
                },
                "limits": map[string]interface{}{
                    "cpu":    "500m",
                    "memory": "512Mi",
                },
            },
        },
    },
},
```


## Step 2: Create Service Templates

Create service directory in `internal/gitops/gitops-base-dir/applications/base/services/my-service/`:

### Namespace

Create `namespace.yaml`:
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: my-service
  labels:
    app.kubernetes.io/name: my-service
    app.kubernetes.io/managed-by: flux
```

### HelmRepository Source

Create `source.yaml.tmpl`:
```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: HelmRepository
metadata:
  name: my-service
  namespace: flux-system
spec:
  url: {{ .OpenCenter.Services.MyService.HelmRepo }}
  interval: 15m
```

### HelmRelease

Create `helmrelease.yaml.tmpl`:
```yaml
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: my-service
  namespace: my-service
spec:
  interval: 15m
  chart:
    spec:
      chart: {{ .OpenCenter.Services.MyService.HelmChart }}
      version: {{ .OpenCenter.Services.MyService.Version }}
      sourceRef:
        kind: HelmRepository
        name: my-service
        namespace: flux-system
  install:
    createNamespace: true
    remediation:
      retries: 3
  upgrade:
    remediation:
      retries: 3
  valuesFrom:
    - kind: ConfigMap
      name: my-service-values
      valuesKey: values.yaml
```

### Hardened Helm Values

Create `helm-values/hardened-values-v1.0.0.yaml`:
```yaml
# Security-hardened values for my-service v1.0.0

# Replica configuration
replicaCount: 2

# Security context
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000
  seccompProfile:
    type: RuntimeDefault
  capabilities:
    drop:
      - ALL

# Pod security context
podSecurityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000

# Resource limits
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi

# Network policy
networkPolicy:
  enabled: true
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: ingress-nginx
      ports:
        - protocol: TCP
          port: 8080
  egress:
    - to:
        - namespaceSelector: {}
      ports:
        - protocol: TCP
          port: 443  # HTTPS
        - protocol: TCP
          port: 53   # DNS
        - protocol: UDP
          port: 53   # DNS

# Service monitor for Prometheus
serviceMonitor:
  enabled: true
  interval: 30s
  scrapeTimeout: 10s

# Pod disruption budget
podDisruptionBudget:
  enabled: true
  minAvailable: 1

# Affinity rules
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
              - key: app.kubernetes.io/name
                operator: In
                values:
                  - my-service
          topologyKey: kubernetes.io/hostname
```

### Kustomization

Create `kustomization.yaml`:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: my-service

resources:
  - namespace.yaml
  - source.yaml
  - helmrelease.yaml

configMapGenerator:
  - name: my-service-values
    files:
      - values.yaml=helm-values/hardened-values-v1.0.0.yaml
```


## Step 3: Add Service Dependencies

If your service depends on other services, document dependencies in `internal/gitops/dependencies.go`:

```go
// Service dependency graph
var serviceDependencies = map[string][]string{
    "my-service": {
        "cert-manager",  // Requires cert-manager for TLS
        "kyverno",       // Requires Kyverno policies
    },
    // ... other services
}

// GetServiceDeploymentOrder returns services in dependency order
func GetServiceDeploymentOrder(enabledServices []string) []string {
    // Topological sort implementation
    return sortedServices
}
```

## Step 4: Add Service Validation

Create validation logic in `internal/config/service_validator.go`:

```go
// ValidateMyServiceConfig validates my-service configuration
func ValidateMyServiceConfig(cfg *Config) []error {
    var errors []error
    
    service := cfg.OpenCenter.Services.MyService
    
    if !service.Enabled {
        return nil  // Skip validation if disabled
    }
    
    // Validate version format
    if !isValidSemver(service.Version) {
        errors = append(errors, fmt.Errorf("invalid version: %s", service.Version))
    }
    
    // Validate Helm repository URL
    if !isValidURL(service.HelmRepo) {
        errors = append(errors, fmt.Errorf("invalid helm_repo URL: %s", service.HelmRepo))
    }
    
    // Validate dependencies are enabled
    if service.Enabled {
        if !cfg.OpenCenter.Services.CertManager.Enabled {
            errors = append(errors, fmt.Errorf("my-service requires cert-manager to be enabled"))
        }
    }
    
    // Validate resource limits
    if service.Values.Resources.Limits.Memory < service.Values.Resources.Requests.Memory {
        errors = append(errors, fmt.Errorf("memory limit must be >= memory request"))
    }
    
    return errors
}
```

## Step 5: Update Schema

Add service schema to `internal/config/schema.go`:

```go
// Add to services schema
"my-service": {
    Type: "object",
    Properties: map[string]*jsonschema.Schema{
        "enabled": {
            Type:        "boolean",
            Description: "Enable my-service deployment",
            Default:     true,
        },
        "namespace": {
            Type:        "string",
            Description: "Kubernetes namespace for my-service",
            Default:     "my-service",
        },
        "version": {
            Type:        "string",
            Description: "Helm chart version",
            Pattern:     "^\\d+\\.\\d+\\.\\d+$",
        },
        "helm_repo": {
            Type:        "string",
            Description: "Helm repository URL",
            Format:      "uri",
        },
        "helm_chart": {
            Type:        "string",
            Description: "Helm chart name",
        },
        "values": {
            Type:        "object",
            Description: "Helm chart values",
        },
    },
}
```

## Step 6: Write Tests

### Unit Tests

Create `internal/config/service_validator_test.go`:

```go
func TestValidateMyServiceConfig(t *testing.T) {
    tests := []struct {
        name    string
        config  *Config
        wantErr bool
    }{
        {
            name: "valid configuration",
            config: &Config{
                OpenCenter: OpenCenterConfig{
                    Services: ServicesConfig{
                        MyService: MyServiceConfig{
                            Enabled:   true,
                            Version:   "1.0.0",
                            HelmRepo:  "https://charts.example.com",
                            HelmChart: "my-service",
                        },
                        CertManager: CertManagerConfig{
                            Enabled: true,
                        },
                    },
                },
            },
            wantErr: false,
        },
        {
            name: "missing dependency",
            config: &Config{
                OpenCenter: OpenCenterConfig{
                    Services: ServicesConfig{
                        MyService: MyServiceConfig{
                            Enabled: true,
                        },
                        CertManager: CertManagerConfig{
                            Enabled: false,  // Dependency not enabled
                        },
                    },
                },
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            errs := ValidateMyServiceConfig(tt.config)
            if tt.wantErr {
                assert.NotEmpty(t, errs)
            } else {
                assert.Empty(t, errs)
            }
        })
    }
}
```

### BDD Tests

Create `tests/features/my_service.feature`:

```gherkin
Feature: My Service Configuration
  As a platform engineer
  I want to configure my-service
  So that I can deploy it to my cluster

  Scenario: Enable my-service
    Given I have a cluster configuration
    When I run "opencenter cluster set test opencenter.services.my_service.enabled=true"
    Then the command should succeed
    And the configuration should have my_service enabled

  Scenario: Validate my-service dependencies
    Given I have a cluster configuration
    And I have disabled cert-manager
    When I run "opencenter cluster set test opencenter.services.my_service.enabled=true"
    And I run "opencenter cluster validate test"
    Then the command should fail
    And the error should contain "my-service requires cert-manager"

  Scenario: Deploy my-service
    Given I have a cluster configuration with my-service enabled
    When I run "opencenter cluster generate test"
    Then the command should succeed
    And the file "applications/base/services/my-service/helmrelease.yaml" should exist
    And the file should contain "chart: my-service"
```

## Step 7: Test Service Rendering

Test that templates render correctly:

```bash
# Build CLI
mise run build

# Initialize test cluster
./bin/opencenter cluster init test --org test-org

# Enable service
./bin/opencenter cluster set test \
  opencenter.services.my_service.enabled=true

# Render templates
./bin/opencenter cluster generate test

# Verify generated files
ls -la ~/.config/opencenter/clusters/test-org/test/gitops/applications/base/services/my-service/

# Check rendered HelmRelease
cat ~/.config/opencenter/clusters/test-org/test/gitops/applications/base/services/my-service/helmrelease.yaml
```


## Step 8: Update Documentation

Add service to `docs/reference/platform-services.md`:

```markdown
### my-service

**Category:** Monitoring / Security / Storage / Networking

**Description:** Brief description of what the service does.

**Default State:** Enabled / Disabled

**Dependencies:**
- cert-manager (TLS certificates)
- kyverno (policy enforcement)

**Configuration:**
```yaml
opencenter:
  services:
    my_service:
      enabled: true
      namespace: my-service
      version: "1.0.0"
      helm_repo: https://charts.example.com
      helm_chart: my-service
      values:
        replicas: 2
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
```

**Helm Chart:** [my-service](https://charts.example.com/my-service)

**Documentation:** [Official Docs](https://docs.example.com/my-service)

**Security Hardening:**
- Runs as non-root user (UID 1000)
- Drops all capabilities
- Network policies restrict ingress/egress
- Resource limits enforced
- Pod disruption budget configured
```

Add to `docs/how-to/customize-services.md`:

```markdown
## Enable my-service

```bash
opencenter cluster set my-cluster \
  opencenter.services.my_service.enabled=true
```

## Configure my-service

```bash
# Set version
opencenter cluster set my-cluster \
  opencenter.services.my_service.version="1.2.0"

# Set replicas
opencenter cluster set my-cluster \
  opencenter.services.my_service.values.replicas=3

# Set resource limits
opencenter cluster set my-cluster \
  opencenter.services.my_service.values.resources.limits.memory="1Gi"
```
```

## Step 9: Submit Pull Request

1. Run all tests:
   ```bash
   mise run test
   mise run godog
   mise run schema-verify
   ```

2. Update CHANGELOG.md:
   ```markdown
   ## [Unreleased]
   
   ### Added
   - my-service platform service with security hardening
   - my-service validation and dependency checking
   - my-service documentation and configuration examples
   ```

3. Create pull request with:
   - Service description and use case
   - Security hardening details
   - Test results
   - Documentation updates

## Service Checklist

Before submitting, verify:

- [ ] Service defaults added to `internal/config/defaults.go`
- [ ] Templates created in `internal/gitops/gitops-base-dir/applications/base/services/my-service/`
- [ ] Namespace manifest created
- [ ] HelmRepository source created
- [ ] HelmRelease manifest created
- [ ] Hardened Helm values created
- [ ] Kustomization manifest created
- [ ] Dependencies documented in `internal/gitops/dependencies.go`
- [ ] Service validation added to `internal/config/service_validator.go`
- [ ] Schema updated in `internal/config/schema.go`
- [ ] Unit tests written for validation
- [ ] BDD tests written for service workflows
- [ ] Service documented in `docs/reference/platform-services.md`
- [ ] Configuration examples added to `docs/how-to/customize-services.md`
- [ ] All tests pass (`mise run test && mise run godog`)
- [ ] Schema verification passes (`mise run schema-verify`)
- [ ] Templates render correctly (`mise run build && opencenter cluster generate`)

## Security Hardening Guidelines

All platform services must follow these security practices:

### Container Security

- **Run as non-root**: Set `runAsNonRoot: true` and `runAsUser: 1000`
- **Drop capabilities**: Drop all capabilities with `capabilities.drop: [ALL]`
- **Seccomp profile**: Use `seccompProfile.type: RuntimeDefault`
- **Read-only root filesystem**: Set `readOnlyRootFilesystem: true` when possible

### Network Security

- **Network policies**: Restrict ingress and egress traffic
- **TLS everywhere**: Use cert-manager for TLS certificates
- **Service mesh**: Consider Istio for mTLS between services

### Resource Management

- **Resource limits**: Set CPU and memory limits
- **Resource requests**: Set CPU and memory requests
- **Pod disruption budget**: Configure PDB for high availability

### Monitoring

- **ServiceMonitor**: Enable Prometheus metrics collection
- **Logging**: Ensure logs are collected by Loki
- **Tracing**: Add OpenTelemetry instrumentation when applicable

### High Availability

- **Replicas**: Run at least 2 replicas for production
- **Anti-affinity**: Spread pods across nodes
- **Health checks**: Configure liveness and readiness probes

## Common Issues

### Service fails to deploy

**Problem:** HelmRelease shows "install retries exhausted"

**Solution:** Check Helm values and chart compatibility:
```bash
# View HelmRelease status
kubectl get helmrelease -n my-service my-service -o yaml

# Check Helm release
helm list -n my-service

# View pod logs
kubectl logs -n my-service -l app.kubernetes.io/name=my-service
```

### Dependency not satisfied

**Problem:** Service requires another service that's disabled

**Solution:** Enable dependencies or update validation:
```bash
# Enable dependency
opencenter cluster set test \
  opencenter.services.cert_manager.enabled=true

# Or make dependency optional in validation
```

### Template rendering fails

**Problem:** Template syntax errors or missing fields

**Solution:** Test template rendering:
```bash
# Render templates
mise run build
./bin/opencenter cluster generate test

# Check for errors in output
# Verify generated files
```

---

## Evidence

This documentation is based on the following repository files:

- Service defaults: `internal/config/defaults.go:293-451` (services section)
- Service templates: `internal/gitops/gitops-base-dir/applications/base/services/` directory
- Template structure: `internal/gitops/copy.go`, `internal/gitops/embed.go`
- Existing services: cert-manager, kyverno, kube-prometheus-stack examples
- Service validation: `internal/config/validator.go`
- Schema generation: `internal/config/schema.go`
- Platform services reference: `docs/reference/platform-services.md`
- Service customization guide: `docs/how-to/customize-services.md`
- Security model: Ecosystem.md security architecture
