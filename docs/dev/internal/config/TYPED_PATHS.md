# Type-Safe Configuration Paths


## Table of Contents

- [Overview](#overview)
- [Usage](#usage)
- [Benefits](#benefits)
- [Available Paths](#available-paths)
- [Migration Guide](#migration-guide)
- [Implementation Details](#implementation-details)
## Overview

The configuration builder now supports compile-time type-safe paths through the `TypedConfigPath` system. This prevents common errors like typos in path strings and type mismatches that would only be caught at runtime.

## Usage

### Basic Example

Instead of using string-based paths with `WithOverride`:

```go
// Runtime validation only - typos and type errors not caught until runtime
builder.WithOverride("opencenter.meta.organization", "my-org")
builder.WithOverride("opencenter.cluster.kubernetes.master_count", 3)
```

Use type-safe paths with the `WithPath*` methods:

```go
// Compile-time validation - errors caught immediately
builder.WithPath(TypedConfigPaths.Organization, "my-org")
builder.WithPathInt(TypedConfigPaths.MasterCount, 3)
```

### Available Methods

- `WithPath(path TypedConfigPath[string], value string)` - For string values
- `WithPathInt(path TypedConfigPath[int], value int)` - For integer values
- `WithPathBool(path TypedConfigPath[bool], value bool)` - For boolean values
- `WithPathStringSlice(path TypedConfigPath[[]string], value []string)` - For string slices

### Complete Example

```go
builder := config.NewConfigBuilder("my-cluster").
    // Type-safe string paths
    WithPath(config.TypedConfigPaths.Organization, "acme-corp").
    WithPath(config.TypedConfigPaths.Provider, "openstack").
    WithPath(config.TypedConfigPaths.Environment, "production").
    
    // Type-safe integer paths
    WithPathInt(config.TypedConfigPaths.MasterCount, 3).
    WithPathInt(config.TypedConfigPaths.WorkerCount, 5).
    
    // Type-safe boolean paths
    WithPathBool(config.TypedConfigPaths.K8sHardening, true).
    WithPathBool(config.TypedConfigPaths.OSHardening, true).
    
    // Type-safe string slice paths
    WithPathStringSlice(config.TypedConfigPaths.DNSNameservers, []string{"8.8.8.8", "8.8.4.4"}).
    WithPathStringSlice(config.TypedConfigPaths.NTPServers, []string{"time.google.com"})

config, err := builder.Build()
```

## Benefits

### 1. Compile-Time Type Safety

The compiler prevents type mismatches:

```go
// ✅ Correct - compiles successfully
builder.WithPath(TypedConfigPaths.Organization, "my-org")

// ❌ Compile error - cannot use int as string
builder.WithPath(TypedConfigPaths.Organization, 123)

// ❌ Compile error - MasterCount expects int, not string
builder.WithPath(TypedConfigPaths.MasterCount, "3")
```

### 2. Path Validation

The compiler ensures you use valid paths:

```go
// ✅ Correct - TypedConfigPaths.Organization is defined
builder.WithPath(TypedConfigPaths.Organization, "my-org")

// ❌ Compile error - TypedConfigPaths.InvalidPath doesn't exist
builder.WithPath(TypedConfigPaths.InvalidPath, "value")
```

### 3. IDE Autocomplete

Your IDE can provide autocomplete for all available paths:

```go
builder.WithPath(TypedConfigPaths.  // IDE shows all available paths
```

### 4. Refactoring Safety

If configuration structure changes, the compiler will catch all affected code:

```go
// If a path is renamed or removed, all usages will fail to compile
// This prevents runtime errors from stale path strings
```

## Available Paths

### Meta Paths
- `Organization` (string)
- `ClusterName` (string)
- `Environment` (string)
- `Region` (string)

### Infrastructure Paths
- `Provider` (string)
- `SSHUser` (string)

### Kubernetes Paths
- `KubernetesVersion` (string)
- `MasterCount` (int)
- `WorkerCount` (int)
- `WindowsWorkerCount` (int)
- `SubnetPods` (string)
- `SubnetServices` (string)

### Networking Paths
- `SubnetNodes` (string)
- `DNSNameservers` ([]string)
- `NTPServers` ([]string)

### Cluster Paths
- `BaseDomain` (string)
- `AdminEmail` (string)
- `SSHAuthorizedKeys` ([]string)

### Security Paths
- `K8sHardening` (bool)
- `OSHardening` (bool)

### Storage Paths
- `DefaultStorageClass` (string)

### GitOps Paths
- `GitURL` (string)
- `GitBranch` (string)

### Secrets Paths
- `SecretsBackend` (string)

### OpenStack Paths
- `OpenStackAuthURL` (string)
- `OpenStackRegion` (string)
- `OpenStackTenantName` (string)

### AWS Paths
- `AWSRegion` (string)

## Migration Guide

### From String-Based Paths

If you have existing code using `WithOverride`:

```go
// Old approach (runtime validation)
builder.WithOverride("opencenter.meta.organization", "my-org")
builder.WithOverride("opencenter.cluster.kubernetes.master_count", 3)
builder.WithOverride("security.k8s_hardening", true)
```

Migrate to type-safe paths:

```go
// New approach (compile-time validation)
builder.WithPath(TypedConfigPaths.Organization, "my-org")
builder.WithPathInt(TypedConfigPaths.MasterCount, 3)
builder.WithPathBool(TypedConfigPaths.K8sHardening, true)
```

### Backward Compatibility

The `WithOverride` method is still available for:
- Dynamic paths that aren't known at compile time
- Paths not yet added to `TypedConfigPaths`
- Gradual migration of existing code

However, prefer type-safe paths whenever possible for better safety and maintainability.

## Implementation Details

The type-safe path system uses Go generics to enforce type constraints at compile time:

```go
type TypedConfigPath[T any] struct {
    path string
}
```

Each path constant is typed with its expected value type:

```go
var TypedConfigPaths = struct {
    Organization TypedConfigPath[string]
    MasterCount  TypedConfigPath[int]
    K8sHardening TypedConfigPath[bool]
    // ...
}{
    Organization: TypedConfigPath[string]{path: "opencenter.meta.organization"},
    MasterCount:  TypedConfigPath[int]{path: "opencenter.cluster.kubernetes.master_count"},
    K8sHardening: TypedConfigPath[bool]{path: "security.k8s_hardening"},
    // ...
}
```

This ensures that the compiler can validate both the path existence and the value type at compile time.
