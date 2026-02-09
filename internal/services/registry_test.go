package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockServicePlugin is a mock implementation of ServicePlugin for testing
type MockServicePlugin struct {
	name        string
	serviceType ServiceType
	validateErr error
	renderErr   error
	status      ServiceStatus
}

func (m *MockServicePlugin) Name() string {
	return m.name
}

func (m *MockServicePlugin) Type() ServiceType {
	return m.serviceType
}

func (m *MockServicePlugin) Validate(config interface{}) error {
	return m.validateErr
}

func (m *MockServicePlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	return m.renderErr
}

func (m *MockServicePlugin) Status(config interface{}) ServiceStatus {
	return m.status
}

func TestNewServiceRegistry(t *testing.T) {
	registry := NewServiceRegistry()
	assert.NotNil(t, registry)

	services := registry.ListServices()
	assert.Empty(t, services)
}

func TestRegisterService(t *testing.T) {
	tests := []struct {
		name        string
		service     ServiceDefinition
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid service",
			service: ServiceDefinition{
				Name:    "test-service",
				Type:    ServiceTypeMonitoring,
				Version: "1.0.0",
			},
			expectError: false,
		},
		{
			name: "service with dependencies",
			service: ServiceDefinition{
				Name:         "dependent-service",
				Type:         ServiceTypeMonitoring,
				Version:      "1.0.0",
				Dependencies: []string{"core-service"},
			},
			expectError: false,
		},
		{
			name: "missing name",
			service: ServiceDefinition{
				Type:    ServiceTypeMonitoring,
				Version: "1.0.0",
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "self-dependency",
			service: ServiceDefinition{
				Name:         "self-dep",
				Type:         ServiceTypeMonitoring,
				Version:      "1.0.0",
				Dependencies: []string{"self-dep"},
			},
			expectError: true,
			errorMsg:    "cannot depend on itself",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewServiceRegistry()
			err := registry.RegisterService(tt.service)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)

				// Verify service was registered
				retrieved, err := registry.GetService(tt.service.Name)
				require.NoError(t, err)
				assert.Equal(t, tt.service.Name, retrieved.Name)
			}
		})
	}
}

func TestRegisterService_Duplicate(t *testing.T) {
	registry := NewServiceRegistry()

	service := ServiceDefinition{
		Name:    "test-service",
		Type:    ServiceTypeMonitoring,
		Version: "1.0.0",
	}

	// First registration should succeed
	err := registry.RegisterService(service)
	require.NoError(t, err)

	// Second registration should fail
	err = registry.RegisterService(service)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestGetService(t *testing.T) {
	registry := NewServiceRegistry()

	service := ServiceDefinition{
		Name:    "test-service",
		Type:    ServiceTypeMonitoring,
		Version: "1.0.0",
	}

	err := registry.RegisterService(service)
	require.NoError(t, err)

	// Get existing service
	retrieved, err := registry.GetService("test-service")
	require.NoError(t, err)
	assert.Equal(t, "test-service", retrieved.Name)

	// Get non-existent service
	_, err = registry.GetService("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolveDependencies(t *testing.T) {
	tests := []struct {
		name        string
		services    []ServiceDefinition
		resolve     []string
		expectError bool
		expectOrder []string
		errorMsg    string
	}{
		{
			name: "no dependencies",
			services: []ServiceDefinition{
				{Name: "service1", Type: ServiceTypeCore},
				{Name: "service2", Type: ServiceTypeCore},
			},
			resolve:     []string{"service1", "service2"},
			expectError: false,
			expectOrder: []string{"service1", "service2"},
		},
		{
			name: "simple dependency chain",
			services: []ServiceDefinition{
				{Name: "core", Type: ServiceTypeCore},
				{Name: "monitoring", Type: ServiceTypeMonitoring, Dependencies: []string{"core"}},
				{Name: "logging", Type: ServiceTypeLogging, Dependencies: []string{"monitoring"}},
			},
			resolve:     []string{"logging"},
			expectError: false,
			expectOrder: []string{"core", "monitoring", "logging"},
		},
		{
			name: "multiple dependencies",
			services: []ServiceDefinition{
				{Name: "core", Type: ServiceTypeCore},
				{Name: "storage", Type: ServiceTypeStorage},
				{Name: "app", Type: ServiceTypeCustom, Dependencies: []string{"core", "storage"}},
			},
			resolve:     []string{"app"},
			expectError: false,
			expectOrder: []string{"core", "storage", "app"},
		},
		{
			name: "missing dependency",
			services: []ServiceDefinition{
				{Name: "app", Type: ServiceTypeCustom, Dependencies: []string{"missing"}},
			},
			resolve:     []string{"app"},
			expectError: true,
			errorMsg:    "not found",
		},
		{
			name: "circular dependency",
			services: []ServiceDefinition{
				{Name: "service1", Type: ServiceTypeCore, Dependencies: []string{"service2"}},
				{Name: "service2", Type: ServiceTypeCore, Dependencies: []string{"service1"}},
			},
			resolve:     []string{"service1"},
			expectError: true,
			errorMsg:    "circular dependency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewServiceRegistry()

			// Register all services
			for _, service := range tt.services {
				err := registry.RegisterService(service)
				require.NoError(t, err)
			}

			// Resolve dependencies
			resolved, err := registry.ResolveDependencies(tt.resolve)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				require.Len(t, resolved, len(tt.expectOrder))

				// Verify order
				for i, expected := range tt.expectOrder {
					assert.Equal(t, expected, resolved[i].Name)
				}
			}
		})
	}
}

