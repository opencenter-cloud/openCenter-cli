// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package di

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/cluster"
	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

// TestFullDependencyChain tests a complete dependency chain resolution.
func TestFullDependencyChain(t *testing.T) {
	container := NewContainer()
	tmpDir := t.TempDir()

	// Register all components in dependency order
	if err := container.Register("logger", ProvideLogger); err != nil {
		t.Fatalf("Failed to register logger: %v", err)
	}

	if err := container.Register("pathResolver", func() (*paths.PathResolver, error) {
		return ProvidePathResolver(tmpDir)
	}); err != nil {
		t.Fatalf("Failed to register pathResolver: %v", err)
	}

	if err := container.Register("validationEngine", ProvideValidationEngine); err != nil {
		t.Fatalf("Failed to register validationEngine: %v", err)
	}

	if err := container.Register("configManager", ProvideConfigManager); err != nil {
		t.Fatalf("Failed to register configManager: %v", err)
	}

	if err := container.Register("initService", ProvideInitService); err != nil {
		t.Fatalf("Failed to register initService: %v", err)
	}

	// Resolve the service - should automatically resolve all dependencies
	instance, err := container.Resolve("initService")
	if err != nil {
		t.Fatalf("Failed to resolve initService: %v", err)
	}

	service, ok := instance.(*cluster.InitService)
	if !ok {
		t.Fatal("Resolved instance is not an InitService")
	}
	if service == nil {
		t.Error("InitService is nil")
	}
}

// TestMultipleServicesWithSharedDependencies tests multiple services sharing dependencies.
func TestMultipleServicesWithSharedDependencies(t *testing.T) {
	container := NewContainer()
	tmpDir := t.TempDir()

	// Register shared dependencies as singletons
	if err := container.Singleton("pathResolver", func() (*paths.PathResolver, error) {
		return ProvidePathResolver(tmpDir)
	}); err != nil {
		t.Fatalf("Failed to register pathResolver: %v", err)
	}

	if err := container.Singleton("validationEngine", ProvideValidationEngine); err != nil {
		t.Fatalf("Failed to register validationEngine: %v", err)
	}

	if err := container.Singleton("configManager", ProvideConfigManager); err != nil {
		t.Fatalf("Failed to register configManager: %v", err)
	}

	// Initialize singletons
	if err := container.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Register all services
	if err := container.Register("initService", ProvideInitService); err != nil {
		t.Fatalf("Failed to register initService: %v", err)
	}

	if err := container.Register("validateService", ProvideValidateService); err != nil {
		t.Fatalf("Failed to register validateService: %v", err)
	}

	if err := container.Register("setupService", ProvideSetupService); err != nil {
		t.Fatalf("Failed to register setupService: %v", err)
	}

	if err := container.Register("bootstrapService", ProvideBootstrapService); err != nil {
		t.Fatalf("Failed to register bootstrapService: %v", err)
	}

	// Resolve all services
	initSvc, err := container.Resolve("initService")
	if err != nil {
		t.Errorf("Failed to resolve initService: %v", err)
	}

	validateSvc, err := container.Resolve("validateService")
	if err != nil {
		t.Errorf("Failed to resolve validateService: %v", err)
	}

	setupSvc, err := container.Resolve("setupService")
	if err != nil {
		t.Errorf("Failed to resolve setupService: %v", err)
	}

	bootstrapSvc, err := container.Resolve("bootstrapService")
	if err != nil {
		t.Errorf("Failed to resolve bootstrapService: %v", err)
	}

	// Verify all services are non-nil
	if initSvc == nil {
		t.Error("InitService is nil")
	}
	if validateSvc == nil {
		t.Error("ValidateService is nil")
	}
	if setupSvc == nil {
		t.Error("SetupService is nil")
	}
	if bootstrapSvc == nil {
		t.Error("BootstrapService is nil")
	}
}

// TestSetupContainerIntegration tests the SetupContainer function.
func TestSetupContainerIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	container, err := SetupContainer(tmpDir)
	if err != nil {
		t.Fatalf("SetupContainer() failed: %v", err)
	}

	// Verify core components are registered and initialized
	logger, err := container.Resolve("logger")
	if err != nil {
		t.Errorf("Failed to resolve logger: %v", err)
	}
	if logger == nil {
		t.Error("Logger is nil")
	}

	configMgr, err := container.Resolve("configManager")
	if err != nil {
		t.Errorf("Failed to resolve configManager: %v", err)
	}
	if configMgr == nil {
		t.Error("ConfigManager is nil")
	}

	errorFormatter, err := container.Resolve("errorFormatter")
	if err != nil {
		t.Errorf("Failed to resolve errorFormatter: %v", err)
	}
	if errorFormatter == nil {
		t.Error("ErrorFormatter is nil")
	}
}

