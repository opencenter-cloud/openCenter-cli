package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDynamicPluginLoading demonstrates the complete workflow of loading
// service plugins dynamically from manifest files
func TestDynamicPluginLoading(t *testing.T) {
	// Create a temporary directory for manifests
	tmpDir := t.TempDir()

	// Create multiple service manifests with dependencies
	manifests := map[string]string{
		"core.yaml": `name: core-service
version: 1.0.0
type: core
description: Core infrastructure service
templates:
  - name: namespace
    path: templates/namespace.yaml
config:
  defaults:
    enabled: true
metadata:
  author: OpenCenter Team
`,
		"storage.yaml": `name: storage-service
version: 1.0.0
type: storage
description: Storage provisioning service
dependencies:
  - core-service
templates:
  - name: storage-class
    path: templates/storage-class.yaml
config:
  defaults:
    enabled: true
    class: standard
metadata:
  author: OpenCenter Team
`,
		"monitoring.yaml": `name: monitoring-service
version: 2.0.0
type: monitoring
description: Prometheus monitoring stack
dependencies:
  - core-service
  - storage-service
templates:
  - name: prometheus
    path: templates/prometheus.yaml
  - name: grafana
    path: templates/grafana.yaml
config:
  schema:
    retention:
      type: string
  defaults:
    enabled: true
    retention: 30d
  required:
    - enabled
metadata:
  author: OpenCenter Team
  homepage: https://prometheus.io
`,
	}

	// Write manifest files
	for filename, content := range manifests {
		path := filepath.Join(tmpDir, filename)
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Create a new service registry
	registry := NewServiceRegistry()

	// Load all manifests from the directory
	err := registry.LoadManifestsFromDirectory(tmpDir)
	require.NoError(t, err, "Failed to load manifests from directory")

	// Verify all services were loaded
	services := registry.ListServices()
	assert.Len(t, services, 3, "Expected 3 services to be loaded")

	// Verify core service
	t.Run("verify core service", func(t *testing.T) {
		core, err := registry.GetService("core-service")
		require.NoError(t, err)
		assert.Equal(t, "core-service", core.Name)
		assert.Equal(t, "1.0.0", core.Version)
		assert.Equal(t, ServiceTypeCore, core.Type)
		assert.Empty(t, core.Dependencies)
		assert.NotNil(t, core.Plugin)
	})

	// Verify storage service
	t.Run("verify storage service", func(t *testing.T) {
		storage, err := registry.GetService("storage-service")
		require.NoError(t, err)
		assert.Equal(t, "storage-service", storage.Name)
		assert.Equal(t, ServiceTypeStorage, storage.Type)
		assert.Len(t, storage.Dependencies, 1)
		assert.Equal(t, "core-service", storage.Dependencies[0])
	})

	// Verify monitoring service
	t.Run("verify monitoring service", func(t *testing.T) {
		monitoring, err := registry.GetService("monitoring-service")
		require.NoError(t, err)
		assert.Equal(t, "monitoring-service", monitoring.Name)
		assert.Equal(t, "2.0.0", monitoring.Version)
		assert.Equal(t, ServiceTypeMonitoring, monitoring.Type)
		assert.Len(t, monitoring.Dependencies, 2)
		assert.Contains(t, monitoring.Dependencies, "core-service")
		assert.Contains(t, monitoring.Dependencies, "storage-service")
	})

	// Test dependency resolution
	t.Run("resolve dependencies", func(t *testing.T) {
		// Request monitoring service - should resolve all dependencies
		resolved, err := registry.ResolveDependencies([]string{"monitoring-service"})
		require.NoError(t, err)
		require.Len(t, resolved, 3)

		// Verify order: dependencies come before dependents
		names := make([]string, len(resolved))
		for i, svc := range resolved {
			names[i] = svc.Name
		}

		// Core should come first (no dependencies)
		assert.Equal(t, "core-service", names[0])

		// Storage should come before monitoring (monitoring depends on it)
		storageIdx := -1
		monitoringIdx := -1
		for i, name := range names {
			if name == "storage-service" {
				storageIdx = i
			}
			if name == "monitoring-service" {
				monitoringIdx = i
			}
		}
		assert.True(t, storageIdx < monitoringIdx, "Storage should be resolved before monitoring")
	})

	// Test validation
	t.Run("validate dependencies", func(t *testing.T) {
		err := registry.ValidateDependencies([]string{"monitoring-service"})
		assert.NoError(t, err, "All dependencies should be satisfied")
	})
}

// TestPluginLoadingWithInvalidManifest tests error handling for invalid manifests
func TestPluginLoadingWithInvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid manifest (missing required fields)
	invalidManifest := `name: invalid-service
# Missing version field
type: custom
`
	manifestPath := filepath.Join(tmpDir, "invalid.yaml")
	err := os.WriteFile(manifestPath, []byte(invalidManifest), 0644)
	require.NoError(t, err)

	// Try to load manifests
	registry := NewServiceRegistry()
	err = registry.LoadManifestsFromDirectory(tmpDir)
	assert.Error(t, err, "Should fail to load invalid manifest")
	assert.Contains(t, err.Error(), "version")
}

