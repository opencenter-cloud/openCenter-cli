# Dependency Injection Container


## Table of Contents

- [Overview](#overview)
- [Purpose](#purpose)
- [Container Interface](#container-interface)
- [Component Registration](#component-registration)
- [Using the Container](#using-the-container)
- [Adding New Components](#adding-new-components)
- [Testing with the Container](#testing-with-the-container)
- [Circular Dependency Detection](#circular-dependency-detection)
- [Component Lifecycle](#component-lifecycle)
- [Best Practices](#best-practices)
- [Currently Registered Components](#currently-registered-components)
- [Migration from Global State](#migration-from-global-state)
- [Troubleshooting](#troubleshooting)
- [References](#references)
## Overview

The openCenter CLI uses a dependency injection (DI) container to manage component lifecycle and dependencies. This eliminates global state, improves testability, and makes the codebase more maintainable.

## Purpose

The DI container provides:
- **Lifecycle management**: Components are initialized once and reused
- **Dependency resolution**: Components automatically receive their dependencies
- **Testability**: Components can be easily mocked for testing
- **No global state**: All dependencies are explicitly passed through the container

## Container Interface

The container provides four main operations:

```go
type Container interface {
    // Register a factory function for a component
    Register(name string, constructor interface{}) error
    
    // Resolve a component by name
    Resolve(name string) (interface{}, error)
    
    // Resolve and assign to a typed pointer
    ResolveAs(name string, target interface{}) error
    
    // Register a singleton (initialized once, cached)
    Singleton(name string, constructor interface{}) error
    
    // Initialize all singletons
    Initialize() error
    
    // Cleanup all components
    Shutdown() error
}
```

## Component Registration

### Factory Registration

Use `Register()` for components that should be created fresh each time:

```go
container.Register("retryHandler", func() (resilience.RetryHandler, error) {
    config := resilience.RetryConfig{
        MaxAttempts: 3,
        BaseDelay:   1 * time.Second,
        MaxDelay:    60 * time.Second,
    }
    return resilience.NewRetryHandler(config), nil
})
```

### Singleton Registration

Use `Singleton()` for components that should be created once and reused:

```go
container.Singleton("logger", func() (*logrus.Logger, error) {
    logger := logrus.New()
    logger.SetLevel(logrus.WarnLevel)
    return logger, nil
})
```

### Dependency Resolution

The container automatically resolves dependencies based on constructor parameters:

```go
// Logger has no dependencies
container.Singleton("logger", func() (*logrus.Logger, error) {
    return logrus.New(), nil
})

// ConfigManager depends on logger
container.Singleton("configManager", func(logger *logrus.Logger) (*config.ConfigManager, error) {
    return config.NewConfigManager("", logger)
})
```

When you resolve `configManager`, the container will:
1. Check if `logger` is already initialized
2. If not, initialize `logger` first
3. Pass the logger to the `configManager` constructor
4. Return the initialized `configManager`

## Using the Container

### In main.go

The container is created and initialized in `main.go`:

```go
func main() {
    // Create and initialize DI container
    container, err := di.SetupContainer()
    if err != nil {
        os.Stderr.WriteString("Failed to initialize: " + err.Error() + "\n")
        os.Exit(1)
    }
    
    // Create context with container
    ctx := context.WithValue(context.Background(), cmd.ContainerKey, container)
    
    // Execute with context
    if err := cmd.ExecuteWithContext(ctx, version); err != nil {
        container.Shutdown()
        os.Exit(1)
    }
    
    // Cleanup
    container.Shutdown()
}
```

### In Commands

Commands retrieve the container from context:

```go
func newMyCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "mycommand",
        Short: "My command description",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Get container from context
            container, err := cmd.GetContainer(cmd.Context())
            if err != nil {
                return err
            }
            
            // Resolve dependencies
            var logger *logrus.Logger
            if err := container.ResolveAs("logger", &logger); err != nil {
                return err
            }
            
            var configMgr *config.ConfigManager
            if err := container.ResolveAs("configManager", &configMgr); err != nil {
                return err
            }
            
            // Use dependencies
            logger.Info("Running mycommand")
            cfg := configMgr.GetConfig()
            
            return nil
        },
    }
}
```

## Adding New Components

To add a new component to the container:

1. **Define the component interface** (if it doesn't exist):

```go
// internal/mypackage/interface.go
type MyService interface {
    DoSomething() error
}
```

2. **Implement the component**:

```go
// internal/mypackage/service.go
type myService struct {
    logger *logrus.Logger
}

func NewMyService(logger *logrus.Logger) MyService {
    return &myService{logger: logger}
}

func (s *myService) DoSomething() error {
    s.logger.Info("Doing something")
    return nil
}
```

3. **Register in the container** (in `internal/di/setup.go`):

```go
func SetupContainer() (Container, error) {
    container := NewContainer()
    
    // ... existing registrations ...
    
    // Register new component
    if err := container.Singleton("myService", func(logger *logrus.Logger) (mypackage.MyService, error) {
        return mypackage.NewMyService(logger), nil
    }); err != nil {
        return nil, err
    }
    
    // Initialize all singletons
    if err := container.Initialize(); err != nil {
        return nil, err
    }
    
    return container, nil
}
```

4. **Use in commands**:

```go
var myService mypackage.MyService
if err := container.ResolveAs("myService", &myService); err != nil {
    return err
}
```

## Testing with the Container

### Unit Testing Components

Test components in isolation by providing mock dependencies:

```go
func TestMyService(t *testing.T) {
    // Create mock logger
    logger := logrus.New()
    logger.SetOutput(io.Discard)
    
    // Create service with mock
    service := mypackage.NewMyService(logger)
    
    // Test the service
    err := service.DoSomething()
    if err != nil {
        t.Errorf("DoSomething() failed: %v", err)
    }
}
```

### Testing Commands

Test commands by creating a test container:

```go
func TestMyCommand(t *testing.T) {
    // Create test container
    container := di.NewContainer()
    
    // Register test dependencies
    container.Singleton("logger", func() (*logrus.Logger, error) {
        logger := logrus.New()
        logger.SetOutput(io.Discard)
        return logger, nil
    })
    
    container.Initialize()
    
    // Create context with container
    ctx := context.WithValue(context.Background(), cmd.ContainerKey, container)
    
    // Create and execute command
    cmd := newMyCommand()
    cmd.SetContext(ctx)
    
    err := cmd.Execute()
    if err != nil {
        t.Errorf("Command failed: %v", err)
    }
}
```

## Circular Dependency Detection

The container automatically detects circular dependencies:

```go
// This will fail with "circular dependency detected"
container.Singleton("serviceA", func(b *ServiceB) (*ServiceA, error) {
    return &ServiceA{B: b}, nil
})

container.Singleton("serviceB", func(a *ServiceA) (*ServiceB, error) {
    return &ServiceB{A: a}, nil
})

err := container.Initialize()
// err: "circular dependency detected: [serviceA, serviceB, serviceA]"
```

To fix circular dependencies:
1. Refactor to remove the cycle (preferred)
2. Use interfaces to break the dependency
3. Use lazy initialization

## Component Lifecycle

### Initialization Order

Components are initialized in dependency order:
1. Components with no dependencies first
2. Components that depend on already-initialized components
3. Circular dependencies are detected and rejected

### Shutdown

The container calls `Shutdown()` on components that implement it:

```go
type MyService struct {
    // ...
}

func (s *MyService) Shutdown() error {
    // Cleanup resources
    return nil
}
```

When `container.Shutdown()` is called, all components with a `Shutdown()` method will have it called.

## Best Practices

1. **Use interfaces**: Register interfaces, not concrete types
2. **Constructor injection**: Pass dependencies through constructors
3. **Single responsibility**: Each component should have one clear purpose
4. **Avoid global state**: Never use global variables for components
5. **Test with mocks**: Create test containers with mock dependencies
6. **Document dependencies**: Clearly document what each component needs
7. **Fail fast**: Return errors from constructors if initialization fails

## Currently Registered Components

The following components are currently registered in the container:

- `logger`: Structured logger (*logrus.Logger)
- `configManager`: Configuration manager (*config.ConfigManager)
- `errorFormatter`: Error formatter (ui.ErrorFormatter)

Additional components will be registered as they are implemented in other phases:
- Security components (Phase 1)
- Resilience components (Phase 2)
- Operational components (Phase 2)
- Observability components (Phase 2)

## Migration from Global State

The DI container replaces global variables that were previously used:

### Before (Global State)
```go
// cmd/root.go
var configManager *config.ConfigManager
var errorFormatter ui.ErrorFormatter

func init() {
    errorFormatter = ui.NewDefaultErrorFormatter()
}

func someCommand() {
    cfg := configManager.GetConfig()
    // ...
}
```

### After (DI Container)
```go
// cmd/root.go
func someCommand() *cobra.Command {
    return &cobra.Command{
        RunE: func(cmd *cobra.Command, args []string) error {
            container, _ := cmd.GetContainer(cmd.Context())
            
            var configMgr *config.ConfigManager
            container.ResolveAs("configManager", &configMgr)
            
            cfg := configMgr.GetConfig()
            // ...
        },
    }
}
```

## Troubleshooting

### "Component not registered"

Make sure the component is registered in `internal/di/setup.go`:

```go
container.Singleton("myComponent", func() (*MyComponent, error) {
    return NewMyComponent(), nil
})
```

### "Cannot resolve dependency"

Check that all dependencies are registered before the component that needs them:

```go
// Register logger first
container.Singleton("logger", ...)

// Then register component that depends on logger
container.Singleton("myComponent", func(logger *logrus.Logger) ...)
```

### "Circular dependency detected"

Refactor your components to remove the circular dependency. Consider:
- Using interfaces to break the cycle
- Splitting components into smaller pieces
- Using lazy initialization

## References

- Container implementation: `internal/di/container.go`
- Container setup: `internal/di/setup.go`
- Container tests: `internal/di/container_test.go`
- Usage example: `main.go` and `cmd/root.go`