// TestDependencyResolutionWithRealComponents tests dependency resolution with actual components.
func TestDependencyResolutionWithRealComponents(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "clusters")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("Failed to create base directory: %v", err)
	}

	container := NewContainer()

	// Register PathResolver
	if err := container.Register("pathResolver", func() (*paths.PathResolver, error) {
		return paths.NewPathResolver(baseDir), nil
	}); err != nil {
		t.Fatalf("Failed to register pathResolver: %v", err)
	}

	// Register ValidationEngine
	if err := container.Register("validationEngine", func() (*validation.ValidationEngine, error) {
		engine := validation.NewValidationEngine()
		// Register validators
		return engine, nil
	}); err != nil {
		t.Fatalf("Failed to register validationEngine: %v", err)
	}

	// Register ConfigManager
	if err := container.Register("configManager", func() (*config.ConfigManager, error) {
		return config.NewConfigManager("")
	}); err != nil {
		t.Fatalf("Failed to register configManager: %v", err)
	}

	// Register InitService with dependencies
	if err := container.Register("initService", func(
		pr *paths.PathResolver,
		ve *validation.ValidationEngine,
		cm *config.ConfigManager,
	) (*cluster.InitService, error) {
		return cluster.NewInitService(pr, ve, cm), nil
	}); err != nil {
		t.Fatalf("Failed to register initService: %v", err)
	}

	// Resolve InitService - should automatically resolve dependencies
	instance, err := container.Resolve("initService")
	if err != nil {
		t.Fatalf("Failed to resolve initService: %v", err)
	}

	service, ok := instance.(*cluster.InitService)
	if !ok {
		t.Fatal("Resolved instance is not an InitService")
	}
	if service == nil {
		t.Error("InitService is nil")
	}
}

// TestComplexDependencyGraph tests a complex dependency graph.
func TestComplexDependencyGraph(t *testing.T) {
	container := NewContainer()
	tmpDir := t.TempDir()

	// Create a complex dependency graph:
	// InitService -> PathResolver, ValidationEngine
	// ValidateService -> PathResolver, ValidationEngine, ConfigManager
	// SetupService -> PathResolver, ValidationEngine
	// BootstrapService -> PathResolver, ValidationEngine

	// Register shared dependencies as singletons
	if err := container.Singleton("pathResolver", func() (*paths.PathResolver, error) {
		return ProvidePathResolver(tmpDir)
	}); err != nil {
		t.Fatalf("Failed to register pathResolver: %v", err)
	}

	if err := container.Singleton("validationEngine", ProvideValidationEngine); err != nil {
		t.Fatalf("Failed to register validationEngine: %v", err)
	}

	if err := container.Singleton("configManager", ProvideConfigManager); err != nil {
		t.Fatalf("Failed to register configManager: %v", err)
	}

	// Initialize singletons
	if err := container.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Register services
	services := map[string]func(*paths.PathResolver, *validation.ValidationEngine, *config.ConfigManager) (interface{}, error){
		"initService": func(pr *paths.PathResolver, ve *validation.ValidationEngine, cm *config.ConfigManager) (interface{}, error) {
			return ProvideInitService(pr, ve, cm)
		},
		"validateService": func(pr *paths.PathResolver, ve *validation.ValidationEngine, cm *config.ConfigManager) (interface{}, error) {
			return ProvideValidateService(pr, ve, cm)
		},
		"setupService": func(pr *paths.PathResolver, ve *validation.ValidationEngine, cm *config.ConfigManager) (interface{}, error) {
			return ProvideSetupService(pr, ve)
		},
		"bootstrapService": func(pr *paths.PathResolver, ve *validation.ValidationEngine, cm *config.ConfigManager) (interface{}, error) {
			return ProvideBootstrapService(pr, ve)
		},
	}

	for name, provider := range services {
		if err := container.Register(name, provider); err != nil {
			t.Fatalf("Failed to register %s: %v", name, err)
		}
	}

	// Resolve all services and verify they work
	for name := range services {
		instance, err := container.Resolve(name)
		if err != nil {
			t.Errorf("Failed to resolve %s: %v", name, err)
		}
		if instance == nil {
			t.Errorf("%s is nil", name)
		}
	}
}

// TestServiceLifecycle tests the complete lifecycle of services.
func TestServiceLifecycle(t *testing.T) {
	container := NewContainer()
	tmpDir := t.TempDir()

	// Register components
	if err := container.Singleton("pathResolver", func() (*paths.PathResolver, error) {
		return ProvidePathResolver(tmpDir)
	}); err != nil {
		t.Fatalf("Failed to register pathResolver: %v", err)
	}

	if err := container.Singleton("validationEngine", ProvideValidationEngine); err != nil {
		t.Fatalf("Failed to register validationEngine: %v", err)
	}

	if err := container.Singleton("configManager", ProvideConfigManager); err != nil {
		t.Fatalf("Failed to register configManager: %v", err)
	}

	// Initialize
	if err := container.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Register service
	if err := container.Register("initService", ProvideInitService); err != nil {
		t.Fatalf("Failed to register initService: %v", err)
	}

	// Resolve
	instance, err := container.Resolve("initService")
	if err != nil {
		t.Fatalf("Failed to resolve initService: %v", err)
	}
	if instance == nil {
		t.Fatal("InitService is nil")
	}

	// Shutdown
	if err := container.Shutdown(); err != nil {
		t.Errorf("Failed to shutdown: %v", err)
	}

	// After shutdown, singletons should be cleared but can be re-initialized
	if err := container.Initialize(); err != nil {
		t.Errorf("Failed to re-initialize after shutdown: %v", err)
	}

	// Resolve again
	instance2, err := container.Resolve("initService")
	if err != nil {
		t.Errorf("Failed to resolve initService after re-initialization: %v", err)
	}
	if instance2 == nil {
		t.Error("InitService is nil after re-initialization")
	}
}