// TestPluginLoadingWithCircularDependencies tests circular dependency detection
func TestPluginLoadingWithCircularDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	manifests := map[string]string{
		"service-a.yaml": `name: service-a
version: 1.0.0
type: custom
dependencies:
  - service-b
`,
		"service-b.yaml": `name: service-b
version: 1.0.0
type: custom
dependencies:
  - service-a
`,
	}

	for filename, content := range manifests {
		path := filepath.Join(tmpDir, filename)
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	registry := NewServiceRegistry()
	err := registry.LoadManifestsFromDirectory(tmpDir)
	require.NoError(t, err, "Loading should succeed")

	// Validation should detect circular dependency
	err = registry.ValidateDependencies([]string{"service-a"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

// TestPluginLoadingFromMultipleDirectories demonstrates loading from multiple sources
func TestPluginLoadingFromMultipleDirectories(t *testing.T) {
	// Create two directories with different manifests
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// Directory 1: Core services
	coreManifest := `name: core
version: 1.0.0
type: core
`
	err := os.WriteFile(filepath.Join(dir1, "core.yaml"), []byte(coreManifest), 0644)
	require.NoError(t, err)

	// Directory 2: Add-on services
	addonManifest := `name: addon
version: 1.0.0
type: custom
dependencies:
  - core
`
	err = os.WriteFile(filepath.Join(dir2, "addon.yaml"), []byte(addonManifest), 0644)
	require.NoError(t, err)

	// Load from both directories
	registry := NewServiceRegistry()
	err = registry.LoadManifestsFromDirectory(dir1)
	require.NoError(t, err)
	err = registry.LoadManifestsFromDirectory(dir2)
	require.NoError(t, err)

	// Verify both services are loaded
	services := registry.ListServices()
	assert.Len(t, services, 2)

	// Verify dependency resolution works across directories
	resolved, err := registry.ResolveDependencies([]string{"addon"})
	require.NoError(t, err)
	assert.Len(t, resolved, 2)
	assert.Equal(t, "core", resolved[0].Name)
	assert.Equal(t, "addon", resolved[1].Name)
}

// TestManifestTemplateReferences tests that template references are preserved
func TestManifestTemplateReferences(t *testing.T) {
	tmpDir := t.TempDir()

	manifest := `name: templated-service
version: 1.0.0
type: custom
templates:
  - name: deployment
    path: templates/deployment.yaml
  - name: service
    path: templates/service.yaml
    condition:
      enabled: true
  - name: ingress
    path: templates/ingress.yaml
    condition:
      ingress_enabled: true
`
	manifestPath := filepath.Join(tmpDir, "service.yaml")
	err := os.WriteFile(manifestPath, []byte(manifest), 0644)
	require.NoError(t, err)

	registry := NewServiceRegistry()
	err = registry.LoadManifestsFromDirectory(tmpDir)
	require.NoError(t, err)

	service, err := registry.GetService("templated-service")
	require.NoError(t, err)

	// Verify templates are preserved
	assert.Len(t, service.Templates, 3)
	assert.Equal(t, "deployment", service.Templates[0].Name)
	assert.Equal(t, "templates/deployment.yaml", service.Templates[0].Path)
	assert.Equal(t, "service", service.Templates[1].Name)
	assert.NotNil(t, service.Templates[1].Condition)
}

// TestManifestConfigSchema tests that configuration schema is preserved
func TestManifestConfigSchema(t *testing.T) {
	tmpDir := t.TempDir()

	manifest := `name: configured-service
version: 1.0.0
type: custom
config:
  schema:
    enabled:
      type: boolean
    replicas:
      type: integer
  defaults:
    enabled: true
    replicas: 3
  required:
    - enabled
  validation:
    - field: replicas
      type: range
      operator: between
      value: "1-10"
      message: "Replicas must be between 1 and 10"
`
	manifestPath := filepath.Join(tmpDir, "service.yaml")
	err := os.WriteFile(manifestPath, []byte(manifest), 0644)
	require.NoError(t, err)

	// Load manifest directly to verify config is parsed
	loadedManifest, err := LoadManifest(manifestPath)
	require.NoError(t, err)

	// Verify config schema
	assert.NotNil(t, loadedManifest.Config.Schema)
	assert.Contains(t, loadedManifest.Config.Schema, "enabled")
	assert.Contains(t, loadedManifest.Config.Schema, "replicas")

	// Verify defaults
	assert.NotNil(t, loadedManifest.Config.Defaults)
	assert.Equal(t, true, loadedManifest.Config.Defaults["enabled"])
	assert.Equal(t, 3, loadedManifest.Config.Defaults["replicas"])

	// Verify required fields
	assert.Len(t, loadedManifest.Config.Required, 1)
	assert.Equal(t, "enabled", loadedManifest.Config.Required[0])

	// Verify validation rules
	assert.Len(t, loadedManifest.Config.Validation, 1)
	assert.Equal(t, "replicas", loadedManifest.Config.Validation[0].Field)
}

// TestCompleteLifecycleWorkflow demonstrates a complete service lifecycle with hooks
func TestCompleteLifecycleWorkflow(t *testing.T) {
	ctx := context.Background()
	registry := NewServiceRegistry()

	// Track lifecycle events
	events := []string{}
	config := map[string]interface{}{
		"cluster": "test-cluster",
	}

	// Create a service with full lifecycle hooks
	service := ServiceDefinition{
		Name: "test-service",
		Type: ServiceTypeCore,
		Lifecycle: ServiceLifecycle{
			PreInstall: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "PreInstall")
				return nil
			},
			PostInstall: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "PostInstall")
				return nil
			},
			PreUpdate: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "PreUpdate")
				return nil
			},
			PostUpdate: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "PostUpdate")
				return nil
			},
			PreRemove: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "PreRemove")
				return nil
			},
			PostRemove: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "PostRemove")
				return nil
			},
		},
		Plugin: &BasicServicePlugin{
			name:        "test-service",
			serviceType: ServiceTypeCore,
		},
	}

	require.NoError(t, registry.RegisterService(service))

	// Simulate install lifecycle
	t.Run("install lifecycle", func(t *testing.T) {
		events = []string{} // Reset events

		err := registry.ExecuteLifecycleHook(ctx, "test-service", "PreInstall", config)
		require.NoError(t, err)

		// Simulate actual installation work here
		// ...

		err = registry.ExecuteLifecycleHook(ctx, "test-service", "PostInstall", config)
		require.NoError(t, err)

		assert.Equal(t, []string{"PreInstall", "PostInstall"}, events)
	})

	// Simulate update lifecycle
	t.Run("update lifecycle", func(t *testing.T) {
		events = []string{} // Reset events

		err := registry.ExecuteLifecycleHook(ctx, "test-service", "PreUpdate", config)
		require.NoError(t, err)

		// Simulate actual update work here
		// ...

		err = registry.ExecuteLifecycleHook(ctx, "test-service", "PostUpdate", config)
		require.NoError(t, err)

		assert.Equal(t, []string{"PreUpdate", "PostUpdate"}, events)
	})

	// Simulate removal lifecycle
	t.Run("removal lifecycle", func(t *testing.T) {
		events = []string{} // Reset events

		err := registry.ExecuteLifecycleHook(ctx, "test-service", "PreRemove", config)
		require.NoError(t, err)

		// Simulate actual removal work here
		// ...

		err = registry.ExecuteLifecycleHook(ctx, "test-service", "PostRemove", config)
		require.NoError(t, err)

		assert.Equal(t, []string{"PreRemove", "PostRemove"}, events)
	})
}

