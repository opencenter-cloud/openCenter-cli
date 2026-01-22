# Built-in Service Plugins


## Table of Contents

- [Overview](#overview)
- [Built-in Services](#built-in-services)
- [Plugin Structure](#plugin-structure)
- [Usage](#usage)
- [Service Dependencies](#service-dependencies)
- [Adding a New Service Plugin](#adding-a-new-service-plugin)
- [Testing](#testing)
- [Migration from Legacy System](#migration-from-legacy-system)
- [Future Enhancements](#future-enhancements)
This directory contains the plugin implementations for all built-in openCenter services. These plugins implement the `ServicePlugin` interface and provide validation, rendering, and status reporting capabilities for each service.

## Overview

The service plugin architecture allows services to be:
- **Modular**: Each service is self-contained with its own validation logic
- **Extensible**: New services can be added without modifying core code
- **Testable**: Each plugin can be tested independently
- **Type-safe**: Plugins work with strongly-typed configuration structs

## Built-in Services

The following built-in services have been migrated to the plugin architecture:

### Monitoring Services
- **kube-prometheus-stack**: Complete monitoring stack with Prometheus, Grafana, and Alertmanager
- **alert-proxy**: Proxy for routing alerts from Alertmanager

### Security Services
- **cert-manager**: Certificate management and Let's Encrypt integration
- **keycloak**: Identity and access management
- **kyverno**: Policy engine for Kubernetes
- **rbac-manager**: RBAC management

### Storage Services
- **velero**: Backup and disaster recovery
- **etcd-backup**: Etcd backup service
- **external-snapshotter**: Volume snapshot support
- **openstack-csi**: OpenStack Cinder CSI driver
- **vsphere-csi**: vSphere CSI driver
- **postgres-operator**: PostgreSQL operator

### Logging Services
- **loki**: Log aggregation system with Swift/S3 storage support

### Networking Services
- **calico**: Network policy and CNI
- **gateway**: Gateway API implementation
- **gateway-api**: Gateway API CRDs

### GitOps Services
- **fluxcd**: GitOps continuous delivery
- **weave-gitops**: Weave GitOps dashboard
- **sources**: GitOps source management

### Core Services
- **headlamp**: Kubernetes dashboard
- **openstack-ccm**: OpenStack cloud controller manager
- **olm**: Operator Lifecycle Manager

## Plugin Structure

Each plugin implements the `ServicePlugin` interface:

```go
type ServicePlugin interface {
    Name() string
    Type() ServiceType
    Validate(config interface{}) error
    Render(ctx context.Context, config interface{}, workspace interface{}) error
    Status(config interface{}) ServiceStatus
}
```

### Plugin Files

- **prometheus_stack.go**: Prometheus monitoring stack plugin
- **cert_manager.go**: Certificate manager plugin
- **velero.go**: Velero backup plugin
- **loki.go**: Loki logging plugin
- **keycloak.go**: Keycloak identity plugin
- **default_services.go**: Plugins for services with minimal configuration
- **registry.go**: Registration of all built-in services
- **plugins_test.go**: Unit tests for individual plugins
- **integration_test.go**: Integration tests for the complete plugin system

## Usage

### Registering Built-in Services

To register all built-in services with a service registry:

```go
import (
    "github.com/rackerlabs/openCenter-cli/internal/services"
    "github.com/rackerlabs/openCenter-cli/internal/services/plugins"
)

registry := services.NewServiceRegistry()
err := plugins.RegisterBuiltInServices(registry)
if err != nil {
    // Handle error
}
```

### Getting a Service

```go
service, err := registry.GetService("kube-prometheus-stack")
if err != nil {
    // Handle error
}

// Validate configuration
err = service.Plugin.Validate(config)

// Get status
status := service.Plugin.Status(config)
```

### Resolving Dependencies

```go
// Resolve dependencies for multiple services
services := []string{"keycloak", "alert-proxy"}
resolved, err := registry.ResolveDependencies(services)
if err != nil {
    // Handle error
}

// Services are returned in dependency order
// For example: [cert-manager, keycloak, kube-prometheus-stack, alert-proxy]
```

## Service Dependencies

Some services have dependencies on other services:

- **keycloak** depends on **cert-manager**
- **alert-proxy** depends on **kube-prometheus-stack**
- **weave-gitops** depends on **fluxcd**
- **gateway** depends on **gateway-api**

The service registry automatically resolves these dependencies and ensures they are processed in the correct order.

## Adding a New Service Plugin

To add a new service plugin:

1. Create a new file in this directory (e.g., `my_service.go`)
2. Implement the `ServicePlugin` interface
3. Add the service to `RegisterBuiltInServices()` in `registry.go`
4. Add the service name to `GetBuiltInServiceNames()` in `registry.go`
5. Create tests in `plugins_test.go`

Example:

```go
package plugins

import (
    "context"
    "fmt"
    
    "github.com/rackerlabs/openCenter-cli/internal/config/services"
    svc "github.com/rackerlabs/openCenter-cli/internal/services"
)

type MyServicePlugin struct{}

func NewMyServicePlugin() svc.ServicePlugin {
    return &MyServicePlugin{}
}

func (p *MyServicePlugin) Name() string {
    return "my-service"
}

func (p *MyServicePlugin) Type() svc.ServiceType {
    return svc.ServiceTypeCore
}

func (p *MyServicePlugin) Validate(config interface{}) error {
    cfg, ok := config.(*services.MyServiceConfig)
    if !ok {
        return fmt.Errorf("invalid config type")
    }
    
    // Add validation logic
    return nil
}

func (p *MyServicePlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
    // Template rendering will be handled by the template system
    return nil
}

func (p *MyServicePlugin) Status(config interface{}) svc.ServiceStatus {
    cfg, ok := config.(*services.MyServiceConfig)
    if !ok {
        return svc.ServiceStatus{
            State:   "failed",
            Message: "Invalid configuration type",
        }
    }
    
    if !cfg.IsEnabled() {
        return svc.ServiceStatus{
            State:   "disabled",
            Message: "Service is disabled",
        }
    }
    
    return svc.ServiceStatus{
        State:   cfg.GetStatus(),
        Message: "My service",
    }
}
```

## Testing

Run all plugin tests:

```bash
go test ./internal/services/plugins/... -v
```

Run specific test suites:

```bash
# Unit tests
go test ./internal/services/plugins/... -v -run TestPrometheusStackPlugin

# Integration tests
go test ./internal/services/plugins/... -v -run TestBuiltInServicesIntegration
```

## Migration from Legacy System

The built-in services were migrated from `internal/config/services/` to this plugin architecture. The configuration structs remain in `internal/config/services/` to maintain backward compatibility, but the plugin logic is now centralized here.

### Benefits of Plugin Architecture

1. **Separation of Concerns**: Configuration types are separate from plugin logic
2. **Testability**: Each plugin can be tested independently
3. **Extensibility**: New services can be added without modifying core code
4. **Dependency Management**: Automatic dependency resolution and validation
5. **Lifecycle Hooks**: Support for pre/post install/update/remove hooks
6. **Type Safety**: Compile-time type checking for service operations

## Future Enhancements

- **Dynamic Plugin Loading**: Load plugins from external directories
- **Plugin Manifests**: YAML-based plugin definitions
- **Template Integration**: Automatic template rendering based on service configuration
- **Status Monitoring**: Real-time service status reporting
- **Lifecycle Hooks**: Pre/post install/update/remove hooks for each service
