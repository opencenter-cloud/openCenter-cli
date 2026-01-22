# Service Plugin System


## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Service Manifest Format](#service-manifest-format)
- [Service Types](#service-types)
- [Usage](#usage)
- [Dependency Resolution](#dependency-resolution)
- [Configuration Schema](#configuration-schema)
- [Lifecycle Hooks](#lifecycle-hooks)
- [Testing](#testing)
- [Example Manifests](#example-manifests)
- [Future Enhancements](#future-enhancements)
- [Contributing](#contributing)
- [License](#license)
The service plugin system provides a modular, extensible architecture for managing cluster services in openCenter. Services can be loaded dynamically from manifest files, with automatic dependency resolution and lifecycle management.

## Overview

The service plugin system consists of:

- **Service Plugins**: Implementations of the `ServicePlugin` interface that provide service-specific logic
- **Service Manifests**: YAML files that define service metadata, dependencies, templates, and configuration
- **Service Registry**: Central registry that manages service definitions and resolves dependencies
- **Lifecycle Hooks**: Optional hooks for service installation, updates, and removal

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Service Registry                         │
│  - Register services from manifests                          │
│  - Resolve dependencies                                      │
│  - Validate circular dependencies                           │
│  - Manage service lifecycle                                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ loads
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Service Manifests                          │
│  - YAML files defining service metadata                     │
│  - Dependencies, templates, configuration                   │
│  - Located in configurable directories                      │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ instantiates
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Service Plugins                           │
│  - Implement ServicePlugin interface                        │
│  - Provide validation, rendering, status                    │
│  - Execute lifecycle hooks                                  │
└─────────────────────────────────────────────────────────────┘
```

## Service Manifest Format

Service manifests are YAML files that define all aspects of a service:

```yaml
# Service identification
name: prometheus-stack
version: 1.0.0
type: monitoring
description: Prometheus monitoring stack with Grafana

# Service dependencies (installed first)
dependencies:
  - cert-manager
  - storage-class

# Templates used by this service
templates:
  - name: prometheus-deployment
    path: templates/prometheus/deployment.yaml
  - name: grafana-deployment
    path: templates/grafana/deployment.yaml
    condition:
      enabled: true

# Configuration schema
config:
  # JSON Schema for validation
  schema:
    enabled:
      type: boolean
    namespace:
      type: string
    retention:
      type: string
      pattern: "^[0-9]+(d|w|m|y)$"

  # Default values
  defaults:
    enabled: true
    namespace: monitoring
    retention: 30d

  # Required fields
  required:
    - enabled
    - namespace

  # Custom validation rules
  validation:
    - field: retention
      type: pattern
      operator: matches
      value: "^[0-9]+(d|w|m|y)$"
      message: "Retention must be in format like 30d, 4w, 6m, or 1y"

# Additional metadata
metadata:
  author: OpenCenter Team
  homepage: https://prometheus.io
  repository: https://github.com/prometheus/prometheus
  license: Apache-2.0
```

## Service Types

The following service types are supported:

- `core`: Core infrastructure services (required by other services)
- `monitoring`: Monitoring and observability services
- `logging`: Log aggregation and analysis services
- `storage`: Storage provisioning and management services
- `networking`: Network configuration and CNI services
- `security`: Security and access control services
- `gitops`: GitOps and continuous delivery services
- `custom`: Custom or third-party services

## Usage

### Creating a Service Registry

```go
import "github.com/rackerlabs/openCenter-cli/internal/services"

// Create a new registry
registry := services.NewServiceRegistry()
```

### Loading Services from Manifests

```go
// Load all manifests from a directory
err := registry.LoadManifestsFromDirectory("/path/to/manifests")
if err != nil {
    log.Fatalf("Failed to load manifests: %v", err)
}

// List all loaded services
services := registry.ListServices()
for _, svc := range services {
    fmt.Printf("Loaded service: %s v%s\n", svc.Name, svc.Version)
}
```

### Resolving Dependencies

```go
// Resolve dependencies for a service
resolved, err := registry.ResolveDependencies([]string{"monitoring-service"})
if err != nil {
    log.Fatalf("Failed to resolve dependencies: %v", err)
}

// Services are returned in dependency order
for _, svc := range resolved {
    fmt.Printf("Install: %s\n", svc.Name)
}
```

### Validating Dependencies

```go
// Validate that all dependencies are satisfied
err := registry.ValidateDependencies([]string{"monitoring-service"})
if err != nil {
    log.Fatalf("Dependency validation failed: %v", err)
}
```

### Implementing a Service Plugin

```go
type MyServicePlugin struct {
    name string
}

func (p *MyServicePlugin) Name() string {
    return p.name
}

func (p *MyServicePlugin) Type() services.ServiceType {
    return services.ServiceTypeCustom
}

func (p *MyServicePlugin) Validate(config interface{}) error {
    // Validate service configuration
    return nil
}

func (p *MyServicePlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
    // Render service templates to workspace
    return nil
}

func (p *MyServicePlugin) Status(config interface{}) services.ServiceStatus {
    return services.ServiceStatus{
        State:   "running",
        Message: "Service is healthy",
    }
}
```

### Registering a Custom Plugin

```go
// Create plugin instance
plugin := &MyServicePlugin{name: "my-service"}

// Load manifest
manifest, err := services.LoadManifest("my-service.yaml")
if err != nil {
    log.Fatalf("Failed to load manifest: %v", err)
}

// Register with registry
err = registry.RegisterFromManifest(manifest, plugin)
if err != nil {
    log.Fatalf("Failed to register plugin: %v", err)
}
```

## Dependency Resolution

The service registry automatically resolves dependencies and ensures:

1. **Correct Order**: Dependencies are installed before dependents
2. **Circular Detection**: Circular dependencies are detected and rejected
3. **Missing Dependencies**: Missing dependencies cause validation errors
4. **Transitive Dependencies**: All transitive dependencies are resolved

Example dependency graph:

```
monitoring-service
├── core-service (no dependencies)
└── storage-service
    └── core-service (already resolved)
```

Resolution order: `[core-service, storage-service, monitoring-service]`

## Configuration Schema

Service manifests can define configuration schemas using JSON Schema syntax:

```yaml
config:
  schema:
    replicas:
      type: integer
      minimum: 1
      maximum: 10
    resources:
      type: object
      properties:
        cpu:
          type: string
        memory:
          type: string
```

This enables:
- Type validation
- Range checking
- Required field enforcement
- Custom validation rules

## Lifecycle Hooks

Services can define lifecycle hooks that execute at specific points during service operations. Hooks are optional and only execute if defined.

### Available Hooks

- **PreInstall**: Executes before service installation (e.g., create prerequisites)
- **PostInstall**: Executes after service installation (e.g., verify deployment)
- **PreUpdate**: Executes before service update (e.g., backup current state)
- **PostUpdate**: Executes after service update (e.g., verify update)
- **PreRemove**: Executes before service removal (e.g., backup data)
- **PostRemove**: Executes after service removal (e.g., clean up resources)

### Defining Lifecycle Hooks

```go
service := services.ServiceDefinition{
    Name: "my-service",
    Type: services.ServiceTypeMonitoring,
    Lifecycle: services.ServiceLifecycle{
        PreInstall: func(ctx context.Context, config interface{}) error {
            // Prepare for installation
            log.Info("Preparing to install my-service")
            return nil
        },
        PostInstall: func(ctx context.Context, config interface{}) error {
            // Verify installation
            log.Info("Verifying my-service installation")
            return nil
        },
        PreUpdate: func(ctx context.Context, config interface{}) error {
            // Prepare for update
            log.Info("Backing up my-service configuration")
            return nil
        },
        PostUpdate: func(ctx context.Context, config interface{}) error {
            // Verify update
            log.Info("Verifying my-service update")
            return nil
        },
        PreRemove: func(ctx context.Context, config interface{}) error {
            // Prepare for removal
            log.Info("Backing up my-service data")
            return nil
        },
        PostRemove: func(ctx context.Context, config interface{}) error {
            // Clean up after removal
            log.Info("Cleaning up my-service resources")
            return nil
        },
    },
}
```

### Executing Lifecycle Hooks

#### Single Service

```go
ctx := context.Background()
config := map[string]interface{}{
    "cluster": "production",
    "namespace": "monitoring",
}

// Execute a specific hook for a service
err := registry.ExecuteLifecycleHook(ctx, "my-service", "PreInstall", config)
if err != nil {
    log.Fatalf("PreInstall hook failed: %v", err)
}

// Perform actual installation
// ...

// Execute post-install hook
err = registry.ExecuteLifecycleHook(ctx, "my-service", "PostInstall", config)
if err != nil {
    log.Fatalf("PostInstall hook failed: %v", err)
}
```

#### Multiple Services with Dependencies

```go
// Execute hooks for multiple services in dependency order
services := []string{"monitoring-service"}

// Pre-install hooks (dependencies first)
err := registry.ExecuteLifecycleHooks(ctx, services, "PreInstall", config)
if err != nil {
    log.Fatalf("PreInstall hooks failed: %v", err)
}

// Perform installations
// ...

// Post-install hooks (dependencies first)
err = registry.ExecuteLifecycleHooks(ctx, services, "PostInstall", config)
if err != nil {
    log.Fatalf("PostInstall hooks failed: %v", err)
}
```

### Hook Execution Order

**For Install/Update Operations** (dependencies first):
```
core-service:PreInstall
storage-service:PreInstall
monitoring-service:PreInstall
```

**For Removal Operations** (dependents first):
```
monitoring-service:PreRemove
storage-service:PreRemove
core-service:PreRemove
```

### Hook Behavior

1. **Optional**: Undefined hooks are skipped without error
2. **Context Propagation**: Context is passed to all hooks for cancellation/timeout
3. **Config Access**: Configuration is available to all hooks
4. **Error Handling**: Hook errors stop execution and return immediately
5. **Dependency Order**: Hooks execute in correct dependency order
6. **Reverse Order**: Removal hooks execute in reverse dependency order

### Example: Database Backup Hook

```go
PreRemove: func(ctx context.Context, config interface{}) error {
    cfg := config.(map[string]interface{})
    dbName := cfg["database"].(string)
    
    // Create backup before removal
    backupPath := fmt.Sprintf("/backups/%s-%s.sql", dbName, time.Now().Format("20060102"))
    
    cmd := exec.CommandContext(ctx, "pg_dump", "-f", backupPath, dbName)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("failed to backup database: %w", err)
    }
    
    log.Infof("Database backed up to %s", backupPath)
    return nil
}
```

### Example: Health Check Hook

```go
PostInstall: func(ctx context.Context, config interface{}) error {
    cfg := config.(map[string]interface{})
    endpoint := cfg["health_endpoint"].(string)
    
    // Wait for service to be healthy
    timeout := time.After(5 * time.Minute)
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-timeout:
            return fmt.Errorf("service did not become healthy within timeout")
        case <-ticker.C:
            resp, err := http.Get(endpoint)
            if err == nil && resp.StatusCode == 200 {
                log.Info("Service is healthy")
                return nil
            }
        }
    }
}
```

### Testing Lifecycle Hooks

```go
func TestServiceLifecycleHooks(t *testing.T) {
    ctx := context.Background()
    registry := services.NewServiceRegistry()
    
    executed := []string{}
    
    service := services.ServiceDefinition{
        Name: "test-service",
        Type: services.ServiceTypeCore,
        Lifecycle: services.ServiceLifecycle{
            PreInstall: func(ctx context.Context, cfg interface{}) error {
                executed = append(executed, "PreInstall")
                return nil
            },
            PostInstall: func(ctx context.Context, cfg interface{}) error {
                executed = append(executed, "PostInstall")
                return nil
            },
        },
        Plugin: &services.BasicServicePlugin{
            name: "test-service",
            serviceType: services.ServiceTypeCore,
        },
    }
    
    require.NoError(t, registry.RegisterService(service))
    
    // Execute install lifecycle
    err := registry.ExecuteLifecycleHook(ctx, "test-service", "PreInstall", nil)
    require.NoError(t, err)
    
    err = registry.ExecuteLifecycleHook(ctx, "test-service", "PostInstall", nil)
    require.NoError(t, err)
    
    assert.Equal(t, []string{"PreInstall", "PostInstall"}, executed)
}
```

## Testing

The service plugin system includes comprehensive tests:

```bash
# Run all tests
go test ./internal/services/...

# Run specific test
go test ./internal/services/... -run TestDynamicPluginLoading

# Run with verbose output
go test ./internal/services/... -v
```

## Example Manifests

See `internal/services/testdata/example-service.yaml` for a complete example manifest.

## Future Enhancements

Planned enhancements include:

1. **Hot Reloading**: Reload service manifests without restarting
2. **Plugin Versioning**: Support multiple versions of the same service
3. **Plugin Discovery**: Automatic discovery from multiple sources
4. **Plugin Marketplace**: Central repository of community plugins
5. **Plugin Signing**: Cryptographic verification of plugin authenticity
6. **Resource Limits**: CPU and memory limits for plugin execution
7. **Plugin Sandboxing**: Isolated execution environments for plugins

## Contributing

When adding new services:

1. Create a manifest file following the format above
2. Implement the `ServicePlugin` interface
3. Add comprehensive tests
4. Document configuration options
5. Include example configurations

## License

This service plugin system is part of openCenter and follows the same license.