// TestMultiServiceLifecycleWorkflow demonstrates lifecycle hooks with multiple dependent services
func TestMultiServiceLifecycleWorkflow(t *testing.T) {
	ctx := context.Background()
	registry := NewServiceRegistry()

	// Track lifecycle events with service names
	events := []string{}
	config := map[string]interface{}{"cluster": "test-cluster"}

	// Create core service
	core := ServiceDefinition{
		Name: "core",
		Type: ServiceTypeCore,
		Lifecycle: ServiceLifecycle{
			PreInstall: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "core:PreInstall")
				return nil
			},
			PostInstall: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "core:PostInstall")
				return nil
			},
			PreRemove: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "core:PreRemove")
				return nil
			},
			PostRemove: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "core:PostRemove")
				return nil
			},
		},
		Plugin: &BasicServicePlugin{name: "core", serviceType: ServiceTypeCore},
	}

	// Create storage service (depends on core)
	storage := ServiceDefinition{
		Name:         "storage",
		Type:         ServiceTypeStorage,
		Dependencies: []string{"core"},
		Lifecycle: ServiceLifecycle{
			PreInstall: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "storage:PreInstall")
				return nil
			},
			PostInstall: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "storage:PostInstall")
				return nil
			},
			PreRemove: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "storage:PreRemove")
				return nil
			},
			PostRemove: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "storage:PostRemove")
				return nil
			},
		},
		Plugin: &BasicServicePlugin{name: "storage", serviceType: ServiceTypeStorage},
	}

	// Create monitoring service (depends on core and storage)
	monitoring := ServiceDefinition{
		Name:         "monitoring",
		Type:         ServiceTypeMonitoring,
		Dependencies: []string{"core", "storage"},
		Lifecycle: ServiceLifecycle{
			PreInstall: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "monitoring:PreInstall")
				return nil
			},
			PostInstall: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "monitoring:PostInstall")
				return nil
			},
			PreRemove: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "monitoring:PreRemove")
				return nil
			},
			PostRemove: func(ctx context.Context, cfg interface{}) error {
				events = append(events, "monitoring:PostRemove")
				return nil
			},
		},
		Plugin: &BasicServicePlugin{name: "monitoring", serviceType: ServiceTypeMonitoring},
	}

	require.NoError(t, registry.RegisterService(core))
	require.NoError(t, registry.RegisterService(storage))
	require.NoError(t, registry.RegisterService(monitoring))

	// Test install lifecycle - should execute in dependency order
	t.Run("install all services", func(t *testing.T) {
		events = []string{} // Reset events

		// Execute PreInstall for all services
		err := registry.ExecuteLifecycleHooks(ctx, []string{"monitoring"}, "PreInstall", config)
		require.NoError(t, err)

		// Execute PostInstall for all services
		err = registry.ExecuteLifecycleHooks(ctx, []string{"monitoring"}, "PostInstall", config)
		require.NoError(t, err)

		// Verify order: core -> storage -> monitoring
		expectedEvents := []string{
			"core:PreInstall",
			"storage:PreInstall",
			"monitoring:PreInstall",
			"core:PostInstall",
			"storage:PostInstall",
			"monitoring:PostInstall",
		}
		assert.Equal(t, expectedEvents, events)
	})

	// Test removal lifecycle - should execute in reverse order
	t.Run("remove all services", func(t *testing.T) {
		events = []string{} // Reset events

		// Execute PreRemove for all services
		err := registry.ExecuteLifecycleHooks(ctx, []string{"monitoring"}, "PreRemove", config)
		require.NoError(t, err)

		// Execute PostRemove for all services
		err = registry.ExecuteLifecycleHooks(ctx, []string{"monitoring"}, "PostRemove", config)
		require.NoError(t, err)

		// Verify order: monitoring -> storage -> core (reverse)
		expectedEvents := []string{
			"monitoring:PreRemove",
			"storage:PreRemove",
			"core:PreRemove",
			"monitoring:PostRemove",
			"storage:PostRemove",
			"core:PostRemove",
		}
		assert.Equal(t, expectedEvents, events)
	})
}