func TestValidateDependencies(t *testing.T) {
	tests := []struct {
		name        string
		services    []ServiceDefinition
		validate    []string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid dependencies",
			services: []ServiceDefinition{
				{Name: "core", Type: ServiceTypeCore},
				{Name: "app", Type: ServiceTypeCustom, Dependencies: []string{"core"}},
			},
			validate:    []string{"app"},
			expectError: false,
		},
		{
			name: "missing dependency",
			services: []ServiceDefinition{
				{Name: "app", Type: ServiceTypeCustom, Dependencies: []string{"missing"}},
			},
			validate:    []string{"app"},
			expectError: true,
			errorMsg:    "not registered",
		},
		{
			name: "circular dependency",
			services: []ServiceDefinition{
				{Name: "service1", Type: ServiceTypeCore, Dependencies: []string{"service2"}},
				{Name: "service2", Type: ServiceTypeCore, Dependencies: []string{"service1"}},
			},
			validate:    []string{"service1"},
			expectError: true,
			errorMsg:    "circular dependency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewServiceRegistry()

			// Register all services
			for _, service := range tt.services {
				err := registry.RegisterService(service)
				require.NoError(t, err)
			}

			// Validate dependencies
			err := registry.ValidateDependencies(tt.validate)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegisterFromManifest(t *testing.T) {
	tests := []struct {
		name        string
		manifest    *ServicePluginManifest
		plugin      ServicePlugin
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid manifest and plugin",
			manifest: &ServicePluginManifest{
				Name:    "test-service",
				Version: "1.0.0",
				Type:    ServiceTypeMonitoring,
			},
			plugin: &MockServicePlugin{
				name:        "test-service",
				serviceType: ServiceTypeMonitoring,
			},
			expectError: false,
		},
		{
			name:        "nil manifest",
			manifest:    nil,
			plugin:      &MockServicePlugin{name: "test"},
			expectError: true,
			errorMsg:    "manifest is nil",
		},
		{
			name: "nil plugin",
			manifest: &ServicePluginManifest{
				Name:    "test-service",
				Version: "1.0.0",
				Type:    ServiceTypeMonitoring,
			},
			plugin:      nil,
			expectError: true,
			errorMsg:    "plugin is nil",
		},
		{
			name: "plugin name mismatch",
			manifest: &ServicePluginManifest{
				Name:    "test-service",
				Version: "1.0.0",
				Type:    ServiceTypeMonitoring,
			},
			plugin: &MockServicePlugin{
				name:        "different-name",
				serviceType: ServiceTypeMonitoring,
			},
			expectError: true,
			errorMsg:    "does not match",
		},
		{
			name: "invalid manifest",
			manifest: &ServicePluginManifest{
				// Missing name
				Version: "1.0.0",
				Type:    ServiceTypeMonitoring,
			},
			plugin: &MockServicePlugin{
				name:        "",
				serviceType: ServiceTypeMonitoring,
			},
			expectError: true,
			errorMsg:    "invalid manifest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewServiceRegistry()
			err := registry.RegisterFromManifest(tt.manifest, tt.plugin)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)

				// Verify service was registered
				service, err := registry.GetService(tt.manifest.Name)
				require.NoError(t, err)
				assert.Equal(t, tt.manifest.Name, service.Name)
				assert.Equal(t, tt.manifest.Version, service.Version)
			}
		})
	}
}

