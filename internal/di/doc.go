/*
Copyright 2025 Victor Palma <victor.palma@rackspace.com>
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package di provides dependency injection container management for opencenter-cli.
//
// This package implements a lightweight dependency injection (DI) container that
// manages service lifecycle, dependency resolution, and singleton registration.
// It provides a single point of initialization for all major components, eliminating
// duplicate service registration code and ensuring consistent dependency management.
//
// # Key Features
//
//   - Singleton service registration with automatic dependency resolution
//   - Type-safe service retrieval using reflection
//   - Automatic initialization of all registered services
//   - Clear error messages for registration and initialization failures
//   - Thread-safe service access after initialization
//
// # Service Registration Pattern
//
// Services are registered as singletons with provider functions:
//
//	container := di.NewContainer()
//
//	// Register service with no dependencies
//	container.Singleton("ErrorHandler", func() (errors.ErrorHandler, error) {
//	    return errors.NewDefaultErrorHandler(), nil
//	})
//
//	// Register service with dependencies (automatically injected)
//	container.Singleton("FileSystem", func(errorHandler errors.ErrorHandler) (fs.FileSystem, error) {
//	    return fs.NewDefaultFileSystem(errorHandler), nil
//	})
//
//	// Initialize all services
//	if err := container.Initialize(); err != nil {
//	    log.Fatal(err)
//	}
//
// # Usage Examples
//
// Basic container setup:
//
//	func main() {
//	    // Create and initialize container with all services
//	    container, err := di.SetupContainer("/path/to/base/dir")
//	    if err != nil {
//	        log.Fatalf("failed to setup container: %v", err)
//	    }
//
//	    // Retrieve services
//	    fs, err := container.Get("FileSystem")
//	    if err != nil {
//	        log.Fatalf("failed to get FileSystem: %v", err)
//	    }
//	    fileSystem := fs.(fs.FileSystem)
//
//	    // Use the service
//	    data, err := fileSystem.ReadFile("/path/to/config.yaml")
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	}
//
// Custom container setup:
//
//	func setupCustomContainer() (di.Container, error) {
//	    container := di.NewContainer()
//
//	    // Register your services
//	    if err := container.Singleton("MyService", func() (*MyService, error) {
//	        return NewMyService(), nil
//	    }); err != nil {
//	        return nil, err
//	    }
//
//	    // Initialize
//	    if err := container.Initialize(); err != nil {
//	        return nil, err
//	    }
//
//	    return container, nil
//	}
//
// Service with multiple dependencies:
//
//	container.Singleton("ConfigManager", func(
//	    fs fs.FileSystem,
//	    errorHandler errors.ErrorHandler,
//	    pathResolver *paths.PathResolver,
//	) (*config.ConfigManager, error) {
//	    return config.NewConfigManager(fs, errorHandler, pathResolver), nil
//	})
//
// # Dependency Resolution
//
// The container automatically resolves dependencies:
//
//  1. Services are registered with provider functions
//  2. Provider functions declare dependencies as parameters
//  3. During Initialize(), the container invokes providers in dependency order
//  4. Dependencies are automatically injected based on parameter types
//  5. Circular dependencies are detected and reported as errors
//
// Example dependency chain:
//
//	ErrorHandler (no dependencies)
//	    ↓
//	FileSystem (depends on ErrorHandler)
//	    ↓
//	PathResolver (depends on FileSystem)
//	    ↓
//	ConfigManager (depends on FileSystem, ErrorHandler, PathResolver)
//
// # SetupContainer Function
//
// The SetupContainer function provides a pre-configured container with all
// core services registered:
//
//	container, err := di.SetupContainer(baseDir)
//	// Container includes:
//	// - ErrorHandler: Structured error handling
//	// - FileSystem: Safe file operations
//	// - PathResolver: Path resolution for clusters
//	// - Logger: Logging infrastructure
//	// - ConfigManager: Configuration management
//	// - ErrorFormatter: User-friendly error formatting
//
// Additional services can be registered after setup:
//
//	container, err := di.SetupContainer(baseDir)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Register additional service
//	container.Singleton("MyService", func(fs fs.FileSystem) (*MyService, error) {
//	    return NewMyService(fs), nil
//	})
//
// # Error Handling
//
// The container provides clear error messages for common issues:
//
// Duplicate registration:
//
//	err := container.Singleton("FileSystem", provider1)
//	err = container.Singleton("FileSystem", provider2)
//	// Error: service FileSystem already registered
//
// Missing dependency:
//
//	container.Singleton("ServiceA", func(serviceB *ServiceB) (*ServiceA, error) {
//	    return NewServiceA(serviceB), nil
//	})
//	err := container.Initialize()
//	// Error: initializing service ServiceA: service ServiceB not registered
//
// Initialization failure:
//
//	container.Singleton("Database", func() (*Database, error) {
//	    return nil, errors.New("connection failed")
//	})
//	err := container.Initialize()
//	// Error: initializing service Database: connection failed
//
// # Thread Safety
//
// The container is thread-safe after initialization:
//
//   - Service registration must happen before Initialize()
//   - Initialize() must be called from a single goroutine
//   - After Initialize(), Get() is thread-safe for concurrent access
//   - All registered services are singletons (single instance)
//
// # Best Practices
//
//  1. Use SetupContainer for standard applications
//  2. Register all services before calling Initialize()
//  3. Call Initialize() once during application startup
//  4. Retrieve services once and pass them to components
//  5. Avoid calling Get() in hot paths (cache service references)
//  6. Use interfaces for service types to enable testing
//  7. Provide clear error messages in provider functions
//
// # Testing with DI Container
//
// For testing, create a minimal container with only required services:
//
//	func setupTestContainer(t *testing.T) di.Container {
//	    container := di.NewContainer()
//
//	    // Register minimal services for testing
//	    container.Singleton("ErrorHandler", func() (errors.ErrorHandler, error) {
//	        return errors.NewDefaultErrorHandler(), nil
//	    })
//
//	    container.Singleton("FileSystem", func(eh errors.ErrorHandler) (fs.FileSystem, error) {
//	        return fs.NewDefaultFileSystem(eh), nil
//	    })
//
//	    if err := container.Initialize(); err != nil {
//	        t.Fatalf("failed to initialize test container: %v", err)
//	    }
//
//	    return container
//	}
//
// Or use mock services:
//
//	container.Singleton("FileSystem", func() (fs.FileSystem, error) {
//	    return &MockFileSystem{}, nil
//	})
//
// # Performance
//
// The DI container has minimal overhead:
//
//   - Service registration: <0.1ms per service
//   - Initialize(): ~1-5ms depending on service count
//   - Get(): <0.01ms (simple map lookup after initialization)
//
// The initialization overhead is negligible compared to typical application
// startup time, and the runtime overhead is effectively zero.
//
// # Future Extensions
//
// The container is designed to support future enhancements:
//
//   - Scoped services (per-request lifecycle)
//   - Factory services (new instance per Get())
//   - Service decorators and middleware
//   - Lazy initialization (initialize on first Get())
//   - Service health checks
//   - Graceful shutdown hooks
//
// These features will be added as needed in future phases of the refactoring.
package di