// TestValidationEngineIntegration tests ValidationEngine integration with container.
func TestValidationEngineIntegration(t *testing.T) {
	container := NewContainer()

	// Register ValidationEngine
	if err := container.Register("validationEngine", ProvideValidationEngine); err != nil {
		t.Fatalf("Failed to register validationEngine: %v", err)
	}

	// Resolve
	instance, err := container.Resolve("validationEngine")
	if err != nil {
		t.Fatalf("Failed to resolve validationEngine: %v", err)
	}

	engine, ok := instance.(*validation.ValidationEngine)
	if !ok {
		t.Fatal("Resolved instance is not a ValidationEngine")
	}

	// Verify validators are registered
	ctx := context.Background()
	result, err := engine.Validate(ctx, "cluster-name", "test-cluster")
	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}
	if !result.Valid {
		t.Errorf("Expected valid cluster name, got errors: %v", result.Errors)
	}
}

// TestPathResolverIntegration tests PathResolver integration with container.
func TestPathResolverIntegration(t *testing.T) {
	tmpDir := t.TempDir()
	container := NewContainer()

	// Create the necessary directory structure
	orgDir := filepath.Join(tmpDir, "test-org", "infrastructure", "clusters", "test-cluster")
	if err := os.MkdirAll(orgDir, 0755); err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}

	// Register PathResolver
	if err := container.Register("pathResolver", func() (*paths.PathResolver, error) {
		return ProvidePathResolver(tmpDir)
	}); err != nil {
		t.Fatalf("Failed to register pathResolver: %v", err)
	}

	// Resolve
	instance, err := container.Resolve("pathResolver")
	if err != nil {
		t.Fatalf("Failed to resolve pathResolver: %v", err)
	}

	resolver, ok := instance.(*paths.PathResolver)
	if !ok {
		t.Fatal("Resolved instance is not a PathResolver")
	}

	// Test path resolution
	ctx := context.Background()
	clusterPaths, err := resolver.Resolve(ctx, "test-cluster", "test-org")
	if err != nil {
		t.Errorf("Path resolution failed: %v", err)
	}
	if clusterPaths == nil {
		t.Error("ClusterPaths is nil")
	}
}

// TestErrorHandlingInDependencyResolution tests error handling during dependency resolution.
func TestErrorHandlingInDependencyResolution(t *testing.T) {
	container := NewContainer()

	// Register a component that returns an error
	if err := container.Register("failing", func() (*string, error) {
		return nil, os.ErrNotExist
	}); err != nil {
		t.Fatalf("Failed to register failing component: %v", err)
	}

	// Register a component that depends on the failing component
	if err := container.Register("dependent", func(s *string) (*int, error) {
		val := 42
		return &val, nil
	}); err != nil {
		t.Fatalf("Failed to register dependent component: %v", err)
	}

	// Resolve should fail with the error from the failing component
	_, err := container.Resolve("dependent")
	if err == nil {
		t.Error("Expected error when resolving dependent component")
	}
}

// TestResolveWithMixedSingletonsAndTransients tests mixed singleton and transient components.
func TestResolveWithMixedSingletonsAndTransients(t *testing.T) {
	container := NewContainer()
	tmpDir := t.TempDir()

	// Register PathResolver as singleton
	if err := container.Singleton("pathResolver", func() (*paths.PathResolver, error) {
		return ProvidePathResolver(tmpDir)
	}); err != nil {
		t.Fatalf("Failed to register pathResolver: %v", err)
	}

	// Register ValidationEngine as transient
	if err := container.Register("validationEngine", ProvideValidationEngine); err != nil {
		t.Fatalf("Failed to register validationEngine: %v", err)
	}

	// Register ConfigManager as singleton
	if err := container.Singleton("configManager", ProvideConfigManager); err != nil {
		t.Fatalf("Failed to register configManager: %v", err)
	}

	// Initialize singletons
	if err := container.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Register service that depends on both
	if err := container.Register("initService", ProvideInitService); err != nil {
		t.Fatalf("Failed to register initService: %v", err)
	}

	// Resolve multiple times
	instance1, err := container.Resolve("initService")
	if err != nil {
		t.Errorf("Failed to resolve initService: %v", err)
	}

	instance2, err := container.Resolve("initService")
	if err != nil {
		t.Errorf("Failed to resolve initService: %v", err)
	}

	// Instances should be different (transient)
	if instance1 == instance2 {
		t.Error("Expected different instances for transient service")
	}
}