func TestLoadManifestsFromDirectory_Integration(t *testing.T) {
	// Create temporary directory with manifests
	tmpDir := t.TempDir()

	manifests := map[string]string{
		"core.yaml": `name: core
version: 1.0.0
type: core
description: Core service
`,
		"monitoring.yaml": `name: monitoring
version: 1.0.0
type: monitoring
description: Monitoring service
dependencies:
  - core
`,
	}

	for filename, content := range manifests {
		path := filepath.Join(tmpDir, filename)
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Load manifests into registry
	registry := NewServiceRegistry()
	err := registry.LoadManifestsFromDirectory(tmpDir)
	require.NoError(t, err)

	// Verify services were registered
	services := registry.ListServices()
	assert.Len(t, services, 2)

	// Verify core service
	core, err := registry.GetService("core")
	require.NoError(t, err)
	assert.Equal(t, "core", core.Name)
	assert.Equal(t, ServiceTypeCore, core.Type)

	// Verify monitoring service
	monitoring, err := registry.GetService("monitoring")
	require.NoError(t, err)
	assert.Equal(t, "monitoring", monitoring.Name)
	assert.Equal(t, ServiceTypeMonitoring, monitoring.Type)
	assert.Len(t, monitoring.Dependencies, 1)
	assert.Equal(t, "core", monitoring.Dependencies[0])
}

func TestListServices(t *testing.T) {
	registry := NewServiceRegistry()

	// Empty registry
	services := registry.ListServices()
	assert.Empty(t, services)

	// Add services
	service1 := ServiceDefinition{Name: "service1", Type: ServiceTypeCore}
	service2 := ServiceDefinition{Name: "service2", Type: ServiceTypeMonitoring}

	err := registry.RegisterService(service1)
	require.NoError(t, err)
	err = registry.RegisterService(service2)
	require.NoError(t, err)

	// List services
	services = registry.ListServices()
	assert.Len(t, services, 2)
}

func TestNewServiceRegistryWithEngine(t *testing.T) {
	engine := validation.NewValidationEngine()
	registry := NewServiceRegistryWithEngine(engine)
	
	assert.NotNil(t, registry)
	
	// Verify the engine is set correctly
	defaultRegistry, ok := registry.(*DefaultServiceRegistry)
	require.True(t, ok)
	assert.Equal(t, engine, defaultRegistry.validationEngine)
}

func TestGetEnabledServices(t *testing.T) {
	registry := NewServiceRegistry()
	
	// Register some services
	service1 := ServiceDefinition{Name: "service1", Type: ServiceTypeCore}
	service2 := ServiceDefinition{Name: "service2", Type: ServiceTypeMonitoring}
	
	err := registry.RegisterService(service1)
	require.NoError(t, err)
	err = registry.RegisterService(service2)
	require.NoError(t, err)
	
	// Get enabled services (currently returns all services)
	enabled := registry.GetEnabledServices(nil)
	assert.Len(t, enabled, 2)
}

func TestExecuteLifecycleHook(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		hook        string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid service and hook",
			serviceName: "test-service",
			hook:        "PreInstall",
			expectError: false,
		},
		{
			name:        "non-existent service",
			serviceName: "nonexistent",
			hook:        "PreInstall",
			expectError: true,
			errorMsg:    "not found",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewServiceRegistry()
			
			// Register a test service
			plugin := &MockServicePlugin{
				name:        "test-service",
				serviceType: ServiceTypeCore,
			}
			service := ServiceDefinition{
				Name:    "test-service",
				Type:    ServiceTypeCore,
				Version: "1.0.0",
				Plugin:  plugin,
			}
			err := registry.RegisterService(service)
			require.NoError(t, err)
			
			// Execute lifecycle hook
			ctx := context.Background()
			err = registry.ExecuteLifecycleHook(ctx, tt.serviceName, tt.hook, nil)
			
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExecuteLifecycleHooks(t *testing.T) {
	tests := []struct {
		name        string
		services    []ServiceDefinition
		execute     []string
		hook        string
		expectError bool
		errorMsg    string
	}{
		{
			name: "install hooks in dependency order",
			services: []ServiceDefinition{
				{Name: "core", Type: ServiceTypeCore, Plugin: &MockServicePlugin{name: "core", serviceType: ServiceTypeCore}},
				{Name: "app", Type: ServiceTypeCustom, Dependencies: []string{"core"}, Plugin: &MockServicePlugin{name: "app", serviceType: ServiceTypeCustom}},
			},
			execute:     []string{"app"},
			hook:        "PreInstall",
			expectError: false,
		},
		{
			name: "remove hooks in reverse order",
			services: []ServiceDefinition{
				{Name: "core", Type: ServiceTypeCore, Plugin: &MockServicePlugin{name: "core", serviceType: ServiceTypeCore}},
				{Name: "app", Type: ServiceTypeCustom, Dependencies: []string{"core"}, Plugin: &MockServicePlugin{name: "app", serviceType: ServiceTypeCustom}},
			},
			execute:     []string{"app"},
			hook:        "PreRemove",
			expectError: false,
		},
		{
			name: "missing dependency",
			services: []ServiceDefinition{
				{Name: "app", Type: ServiceTypeCustom, Dependencies: []string{"missing"}, Plugin: &MockServicePlugin{name: "app", serviceType: ServiceTypeCustom}},
			},
			execute:     []string{"app"},
			hook:        "PreInstall",
			expectError: true,
			errorMsg:    "not found",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewServiceRegistry()
			
			// Register all services
			for _, service := range tt.services {
				err := registry.RegisterService(service)
				require.NoError(t, err)
			}
			
			// Execute lifecycle hooks
			ctx := context.Background()
			err := registry.ExecuteLifecycleHooks(ctx, tt.execute, tt.hook, nil)
			
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetValidationEngine(t *testing.T) {
	registry := NewServiceRegistry()
	
	engine := registry.GetValidationEngine()
	assert.NotNil(t, engine)
}

func TestValidateService(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid service",
			serviceName: "test-service",
			expectError: false,
		},
		{
			name:        "non-existent service",
			serviceName: "nonexistent",
			expectError: true,
			errorMsg:    "not found",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewServiceRegistry()
			
			// Register a test service
			service := ServiceDefinition{
				Name:    "test-service",
				Type:    ServiceTypeCore,
				Version: "1.0.0",
			}
			err := registry.RegisterService(service)
			require.NoError(t, err)
			
			// Validate service
			ctx := context.Background()
			result, err := registry.ValidateService(ctx, tt.serviceName, nil)
			
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestBasicServicePlugin(t *testing.T) {
	plugin := &BasicServicePlugin{
		name:        "test-plugin",
		serviceType: ServiceTypeCore,
	}
	
	assert.Equal(t, "test-plugin", plugin.Name())
	assert.Equal(t, ServiceTypeCore, plugin.Type())
	assert.NoError(t, plugin.Validate(nil))
	assert.NoError(t, plugin.Render(context.Background(), nil, nil))
	
	status := plugin.Status(nil)
	assert.Equal(t, "pending", status.State)
}
