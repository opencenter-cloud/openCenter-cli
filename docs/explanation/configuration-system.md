---
title: Configuration System Architecture
doc_type: explanation
category: architecture
tags: [configuration, schema, validation, builder, migration]
related:
  - ../reference/configuration-schema.md
  - ../how-to/cluster-init.md
  - ./provider-comparison.md
---

# Configuration System Architecture


## Table of Contents

- [Overview](#overview)
- [Core Design Principles](#core-design-principles)
- [Architecture Components](#architecture-components)
- [Design Decisions and Rationale](#design-decisions-and-rationale)
- [Configuration Lifecycle](#configuration-lifecycle)
- [Advanced Features](#advanced-features)
- [Performance Considerations](#performance-considerations)
- [Testing Strategy](#testing-strategy)
- [Future Enhancements](#future-enhancements)
- [Conclusion](#conclusion)
- [Related Documentation](#related-documentation)
This document explains the design and architecture of openCenter's configuration system, including schema validation, the builder pattern, configuration migration, and the rationale behind key design decisions.

## Overview

The configuration system is the foundation of openCenter's declarative approach to Kubernetes cluster management. It transforms a single YAML file into a complete, validated cluster specification that drives infrastructure provisioning, Kubernetes installation, and GitOps repository generation.

## Core Design Principles

### 1. Configuration as Code

All cluster specifications are declarative YAML files stored in version control. This enables:
- **Reproducibility**: Same configuration produces identical clusters
- **Auditability**: All changes tracked in Git history
- **Collaboration**: Team members can review and approve changes
- **Disaster Recovery**: Cluster configuration is backed up in Git

### 2. Schema-Driven Validation

JSON Schema (Draft 2020-12) defines the structure and constraints:
- **IDE Integration**: Auto-completion and inline validation
- **Early Error Detection**: Catch mistakes before deployment
- **Documentation**: Schema serves as machine-readable documentation
- **Versioning**: Schema version tracks breaking changes

### 3. Layered Validation

Validation occurs at multiple levels:

1. **Structural Validation**: JSON Schema validates syntax and types
2. **Semantic Validation**: Business rules check logical consistency
3. **Provider Validation**: Cloud-specific requirements verified
4. **Connectivity Validation**: Network and API accessibility checked

### 4. Sensible Defaults

Default values minimize configuration burden:
- **Convention over Configuration**: Common settings pre-configured
- **Provider-Specific Defaults**: Optimized for each cloud provider
- **Production-Ready**: Defaults suitable for production use
- **Override Capability**: All defaults can be customized

## Architecture Components

### Configuration Structure

```
Config (Root)
├── SchemaVersion: "1.0.0"
├── OpenCenter
│   ├── Meta (cluster identity)
│   ├── Infrastructure (provider config)
│   ├── Cluster (Kubernetes settings)
│   ├── GitOps (repository config)
│   ├── Storage (persistent volumes)
│   ├── Talos (optional secure Linux)
│   ├── Services (enabled services)
│   └── ManagedService (managed services)
├── OpenTofu (state backend)
├── Secrets (credentials)
├── Networking (network config)
├── Deployment (auto-deploy settings)
├── Overrides (runtime overrides)
└── Metadata (timestamps, tags)
```

### Schema Generation

The schema is generated programmatically from Go structs:

```go
// From internal/config/schema.go
func GenerateSchema(pretty bool) ([]byte, error) {
    // Base service schema for services
    baseServiceSchema := map[string]any{
        "type": "object",
        "properties": map[string]any{
            "enabled": map[string]any{
                "type": "boolean",
                "description": "Enable or disable this service",
            },
            // ... additional properties
        },
    }
    // ... build complete schema
}
```



**Key Features**:
- **Type Safety**: Schema enforces correct types (string, int, bool, array)
- **Constraints**: Min/max values, patterns, enums
- **Descriptions**: Human-readable field documentation
- **Examples**: Sample values for common fields
- **Conditional Logic**: OneOf, anyOf for mutually exclusive options

### Validation Pipeline

The validation system uses a pipeline architecture:

```go
// From internal/config/validator.go
type ClusterConfigValidator struct {
    autoRepair       bool
    pipelineAdapter  *PipelineAdapter
    suggestionEngine *SuggestionEngine
}

func (cv *ClusterConfigValidator) Validate(ctx context.Context, config *Config) *ConfigValidationResult {
    return cv.pipelineAdapter.Validate(ctx, config)
}
```

**Validation Stages**:

1. **Structure Validation**: Schema compliance
   - Type checking
   - Required fields
   - Format validation (email, URI, CIDR)

2. **Semantic Validation**: Business logic
   - Node count constraints (odd masters for HA)
   - Network subnet overlap detection
   - Service dependency checking

3. **Networking Validation**: Network plugin rules
   - Only one CNI enabled at a time
   - CNI-specific configuration requirements
   - Network CIDR validation

4. **Cloud Provider Validation**: Provider-specific checks
   - OpenStack: auth_url, credentials, image IDs
   - AWS: region, VPC, subnets
   - Talos: encryption keys, security settings



### Builder Pattern

The ConfigBuilder provides a fluent API for programmatic configuration:

```go
// From internal/config/builder.go
type ConfigBuilder interface {
    WithProvider(provider string) ConfigBuilder
    WithClusterName(name string) ConfigBuilder
    WithKubernetesVersion(version string) ConfigBuilder
    WithNodeCounts(masters, workers int) ConfigBuilder
    WithServices(services ...string) ConfigBuilder
    Build() (Config, error)
}
```

**Benefits**:
- **Type Safety**: Compile-time checking for configuration paths
- **Method Chaining**: Readable, fluent configuration building
- **Validation**: Built-in validation before Build()
- **Conditional Logic**: Provider-specific configuration with WhenProvider()

**Example Usage**:

```go
config, err := NewConfigBuilder("my-cluster").
    WithProvider("openstack").
    WithOrganization("acme-corp").
    WithKubernetesVersion("1.31.4").
    WithNodeCounts(3, 5).
    WithServices("cert-manager", "prometheus").
    WhenProvider("openstack", func(b ConfigBuilder) ConfigBuilder {
        return b.WithOpenStackConfig(osConfig)
    }).
    Build()
```

**Type-Safe Paths**:

```go
// Compile-time type safety for configuration paths
builder.WithPath(TypedConfigPaths.ClusterName, "my-cluster")
builder.WithPathInt(TypedConfigPaths.MasterCount, 3)
builder.WithPathBool(TypedConfigPaths.K8sHardening, true)
```



### Configuration Migration

The migration system handles schema evolution and directory structure changes:

```go
// From internal/config/migrator.go
type ConfigMigrator struct {
    pathResolver PathResolverInterface
    loader       ConfigLoaderInterface
    validator    ConfigValidatorInterface
}

func (cm *ConfigMigrator) MigrateToOrganization(ctx context.Context, clusterName, organization string) error {
    // Migrate from flat structure to organization-based structure
    // ...
}
```

**Migration Capabilities**:

1. **Schema Version Migration**: Upgrade configurations to new schema versions
2. **Directory Structure Migration**: Move from flat to organization-based layout
3. **Backup and Restore**: Safe migration with rollback capability
4. **Validation**: Ensure migrated configuration is valid

**Migration Process**:

```
Legacy Structure:
~/.config/openCenter/clusters/
└── my-cluster/
    ├── .my-cluster-config.yaml
    ├── secrets/
    └── inventory/

Organization Structure:
~/.config/openCenter/clusters/
└── acme-corp/
    ├── .my-cluster-config.yaml
    ├── infrastructure/
    │   └── clusters/
    │       └── my-cluster/
    ├── secrets/
    └── gitops/
```



## Design Decisions and Rationale

### Why YAML Instead of HCL or JSON?

**Decision**: Use YAML as the primary configuration format

**Rationale**:
- **Human-Readable**: YAML is more readable than JSON for large configurations
- **Comments**: YAML supports comments for documentation
- **Multi-Line Strings**: Better for SSH keys, certificates, scripts
- **Ecosystem**: Kubernetes ecosystem standardizes on YAML
- **Familiarity**: DevOps teams already know YAML

**Trade-offs**:
- YAML parsing can be ambiguous (indentation-sensitive)
- No native type safety (addressed with JSON Schema validation)
- Whitespace errors can be frustrating (mitigated with IDE integration)

### Why JSON Schema for Validation?

**Decision**: Use JSON Schema Draft 2020-12 for validation

**Rationale**:
- **IDE Integration**: VS Code, IntelliJ support JSON Schema
- **Tooling**: Extensive validation libraries available
- **Expressiveness**: Rich constraint language (patterns, enums, conditionals)
- **Documentation**: Schema serves as machine-readable docs
- **Versioning**: Schema version tracks breaking changes

**Trade-offs**:
- Schema generation code can be verbose
- Complex conditional logic can be hard to express
- Error messages sometimes cryptic (mitigated with custom validation)



### Why Builder Pattern?

**Decision**: Provide both YAML configuration and programmatic builder API

**Rationale**:
- **Flexibility**: Support both declarative (YAML) and imperative (code) approaches
- **Type Safety**: Builder provides compile-time checking
- **Testing**: Easier to construct test configurations programmatically
- **Plugins**: Extensions can build configurations without parsing YAML
- **Validation**: Builder validates incrementally during construction

**Trade-offs**:
- Maintenance burden (keep builder in sync with schema)
- Two ways to do the same thing (can confuse users)
- Builder API surface area grows with configuration complexity

### Why Organization-Based Directory Structure?

**Decision**: Organize clusters by organization, not flat structure

**Rationale**:
- **Multi-Tenancy**: Support multiple organizations/teams
- **Isolation**: Separate secrets and configurations by organization
- **Scalability**: Flat structure doesn't scale beyond ~10 clusters
- **GitOps Alignment**: Matches GitOps repository structure
- **Access Control**: Easier to implement RBAC per organization

**Migration Path**:
- Automatic detection of legacy flat structure
- Safe migration with backup and rollback
- Validation ensures successful migration
- Backward compatibility during transition



### Why Layered Validation?

**Decision**: Multiple validation stages instead of single-pass validation

**Rationale**:
- **Early Feedback**: Catch syntax errors before expensive operations
- **Separation of Concerns**: Different validators for different aspects
- **Provider Isolation**: Cloud-specific validation in provider packages
- **Performance**: Skip expensive checks if basic validation fails
- **Extensibility**: Easy to add new validation stages

**Validation Order**:
1. Schema validation (fast, catches 80% of errors)
2. Semantic validation (business rules)
3. Networking validation (CNI conflicts)
4. Provider validation (cloud-specific)
5. Connectivity validation (optional, expensive)

## Configuration Lifecycle

### 1. Initialization

```bash
mise run build
./bin/openCenter cluster init my-cluster --provider openstack
```

**Process**:
- Generate default configuration from `defaultConfig()`
- Apply provider-specific defaults
- Create organization directory structure
- Write `.my-cluster-config.yaml`
- Initialize metadata (created_at, created_by)

### 2. Loading

```go
// From internal/config/config.go
func Load(name string) (Config, error) {
    path, err := ConfigPath(name)
    data, err := os.ReadFile(path)
    
    // Expand environment variables
    expandedData := []byte(os.ExpandEnv(string(data)))
    
    // Unmarshal onto default config
    cfg := defaultConfig(clusterName)
    yaml.Unmarshal(expandedData, &cfg)
    
    return cfg, nil
}
```



**Features**:
- Environment variable expansion (`${VAR}` or `$VAR`)
- Default value overlay (missing fields get defaults)
- Schema version detection
- Metadata preservation

### 3. Validation

```bash
mise run build
./bin/openCenter cluster validate my-cluster
```

**Process**:
- Load configuration
- Run validation pipeline
- Generate validation report with suggestions
- Return structured errors with context

**Error Reporting**:
```go
type ValidationError struct {
    Field       string
    Message     string
    Suggestions []string
    Context     map[string]interface{}
}
```

### 4. Modification

**Via YAML**:
```bash
vim ~/.config/openCenter/clusters/acme-corp/.my-cluster-config.yaml
mise run build
./bin/openCenter cluster validate my-cluster
```

**Via Builder**:
```go
config, _ := Load("my-cluster")
builder := NewConfigBuilderFromConfig(config)
updated, _ := builder.
    WithWorkerCount(5).
    WithService("prometheus", true).
    Build()
```

### 5. Migration

```bash
mise run build
./bin/openCenter cluster migrate my-cluster --organization acme-corp
```

**Process**:
- Detect legacy structure
- Create backup
- Create organization structure
- Migrate files and directories
- Update configuration with organization metadata
- Validate post-migration
- Remove legacy directory if empty



## Advanced Features

### Environment Variable Expansion

Configurations support environment variable expansion for secrets:

```yaml
secrets:
  cert_manager:
    aws_access_key: ${AWS_ACCESS_KEY}
    aws_secret_access_key: ${AWS_SECRET_KEY}
```

**Benefits**:
- Avoid plaintext secrets in configuration files
- Support CI/CD pipelines with environment-based secrets
- Compatible with SOPS encryption (use SOPS for persistent secrets)

### Configuration Overrides

Runtime overrides for testing and customization:

```yaml
overrides:
  opencenter.cluster.kubernetes.version: "1.32.0"
  opencenter.cluster.kubernetes.master_count: 5
```

**Use Cases**:
- Testing different Kubernetes versions
- Temporary configuration changes
- Provider-specific tweaks without modifying base config

### Metadata Tracking

Automatic metadata for audit and lifecycle management:

```yaml
metadata:
  created_at: "2024-01-15T10:30:00Z"
  created_by: "alice@example.com"
  updated_at: "2024-01-20T14:45:00Z"
  updated_by: "bob@example.com"
  tags:
    environment: production
    team: platform
  annotations:
    cost-center: "engineering"
    compliance: "pci-dss"
```



### Path Resolution

Intelligent path resolution supports multiple directory structures:

```go
// From internal/config/config.go
func ConfigPath(name string) (string, error) {
    // Priority 1: Organization-based structure
    // Priority 2: Search all organizations
    // Priority 3: Flat config file (backward compatibility)
    // Priority 4: Legacy directory structure
}
```

**Search Order**:
1. `~/.config/openCenter/clusters/org/.cluster-config.yaml`
2. `~/.config/openCenter/clusters/org/infrastructure/clusters/cluster/.cluster-config.yaml`
3. `~/.config/openCenter/cluster.yaml` (flat)
4. `~/.config/openCenter/clusters/cluster/.cluster-config.yaml` (legacy)

## Performance Considerations

### Configuration Loading

- **Caching**: Parsed configurations cached in memory
- **Lazy Loading**: Provider-specific validation only when needed
- **Parallel Validation**: Independent validators run concurrently

### Schema Validation

- **Early Exit**: Stop on first structural error
- **Compiled Schemas**: JSON Schema compiled once, reused
- **Selective Validation**: Validate only changed sections

### Metrics Collection

```go
// From internal/config/builder.go
defer func() {
    duration := time.Since(startTime)
    metrics.RecordConfigBuild(clusterName, duration, buildErr == nil, buildErr)
}()
```

**Tracked Metrics**:
- Configuration load time
- Validation duration
- Build success/failure rate
- Migration time



## Testing Strategy

### Unit Tests

Test individual components in isolation:

```go
func TestConfigBuilder_WithProvider(t *testing.T) {
    builder := NewConfigBuilder("test-cluster")
    config, err := builder.
        WithProvider("openstack").
        Build()
    
    assert.NoError(t, err)
    assert.Equal(t, "openstack", config.OpenCenter.Infrastructure.Provider)
}
```

### Integration Tests

Test configuration lifecycle end-to-end:

```go
func TestConfigLifecycle(t *testing.T) {
    // Initialize
    config := NewDefault("test-cluster")
    
    // Save
    err := Save(config)
    
    // Load
    loaded, err := Load("test-cluster")
    
    // Validate
    validator := NewConfigValidator(false)
    result := validator.Validate(context.Background(), &loaded)
    
    assert.True(t, result.Valid)
}
```

### BDD Tests

Test user workflows with Gherkin scenarios:

```gherkin
Feature: Configuration Management
  Scenario: Initialize and validate cluster
    Given I have openCenter CLI installed
    When I run "openCenter cluster init test-cluster --provider openstack"
    Then a configuration file should be created
    And the configuration should be valid
```



## Future Enhancements

### Planned Features

1. **Configuration Diffing**: Show differences between configurations
2. **Configuration Templates**: Reusable configuration templates
3. **Policy Enforcement**: OPA-based policy validation
4. **Configuration Linting**: Style and best practice checks
5. **Interactive Configuration**: TUI for configuration editing
6. **Configuration Import**: Import from existing clusters

### Schema Evolution

**Version 2.0 Considerations**:
- Simplified service configuration (reduce nesting)
- Unified secrets management (consolidate secret fields)
- Enhanced provider abstraction (reduce provider-specific config)
- Improved validation messages (more actionable suggestions)

## Conclusion

The configuration system is the cornerstone of openCenter's declarative approach. Its layered architecture provides:

- **Flexibility**: Support both YAML and programmatic configuration
- **Safety**: Multi-stage validation catches errors early
- **Maintainability**: Clear separation of concerns
- **Extensibility**: Easy to add new providers and services
- **User Experience**: Helpful error messages and suggestions

Understanding the configuration system helps you:
- Write correct configurations faster
- Debug validation errors effectively
- Extend openCenter with custom providers
- Contribute to configuration system improvements

---

## Related Documentation

- [Reference: Configuration Schema](../reference/configuration-schema.md)
- [How-To: Initialize a Cluster](../how-to/cluster-init.md)
- [How-To: Validate Configuration](../how-to/cluster-validate.md)
- [Explanation: Provider Comparison](./provider-comparison.md)
- [Explanation: Template Engine](./template-engine.md)
